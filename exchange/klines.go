package exchange

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2/futures"

	"trading_bot/data"
)

// Binance USDⓈ-M Futures klines: GET https://fapi.binance.com/fapi/v1/klines

const maxKlinesLimit = 1000

// BinanceFuturesGenesisMs is the earliest open time for USDⓈ-M futures klines (2019-09-08 UTC).
const BinanceFuturesGenesisMs int64 = 1567900800000

// BinanceSpotGenesisMs is the earliest open time for BTCUSDT spot klines (2017-08-17 UTC).
const BinanceSpotGenesisMs int64 = 1502928000000

// Candle represents a parsed OHLCV candle ready for technical analysis.
type Candle struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

// GetKlines fetches historical candles via GET /fapi/v1/klines (public, no signature).
// b.client is *futures.Client — never the spot binance.Client.
func (b *BinanceExchange) GetKlines(symbol, interval string, limit int) ([]Candle, error) {
	if b.client == nil {
		return nil, fmt.Errorf("futures client is not configured")
	}
	symbol = NormalizeFuturesSymbol(symbol)
	if limit <= 0 || limit > maxKlinesLimit {
		return nil, fmt.Errorf("limit must be between 1 and %d, got %d", maxKlinesLimit, limit)
	}

	klines, err := b.client.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		Limit(limit).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("fetch klines for %s: %w", symbol, err)
	}

	candles := make([]Candle, 0, len(klines))
	for i, k := range klines {
		if k == nil {
			return nil, fmt.Errorf("nil kline at index %d", i)
		}

		candle, err := candleFromFuturesKline(k)
		if err != nil {
			return nil, fmt.Errorf("parse kline at index %d: %w", i, err)
		}
		candles = append(candles, candle)
	}

	return candles, nil
}

// GetKlinesBefore fetches historical candles ending before endTimeMs (exclusive).
func (b *BinanceExchange) GetKlinesBefore(symbol, interval string, limit int, endTimeMs int64) ([]Candle, error) {
	if b.client == nil {
		return nil, fmt.Errorf("futures client is not configured")
	}
	symbol = NormalizeFuturesSymbol(symbol)
	if limit <= 0 || limit > maxKlinesLimit {
		return nil, fmt.Errorf("limit must be between 1 and %d, got %d", maxKlinesLimit, limit)
	}

	svc := b.client.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		Limit(limit)
	if endTimeMs > 0 {
		endTimeMs = alignOpenTimeMs(endTimeMs, interval)
		svc = svc.EndTime(endTimeMs)
	}

	klines, err := svc.Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("fetch klines for %s: %w", symbol, err)
	}

	candles := make([]Candle, 0, len(klines))
	for i, k := range klines {
		if k == nil {
			return nil, fmt.Errorf("nil kline at index %d", i)
		}
		candle, err := candleFromFuturesKline(k)
		if err != nil {
			return nil, fmt.Errorf("parse kline at index %d: %w", i, err)
		}
		candles = append(candles, candle)
	}
	return candles, nil
}

const closedRangePageDelay = 150 * time.Millisecond

// FetchClosedRange performs exactly ONE Binance USDⓈ-M futures klines REST call
// for [fromMs, toMs] (open-time window, capped to last closed bar).
//
// Sterile pipe (Shot 9E): no SQLite, no gap synthesis, no multi-page stitching.
// Binance returns at most 1000 bars; if the window is larger, only the first page
// from fromMs is returned. Callers that need more must use FetchClosedRangePages.
// Exchange holes are preserved as missing open_times — never invented.
func (b *BinanceExchange) FetchClosedRange(symbol, interval string, fromMs, toMs int64) ([]Candle, error) {
	return b.fetchClosedRange(symbol, interval, fromMs, toMs, true)
}

// FetchClosedRangeExact is heal-only: [fromMs, toMs] without CapKlineEndToLastClosed(now).
// Caller must guarantee toMs is a fully closed bar open (e.g. PreviousBarOpen of a WS
// forming tip). Used when settle grace still excludes bars that the live tip proves closed.
func (b *BinanceExchange) FetchClosedRangeExact(symbol, interval string, fromMs, toMs int64) ([]Candle, error) {
	return b.fetchClosedRange(symbol, interval, fromMs, toMs, false)
}

func (b *BinanceExchange) fetchClosedRange(symbol, interval string, fromMs, toMs int64, applyCap bool) ([]Candle, error) {
	if b == nil || b.client == nil {
		return nil, fmt.Errorf("futures client is not configured")
	}
	symbol = NormalizeFuturesSymbol(symbol)

	if applyCap {
		if capped, err := data.CapKlineEndToLastClosed(toMs, interval); err == nil {
			toMs = capped
		}
	}
	fromMs, toMs = alignKlineRangeMs(fromMs, toMs, interval)
	if fromMs < BinanceFuturesGenesisMs {
		fromMs = alignOpenTimeMs(BinanceFuturesGenesisMs, interval)
	}
	if fromMs > toMs {
		return nil, nil
	}

	klines, err := b.client.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		StartTime(fromMs).
		EndTime(toMs).
		Limit(maxKlinesLimit).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("FetchClosedRange %s %s [%d..%d]: %w", symbol, interval, fromMs, toMs, err)
	}

	out := make([]Candle, 0, len(klines))
	for i, k := range klines {
		if k == nil {
			return nil, fmt.Errorf("nil kline at index %d", i)
		}
		candle, err := candleFromFuturesKline(k)
		if err != nil {
			return nil, fmt.Errorf("parse kline at index %d: %w", i, err)
		}
		out = append(out, candle)
	}
	return out, nil
}

// FetchClosedRangePages walks [fromMs, toMs] via repeated FetchClosedRange calls.
// Still sterile: no synthesize, no SQLite writes. Rate-limited between pages.
func (b *BinanceExchange) FetchClosedRangePages(symbol, interval string, fromMs, toMs int64) ([]Candle, error) {
	return b.fetchClosedRangePages(symbol, interval, fromMs, toMs, true)
}

// FetchClosedRangePagesExact pages [fromMs, toMs] without Cap settle grace (ADR-017 heal fill).
func (b *BinanceExchange) FetchClosedRangePagesExact(symbol, interval string, fromMs, toMs int64) ([]Candle, error) {
	return b.fetchClosedRangePages(symbol, interval, fromMs, toMs, false)
}

func (b *BinanceExchange) fetchClosedRangePages(symbol, interval string, fromMs, toMs int64, applyCap bool) ([]Candle, error) {
	if applyCap {
		if capped, err := data.CapKlineEndToLastClosed(toMs, interval); err == nil {
			toMs = capped
		}
	}
	fromMs, toMs = alignKlineRangeMs(fromMs, toMs, interval)
	if fromMs < BinanceFuturesGenesisMs {
		fromMs = alignOpenTimeMs(BinanceFuturesGenesisMs, interval)
	}
	if fromMs > toMs {
		return nil, nil
	}

	cursor := fromMs
	all := make([]Candle, 0, maxKlinesLimit)
	for cursor <= toMs {
		page, err := b.fetchClosedRange(symbol, interval, cursor, toMs, false) // already capped above if needed
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		all = append(all, page...)
		last := page[len(page)-1]
		if last.CloseTime >= toMs || len(page) < maxKlinesLimit {
			break
		}
		next := last.CloseTime + 1
		if next <= cursor {
			break
		}
		cursor = next
		time.Sleep(closedRangePageDelay)
	}
	return dedupeCandlesByOpenTime(all), nil
}

// alignOpenTimeMs floors t to the bar open for interval (ADR-011 CurrentBarOpen).
func alignOpenTimeMs(t int64, interval string) int64 {
	open, err := data.CurrentBarOpen(t, interval)
	if err != nil {
		return t
	}
	return open
}

// alignKlineRangeMs floors start/end to candle open boundaries for REST API requests.
func alignKlineRangeMs(startMs, endMs int64, interval string) (int64, int64) {
	return alignOpenTimeMs(startMs, interval), alignOpenTimeMs(endMs, interval)
}

// CandlesToData converts exchange candles to data.Candle for PersistenceQueue / SQLite.
func CandlesToData(in []Candle) []data.Candle {
	out := make([]data.Candle, len(in))
	for i, c := range in {
		out[i] = data.Candle{
			OpenTime:  c.OpenTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
			CloseTime: c.CloseTime,
		}
	}
	return out
}

func candlesFromData(in []data.Candle) []Candle {
	out := make([]Candle, len(in))
	for i, c := range in {
		out[i] = Candle{
			OpenTime:  c.OpenTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
			CloseTime: c.CloseTime,
		}
	}
	return out
}

func dedupeCandlesByOpenTime(candles []Candle) []Candle {
	if len(candles) == 0 {
		return candles
	}
	out := make([]Candle, 0, len(candles))
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

func candleFromFuturesKline(k *futures.Kline) (Candle, error) {
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
