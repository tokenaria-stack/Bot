package server

import (
	"context"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/market"
)

// HistoryWindowQuery is the sole input contract for history delivery (Shot 9A).
type HistoryWindowQuery struct {
	Spec        TimeframeSpec
	EndTimeMs   int64 // Unix ms; 0 → now (then Closed-bar Boundary Cap)
	CandleLimit int   // display bars (warmup added inside GetWindow)
}

// HistoryWindow is a continuous kline series ready for packing (columnar/JSON).
// Controllers must not know whether bars came from SQLite, RAM, or both.
type HistoryWindow struct {
	Klines  []exchange.Kline
	HasMore bool
}

// resolveClosedBarBoundary is the Closed-bar Boundary SSOT (#67 / ADR-009).
// Any path that asks "what is the last closed bar?" must use CapKlineEndToLastClosed —
// the same settle-grace law as Frame boot and REST fetch. Wall-clock Now() is not a boundary.
func resolveClosedBarBoundary(endTimeMs int64, interval string) int64 {
	if endTimeMs <= 0 {
		endTimeMs = time.Now().UnixMilli()
	}
	if interval == "" {
		return endTimeMs
	}
	if capped, err := data.CapKlineEndToLastClosed(endTimeMs, interval); err == nil {
		return capped
	}
	return endTimeMs
}

// GetWindow is the unique owner of history delivery for REST chart paths.
// Temporary seam (P0a): SQLite archive ∪ Frame RAM (overlay wins), filtered to EndTimeMs.
// Live/near-live ends are clipped by resolveClosedBarBoundary (CapKlineEndToLastClosed).
// Thread-safety: RAM is read only via Frame.GetKlines / GetKlinesTail (RLock copy).
func (d *DashboardServer) GetWindow(ctx context.Context, q HistoryWindowQuery) (HistoryWindow, bool) {
	if d == nil {
		return HistoryWindow{}, false
	}
	if err := requestCtxErr(ctx); err != nil {
		return HistoryWindow{}, false
	}

	limit := q.CandleLimit
	if limit <= 0 {
		limit = defaultStateCandleLimit
	}
	warmup := market.IndicatorWarmupBars
	wantBars := limit + warmup

	spec := q.Spec
	interval := spec.BinanceInterval
	if interval == "" {
		interval = spec.ID
	}
	endTimeMs := resolveClosedBarBoundary(q.EndTimeMs, interval)

	// RAM-only timeframes: working set is already the delivery source.
	if spec.Kind == TFRAMOnly {
		klines := d.ramKlines(spec.ID, wantBars)
		klines = filterKlinesUntilOpenMs(klines, endTimeMs)
		if len(klines) == 0 {
			return HistoryWindow{}, false
		}
		return HistoryWindow{Klines: klines, HasMore: false}, true
	}

	if spec.Kind != TFBinanceREST || spec.BinanceInterval == "" {
		return HistoryWindow{}, false
	}

	dbKlines := d.loadRESTKlinesFromStore(ctx, spec, endTimeMs, limit, false)
	if err := requestCtxErr(ctx); err != nil {
		return HistoryWindow{}, false
	}

	// Thread-safe copy under Frame RLock — never touch raw a.klines.
	ramKlines := d.frameKlinesTail(spec, wantBars)
	ramKlines = filterKlinesUntilOpenMs(ramKlines, endTimeMs)

	merged := exchange.MergeKlineSeries(dbKlines, ramKlines, exchange.AuthoritySettled, exchange.AuthorityFinal)
	if len(merged) == 0 {
		return HistoryWindow{}, false
	}
	if wantBars > 0 && len(merged) > wantBars {
		merged = merged[len(merged)-wantBars:]
	}

	hasMore := false
	if len(merged) > 0 && spec.BinanceInterval != "" {
		hasMore = d.sqliteHasBarsBefore(spec.BinanceInterval, exchange.ChartTimeSec(merged[0].OpenTime)*1000)
	}

	return HistoryWindow{Klines: merged, HasMore: hasMore}, true
}

// frameKlinesTail returns a defensive copy of the live working-set tail (RLock inside Frame).
func (d *DashboardServer) frameKlinesTail(spec TimeframeSpec, maxBars int) []exchange.Kline {
	if d == nil {
		return nil
	}
	if maxBars <= 0 {
		maxBars = market.LiveKlineRAMCap
	}
	if frame, ok := d.frames[spec.ID]; ok && frame != nil {
		return frame.GetKlinesTail(maxBars)
	}
	if spec.BinanceInterval != "" {
		if frame, ok := d.frames[spec.BinanceInterval]; ok && frame != nil {
			return frame.GetKlinesTail(maxBars)
		}
	}
	return nil
}

// filterKlinesUntilOpenMs keeps bars with OpenTime <= endTimeMs (inclusive).
// Prevents live tip from leaking into deep-history prepend windows.
func filterKlinesUntilOpenMs(klines []exchange.Kline, endTimeMs int64) []exchange.Kline {
	if len(klines) == 0 || endTimeMs <= 0 {
		return klines
	}
	out := make([]exchange.Kline, 0, len(klines))
	for _, k := range klines {
		k = exchange.NormalizeKline(k)
		if k.OpenTime <= endTimeMs {
			out = append(out, k)
		}
	}
	return out
}
