package strategy

import (
	"log"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

// LoadRAMHistory boots Marker RAM from SQLite, then REST-fills holes via FetchClosedRangePages.
// Sterile (Shot 9E): never synthesizes bars; never writes SQLite (PersistenceQueue owns disk).
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

	db := LoadRAMHistoryFromDB(symbol, interval, maxBars)
	if rest == nil {
		return db
	}

	candles, fetchErr := rest.FetchClosedRangePages(symbol, interval, startMs, endMs)
	if fetchErr != nil {
		log.Printf("[RAM] FetchClosedRangePages %s %s: %v — SQLite fallback", symbol, interval, fetchErr)
		return db
	}
	if len(candles) == 0 {
		return db
	}
	restKlines := exchange.KlinesFromCandles(candles)
	if len(db) == 0 {
		return restKlines
	}
	// Overlay: REST wins on duplicate open_time (exchange is source of truth for OHLCV).
	return mergeKlinesByOpenTime(db, restKlines)
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
