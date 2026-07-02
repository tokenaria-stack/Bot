package exchange

import (
	"fmt"
	"sync"
	"time"

	"trading_bot/data"
)

// htfCacheEntry stores the earliest startMs covered by cached klines for a symbol/interval key.
type htfCacheEntry struct {
	startMs  int64
	klines   []Kline
	Pinned   bool
	LastUsed time.Time
}

// HTFProvider is a thread-safe hub for loading and caching historical klines across timeframes.
type HTFProvider struct {
	cache sync.Map // map[string]htfCacheEntry keyed by symbol_interval
}

// NewHTFProvider creates an empty HTF data hub.
func NewHTFProvider() *HTFProvider {
	return &HTFProvider{}
}

func htfCacheKey(symbol, interval string) string {
	return fmt.Sprintf("%s_%s", NormalizeFuturesSymbol(symbol), interval)
}

// GetKlines returns continuous-contract klines for [startMs, now] from SQLite cache.
// Results are cached per symbol+interval; a later request with an earlier startMs reloads the range.
func (p *HTFProvider) GetKlines(symbol, interval string, startMs int64) ([]Kline, error) {
	if p == nil {
		return nil, fmt.Errorf("htf provider is nil")
	}
	key := htfCacheKey(symbol, interval)
	if raw, ok := p.cache.Load(key); ok {
		entry := raw.(htfCacheEntry)
		if entry.startMs <= startMs {
			entry.LastUsed = time.Now()
			p.cache.Store(key, entry)
			return sliceKlinesFrom(entry.klines, startMs), nil
		}
	}

	endMs := time.Now().UnixMilli()
	candles, err := LoadContinuousContractFromDB(symbol, interval, startMs, endMs, 0)
	if err != nil {
		return nil, err
	}
	klines := KlinesFromCandles(candles)
	now := time.Now()
	p.cache.Store(key, htfCacheEntry{
		startMs:  startMs,
		klines:   klines,
		Pinned:   false,
		LastUsed: now,
	})
	return cloneKlines(klines), nil
}

// PinKlines marks a cached symbol/interval entry as pinned so it survives soft eviction.
// No-op when the entry is missing — call GetKlines first to populate the cache.
func (p *HTFProvider) PinKlines(symbol, interval string) {
	if p == nil {
		return
	}
	key := htfCacheKey(symbol, interval)
	raw, ok := p.cache.Load(key)
	if !ok {
		return
	}
	entry := raw.(htfCacheEntry)
	entry.Pinned = true
	p.cache.Store(key, entry)
}

// ClearCache removes cached entries. When force is false, pinned entries are kept.
// Returns the number of removed entries.
func (p *HTFProvider) ClearCache(force bool) int {
	if p == nil {
		return 0
	}
	removed := 0
	p.cache.Range(func(key, value any) bool {
		if force {
			p.cache.Delete(key)
			removed++
			return true
		}
		entry := value.(htfCacheEntry)
		if !entry.Pinned {
			p.cache.Delete(key)
			removed++
		}
		return true
	})
	return removed
}

// CleanupIdle evicts unpinned entries idle longer than maxIdle.
// Returns the number of evicted entries.
func (p *HTFProvider) CleanupIdle(maxIdle time.Duration) int {
	if p == nil {
		return 0
	}
	removed := 0
	cutoff := time.Now().Add(-maxIdle)
	p.cache.Range(func(key, value any) bool {
		entry := value.(htfCacheEntry)
		if !entry.Pinned && entry.LastUsed.Before(cutoff) {
			p.cache.Delete(key)
			removed++
		}
		return true
	})
	return removed
}

// KlinesFromCandles converts exchange candles into normalized klines.
func KlinesFromCandles(candles []Candle) []Kline {
	if len(candles) == 0 {
		return nil
	}
	klines := make([]Kline, len(candles))
	for i, c := range candles {
		klines[i] = NormalizeKline(Kline{
			OpenTime:  c.OpenTime,
			CloseTime: c.CloseTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
		})
	}
	return klines
}

func cloneKlines(klines []Kline) []Kline {
	if len(klines) == 0 {
		return nil
	}
	out := make([]Kline, len(klines))
	copy(out, klines)
	return out
}

// ParseIntervalToSeconds converts a Binance interval string to duration in seconds.
func ParseIntervalToSeconds(interval string) int64 {
	durMs, err := data.IntervalDurationMs(interval)
	if err != nil || durMs <= 0 {
		return 0
	}
	return durMs / 1000
}

// GetCandlesStrictlyBefore returns HTF klines that were fully closed at or before maxTimeSec.
// Uses the in-memory cache populated by GetKlines; returns nil when the cache entry is missing.
func (p *HTFProvider) GetCandlesStrictlyBefore(symbol, interval string, maxTimeSec int64) []Kline {
	if p == nil || maxTimeSec <= 0 {
		return nil
	}
	key := htfCacheKey(symbol, interval)
	raw, ok := p.cache.Load(key)
	if !ok {
		return nil
	}
	entry := raw.(htfCacheEntry)
	intervalSec := ParseIntervalToSeconds(interval)

	var filtered []Kline
	for _, c := range entry.klines {
		closeTimeMs := c.CloseTime
		if closeTimeMs <= 0 {
			if intervalSec > 0 {
				closeTimeMs = c.OpenTime + intervalSec*1000
			} else {
				closeTimeMs = c.OpenTime
			}
		}
		closeTimeSec := closeTimeMs / 1000
		if closeTimeSec <= maxTimeSec {
			filtered = append(filtered, c)
			continue
		}
		// Cache is time-sorted; first future candle ends the scan.
		break
	}
	return cloneKlines(filtered)
}

func sliceKlinesFrom(klines []Kline, startMs int64) []Kline {
	if startMs <= 0 || len(klines) == 0 {
		return cloneKlines(klines)
	}
	i := 0
	for i < len(klines) && klines[i].OpenTime < startMs {
		i++
	}
	if i >= len(klines) {
		return nil
	}
	out := make([]Kline, len(klines)-i)
	copy(out, klines[i:])
	return out
}
