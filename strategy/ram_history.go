package strategy

import (
	"log"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

// LoadRAMHistory reads klines for Marker boot with mandatory REST catch-up when SQLite has gaps.
func LoadRAMHistory(rest *exchange.BinanceExchange, symbol, interval string, maxBars int) []exchange.Kline {
	if maxBars <= 0 {
		maxBars = LiveKlineRAMCap
	}
	endMs := time.Now().UnixMilli()
	intervalMs, err := data.IntervalDurationMs(interval)
	if err != nil || intervalMs <= 0 {
		return nil
	}
	startMs := endMs - intervalMs*int64(maxBars)
	if startMs < 0 {
		startMs = 0
	}
	if intervalSkipsKlineGapFill(interval) {
		return LoadRAMHistoryFromDB(symbol, interval, maxBars)
	}
	if rest != nil {
		candles, fetchErr := rest.FetchHistoricalKlines(symbol, interval, startMs, endMs)
		if fetchErr == nil && len(candles) > 0 {
			return exchange.KlinesFromCandles(candles)
		}
		if fetchErr != nil {
			log.Printf("[RAM] gap-fill %s %s: %v — SQLite fallback", symbol, interval, fetchErr)
		}
	}
	return LoadRAMHistoryFromDB(symbol, interval, maxBars)
}

// LoadRAMHistoryFromDB reads up to maxBars klines from SQLite once (startup / hydrate only).
func LoadRAMHistoryFromDB(symbol, interval string, maxBars int) []exchange.Kline {
	if maxBars <= 0 {
		maxBars = LiveKlineRAMCap
	}
	if err := data.InitDB(); err != nil {
		return nil
	}
	endMs := time.Now().UnixMilli()
	intervalMs, err := data.IntervalDurationMs(interval)
	if err != nil || intervalMs <= 0 {
		return nil
	}
	startMs := endMs - intervalMs*int64(maxBars)
	if startMs < 0 {
		startMs = 0
	}
	candles, err := exchange.LoadContinuousContractFromDB(symbol, interval, startMs, endMs, 0)
	if err != nil || len(candles) == 0 {
		return nil
	}
	return exchange.KlinesFromCandles(candles)
}
