package server

import (
	"context"
	"time"

	"trading_bot/exchange"
	"trading_bot/strategy"
)

// HistoryWindowQuery is the sole input contract for history delivery (Shot 9A).
type HistoryWindowQuery struct {
	Spec        TimeframeSpec
	EndTimeMs   int64 // Unix ms; 0 → now
	CandleLimit int   // display bars (warmup added inside GetWindow)
}

// HistoryWindow is a continuous kline series ready for packing (columnar/JSON).
// Controllers must not know whether bars came from SQLite, RAM, or both.
type HistoryWindow struct {
	Klines  []exchange.Kline
	HasMore bool
}

// GetWindow is the unique owner of history delivery for REST chart paths.
// Temporary seam (P0a): SQLite archive ∪ Analyst RAM (overlay wins), filtered to EndTimeMs.
// Thread-safety: RAM is read only via Marker.GetKlines / GetKlinesTail (RLock copy).
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
	warmup := strategy.IndicatorWarmupBars
	wantBars := limit + warmup

	endTimeMs := q.EndTimeMs
	if endTimeMs <= 0 {
		endTimeMs = time.Now().UnixMilli()
	}

	spec := q.Spec

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

	// Thread-safe copy under Marker RLock — never touch raw a.klines.
	ramKlines := d.analystKlinesTail(spec, wantBars)
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

// analystKlinesTail returns a defensive copy of the live working-set tail (RLock inside Marker).
func (d *DashboardServer) analystKlinesTail(spec TimeframeSpec, maxBars int) []exchange.Kline {
	if d == nil {
		return nil
	}
	if maxBars <= 0 {
		maxBars = strategy.LiveKlineRAMCap
	}
	if analyst, ok := d.analysts[spec.ID]; ok && analyst != nil {
		return analyst.GetKlinesTail(maxBars)
	}
	if spec.BinanceInterval != "" {
		if analyst, ok := d.analysts[spec.BinanceInterval]; ok && analyst != nil {
			return analyst.GetKlinesTail(maxBars)
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
