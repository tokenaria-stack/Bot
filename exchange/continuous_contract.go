package exchange

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	binancespot "github.com/adshao/go-binance/v2"

	"trading_bot/data"
)

// SpotStorageSymbol returns the SQLite symbol for spot-era candles (e.g. BTCUSDT_SPOT).
func SpotStorageSymbol(symbol string) string {
	return NormalizeFuturesSymbol(symbol) + "_SPOT"
}

// contractSegment is one contiguous slice of a continuous-contract request.
type contractSegment struct {
	start       int64
	end         int64
	spotStorage bool // spot API + _SPOT SQLite key when true; futures otherwise
}

// splitRangeAtGenesis splits [startMs, endMs] around BinanceFuturesGenesisMs.
func splitRangeAtGenesis(startMs, endMs int64) []contractSegment {
	if endMs < startMs {
		return nil
	}
	if startMs >= BinanceFuturesGenesisMs {
		return []contractSegment{{start: startMs, end: endMs, spotStorage: false}}
	}
	if endMs < BinanceFuturesGenesisMs {
		return []contractSegment{{start: startMs, end: endMs, spotStorage: true}}
	}
	return []contractSegment{
		{start: startMs, end: BinanceFuturesGenesisMs - 1, spotStorage: true},
		{start: BinanceFuturesGenesisMs, end: endMs, spotStorage: false},
	}
}

func storageSymbolForSegment(symbol string, seg contractSegment) string {
	if seg.spotStorage {
		return SpotStorageSymbol(symbol)
	}
	return NormalizeFuturesSymbol(symbol)
}

// LoadContinuousContractFromDB reads spot (_SPOT) and futures rows from SQLite and stitches them.
// When limit > 0, returns at most the last limit stitched bars (ascending).
func LoadContinuousContractFromDB(symbol, interval string, startTimeMs, endTimeMs int64, limit int) ([]Candle, error) {
	symbol = NormalizeFuturesSymbol(symbol)
	var merged []data.Candle

	for _, seg := range splitRangeAtGenesis(startTimeMs, endTimeMs) {
		storageSym := storageSymbolForSegment(symbol, seg)
		rows, err := data.LoadKlines(storageSym, interval, seg.start, seg.end, 0)
		if err != nil {
			return nil, fmt.Errorf("load %s %s [%d..%d]: %w", storageSym, interval, seg.start, seg.end, err)
		}
		merged = append(merged, rows...)
	}

	out := candlesFromData(dedupeDataCandlesByOpenTime(merged))
	return TruncateCandlesTail(out, limit), nil
}

// TruncateCandlesTail keeps the last max bars; max <= 0 is a no-op.
func TruncateCandlesTail(candles []Candle, max int) []Candle {
	if max <= 0 || len(candles) <= max {
		return candles
	}
	return candles[len(candles)-max:]
}

func queryContinuousContractCacheBounds(symbol, interval string, startTimeMs, endTimeMs int64) data.KlineCacheBounds {
	symbol = NormalizeFuturesSymbol(symbol)
	var out data.KlineCacheBounds

	for _, seg := range splitRangeAtGenesis(startTimeMs, endTimeMs) {
		storageSym := storageSymbolForSegment(symbol, seg)
		bounds, err := data.QueryKlineCacheBounds(storageSym, interval)
		if err != nil || !bounds.HasData {
			continue
		}
		out = mergeKlineCacheBounds(out, bounds)
	}
	return out
}

func mergeKlineCacheBounds(a, b data.KlineCacheBounds) data.KlineCacheBounds {
	if !a.HasData {
		return b
	}
	if !b.HasData {
		return a
	}
	minT := a.MinTime
	if b.MinTime < minT {
		minT = b.MinTime
	}
	maxT := a.MaxTime
	if b.MaxTime > maxT {
		maxT = b.MaxTime
	}
	return data.KlineCacheBounds{
		Count:   a.Count + b.Count,
		MinTime: minT,
		MaxTime: maxT,
		HasData: true,
	}
}

// normalizeSpotRange clamps spot fetch start to BinanceSpotGenesisMs.
// Returns ok=false when no spot data exists for the requested window.
func normalizeSpotRange(startMs, endMs int64) (int64, int64, bool) {
	if endMs <= 0 || endMs < BinanceSpotGenesisMs {
		return 0, 0, false
	}
	if startMs < BinanceSpotGenesisMs {
		startMs = BinanceSpotGenesisMs
	}
	if startMs >= endMs {
		return 0, 0, false
	}
	return startMs, endMs, true
}

// normalizeContinuousContractRange clamps pre-spot starts for continuous-contract fetches.
// Returns ok=false when the window is entirely before spot genesis or otherwise empty.
func normalizeContinuousContractRange(startMs, endMs int64) (int64, int64, bool) {
	if endMs <= 0 {
		return 0, 0, false
	}
	if endMs < BinanceSpotGenesisMs {
		return 0, 0, false
	}
	if startMs < BinanceSpotGenesisMs {
		startMs = BinanceSpotGenesisMs
	}
	if startMs >= endMs {
		return 0, 0, false
	}
	return startMs, endMs, true
}

func dedupeDataCandlesByOpenTime(candles []data.Candle) []data.Candle {
	if len(candles) == 0 {
		return candles
	}
	out := make([]data.Candle, 0, len(candles))
	var lastOpen int64 = -1
	for _, c := range candles {
		if c.OpenTime == lastOpen {
			out[len(out)-1] = c
			continue
		}
		out = append(out, c)
		lastOpen = c.OpenTime
	}
	return out
}

func (b *BinanceExchange) fetchGapSegment(symbol, interval string, seg contractSegment) ([]Candle, error) {
	stepMs, err := data.IntervalDurationMs(interval)
	if err != nil {
		return nil, err
	}
	start, end := alignKlineRangeMs(seg.start, seg.end, stepMs)
	if start > end {
		return nil, nil
	}
	if seg.spotStorage {
		start, end, ok := normalizeSpotRange(start, end)
		if !ok {
			log.Printf("[Spot API] skipped gap segment: normalized spot range empty [%d..%d]", start, end)
			return nil, nil
		}
		return b.fetchSpotHistoricalKlinesFromAPI(symbol, interval, start, end)
	}
	return b.fetchHistoricalKlinesFromAPI(symbol, interval, start, end)
}

func (b *BinanceExchange) fetchSpotHistoricalKlinesFromAPI(symbol, interval string, startTimeMs, endTimeMs int64) ([]Candle, error) {
	rawStart, rawEnd := startTimeMs, endTimeMs
	stepMs, err := data.IntervalDurationMs(interval)
	if err != nil {
		return nil, err
	}
	startTimeMs, endTimeMs = alignKlineRangeMs(startTimeMs, endTimeMs, stepMs)
	startTimeMs, endTimeMs, ok := normalizeSpotRange(startTimeMs, endTimeMs)
	if !ok {
		log.Printf("[Spot API] skipped fetch: normalized range empty for %s %s [%d..%d]", symbol, interval, rawStart, rawEnd)
		return nil, nil
	}

	symbol = NormalizeFuturesSymbol(symbol)
	client := binancespot.NewClient("", "")

	cursor := startTimeMs
	result := make([]Candle, 0, historicalKlinesPageLimit)

	for cursor < endTimeMs {
		klines, err := client.NewKlinesService().
			Symbol(symbol).
			Interval(interval).
			StartTime(cursor).
			EndTime(endTimeMs).
			Limit(historicalKlinesPageLimit).
			Do(context.Background())
		if err != nil {
			log.Printf("[Spot API] ERROR %s %s page at cursor=%d (https://api.binance.com/api/v3/klines): %v",
				symbol, interval, cursor, err)
			return nil, fmt.Errorf("fetch spot klines page at %d: %w", cursor, err)
		}
		if len(klines) == 0 {
			log.Printf("[Spot API] empty page at cursor=%d for %s %s [%d..%d]", cursor, symbol, interval, startTimeMs, endTimeMs)
			break
		}

		for i, k := range klines {
			if k == nil {
				return nil, fmt.Errorf("nil spot kline at page offset %d", i)
			}
			candle, err := candleFromSpotKline(k)
			if err != nil {
				return nil, fmt.Errorf("parse spot kline: %w", err)
			}
			result = append(result, candle)
		}

		last := klines[len(klines)-1]
		cursor = last.CloseTime + 1
		if cursor >= endTimeMs || len(klines) < historicalKlinesPageLimit {
			break
		}
		time.Sleep(historicalKlinesRequestDelay)
	}

	result = dedupeCandlesByOpenTime(result)
	log.Printf("[Spot API] Fetched total %d bars for %s %s [%d..%d]", len(result), symbol, interval, startTimeMs, endTimeMs)
	return result, nil
}

func candleFromSpotKline(k *binancespot.Kline) (Candle, error) {
	open, err := strconv.ParseFloat(k.Open, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse open %q: %w", k.Open, err)
	}
	high, err := strconv.ParseFloat(k.High, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse high %q: %w", k.High, err)
	}
	low, err := strconv.ParseFloat(k.Low, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse low %q: %w", k.Low, err)
	}
	closePrice, err := strconv.ParseFloat(k.Close, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse close %q: %w", k.Close, err)
	}
	volume, err := strconv.ParseFloat(k.Volume, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse volume %q: %w", k.Volume, err)
	}

	return Candle{
		OpenTime:  k.OpenTime,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     closePrice,
		Volume:    volume,
		CloseTime: k.CloseTime,
	}, nil
}
