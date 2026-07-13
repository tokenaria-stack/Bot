package strategy

import (
	"context"
	"log"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

const klineGapFillInterval = 5 * time.Minute

// intervalSkipsKlineGapFill reports intervals where REST gap-fill is unreliable or unused.
func intervalSkipsKlineGapFill(interval string) bool {
	switch interval {
	case "1M", "1w":
		return true
	default:
		return false
	}
}

// KlineTailNeedsGapFill reports whether the live tail is missing more than two closed bars.
func KlineTailNeedsGapFill(lastOpenMs, endMs, intervalMs int64) bool {
	if lastOpenMs <= 0 || endMs <= 0 || intervalMs <= 0 {
		return false
	}
	return endMs-lastOpenMs > intervalMs*2
}

// KlineSeriesNeedsGapFill reports tail staleness or an internal hole between consecutive bars.
func KlineSeriesNeedsGapFill(klines []exchange.Kline, endMs, intervalMs int64) bool {
	if len(klines) == 0 {
		return true
	}
	if KlineTailNeedsGapFill(klines[len(klines)-1].OpenTime, endMs, intervalMs) {
		return true
	}
	for i := 1; i < len(klines); i++ {
		if klines[i].OpenTime-klines[i-1].OpenTime > intervalMs*2 {
			return true
		}
	}
	return false
}

func klineReconcileWindowMs(interval string, maxBars int) (startMs, endMs int64) {
	endMs = time.Now().UnixMilli()
	if capped, err := data.CapKlineEndToLastClosed(endMs, interval); err == nil {
		endMs = capped
	}
	intervalMs, err := data.IntervalDurationMs(interval)
	if err != nil || intervalMs <= 0 {
		return 0, endMs
	}
	if maxBars <= 0 {
		maxBars = LiveKlineRAMCap
	}
	startMs = endMs - intervalMs*int64(maxBars)
	if startMs < 0 {
		startMs = 0
	}
	return startMs, endMs
}

// StartKlineGapFillLoop runs an immediate reconcile and periodic RAM gap-fill (Master SSOT).
func (m *MasterGeneral) StartKlineGapFillLoop(ctx context.Context) {
	if m == nil {
		return
	}
	go func() {
		m.ReconcileAllKlineGaps()
		ticker := time.NewTicker(klineGapFillInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.ReconcileAllKlineGaps()
			}
		}
	}()
}

// ReconcileAllKlineGaps checks every Marker for holes and backfills via FetchClosedRange → RAM.
func (m *MasterGeneral) ReconcileAllKlineGaps() {
	if m == nil || m.exchangeClient == nil {
		return
	}
	m.mu.RLock()
	symbol := m.symbol
	intervals := make([]string, 0, len(m.analysts))
	for interval := range m.analysts {
		intervals = append(intervals, interval)
	}
	m.mu.RUnlock()

	for _, interval := range intervals {
		m.mu.RLock()
		analyst := m.analysts[interval]
		m.mu.RUnlock()
		if analyst == nil {
			continue
		}
		m.reconcileKlineGap(symbol, interval, analyst)
	}
}

func (m *MasterGeneral) reconcileKlineGap(symbol, interval string, analyst *Marker) {
	if m == nil || m.exchangeClient == nil || analyst == nil {
		return
	}
	if intervalSkipsKlineGapFill(interval) {
		return
	}
	startMs, endMs := klineReconcileWindowMs(interval, LiveKlineRAMCap)
	intervalMs, err := data.IntervalDurationMs(interval)
	if err != nil || intervalMs <= 0 {
		return
	}
	tail := analyst.GetKlines()
	// RAM-only gate: healthy Analyst skips REST. SQLite tip heal is StartSQLiteArchiveCatchUpLoop.
	if len(tail) > 0 && !KlineSeriesNeedsGapFill(tail, endMs, intervalMs) {
		return
	}

	candles, err := m.exchangeClient.FetchClosedRangePages(symbol, interval, startMs, endMs)
	if err != nil {
		log.Printf("[Master] gap-fill %s %s [%d..%d]: %v", symbol, interval, startMs, endMs, err)
		return
	}
	if len(candles) == 0 {
		return
	}
	klines := exchange.KlinesFromCandles(candles)
	analyst.LoadHistoricalKlines(klines)

	// Archive the same real bars via sole writer (does not mutate RAM).
	m.enqueueArchiveCandles(symbol, interval, candles)

	m.mu.RLock()
	workTF := m.timeframe
	m.mu.RUnlock()
	if interval == workTF {
		m.SeedClosedBarTelemetry()
		m.rebuildMTFTrackerIfReady()
	}
}

func (m *MasterGeneral) enqueueArchiveCandles(symbol, interval string, candles []exchange.Candle) {
	if m == nil || len(candles) == 0 {
		return
	}
	m.mu.RLock()
	q := m.persistQ
	m.mu.RUnlock()
	if q == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := q.AppendClosedBars(ctx, symbol, interval, exchange.CandlesToData(candles)); err != nil {
		log.Printf("[Master] archive enqueue %s %s: %v", symbol, interval, err)
	}
}
