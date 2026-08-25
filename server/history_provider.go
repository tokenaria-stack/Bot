package server

import (
	"context"
	"log"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/market"
)

// HistoryWindowQuery is the sole input contract for history delivery (Shot 9A).
type HistoryWindowQuery struct {
	Spec        TimeframeSpec
	EndTimeMs   int64 // Unix ms; 0 → now (then Closed-bar Boundary Cap). Ignored when StartTimeMs > 0.
	StartTimeMs int64 // Unix ms exclusive lower bound (1s forward page only).
	CandleLimit int   // display bars (warmup added inside GetWindow for endTime)
}

// HistoryWindow is a continuous kline series ready for packing (columnar/JSON).
// Controllers must not know whether bars came from SQLite, RAM, or both.
type HistoryWindow struct {
	Klines   []exchange.Kline
	HasMore  bool
	HasNewer bool
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
	if exchange.IsLiveSecond(spec.ID) {
		if q.StartTimeMs > 0 {
			return d.getMicroWindowAfter(spec, q.StartTimeMs, limit)
		}
		return d.getMicroWindow(spec, q.EndTimeMs, wantBars)
	}
	if exchange.IsDerivedTime(spec.ID) {
		return d.getDerivedWindow(ctx, q, wantBars)
	}
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

func (d *DashboardServer) getMicroWindow(spec TimeframeSpec, endTimeMs int64, wantBars int) (HistoryWindow, bool) {
	if endTimeMs <= 0 {
		endTimeMs = time.Now().UnixMilli()
	}
	symbol := ""
	if d != nil {
		symbol = d.symbol
	}
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	dbRows, err := data.LoadMicroKlinesBeforeEnd(symbol, exchange.SecondTF, endTimeMs, wantBars)
	if err != nil {
		log.Printf("[Dashboard] micro_klines load %s 1s: %v", symbol, err)
	}
	dbKlines := exchange.KlinesFromDataCandles(dbRows)
	ramKlines := filterKlinesUntilOpenMs(d.ramKlines(spec.ID, wantBars), endTimeMs)
	merged := exchange.MergeKlineSeries(dbKlines, ramKlines, exchange.AuthoritySettled, exchange.AuthorityFinal)
	if len(merged) == 0 {
		return HistoryWindow{}, false
	}
	if wantBars > 0 && len(merged) > wantBars {
		merged = merged[len(merged)-wantBars:]
	}
	hasMore := false
	if len(merged) > 0 {
		older, herr := data.HasOlderMicroKline(symbol, exchange.SecondTF, merged[0].OpenTime)
		if herr != nil {
			log.Printf("[Dashboard] micro hasMore %s: %v", symbol, herr)
		}
		hasMore = older
	}
	hasNewer := false
	if len(merged) > 0 {
		hasNewer = d.microHasNewerClosed(spec, symbol, merged[len(merged)-1].OpenTime)
	}
	return HistoryWindow{Klines: merged, HasMore: hasMore, HasNewer: hasNewer}, true
}

func (d *DashboardServer) getMicroWindowAfter(spec TimeframeSpec, startTimeMs int64, wantBars int) (HistoryWindow, bool) {
	if startTimeMs <= 0 {
		return HistoryWindow{}, false
	}
	if wantBars <= 0 {
		wantBars = defaultStateCandleLimit
	}
	symbol := ""
	if d != nil {
		symbol = d.symbol
	}
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	dbRows, err := data.LoadMicroKlinesAfterStart(symbol, exchange.SecondTF, startTimeMs, wantBars)
	if err != nil {
		log.Printf("[Dashboard] micro_klines after %s 1s: %v", symbol, err)
	}
	dbKlines := exchange.KlinesFromDataCandles(dbRows)
	ramKlines := filterKlinesAfterOpenMs(d.ramKlines(spec.ID, wantBars+8), startTimeMs)
	merged := exchange.MergeKlineSeries(dbKlines, ramKlines, exchange.AuthoritySettled, exchange.AuthorityFinal)
	if wantBars > 0 && len(merged) > wantBars {
		merged = merged[:wantBars]
	}
	hasMore, herr := data.HasOlderMicroKline(symbol, exchange.SecondTF, startTimeMs)
	if herr != nil {
		log.Printf("[Dashboard] micro hasMore after-cursor %s: %v", symbol, herr)
		hasMore = false
	}
	afterMs := startTimeMs
	if len(merged) > 0 {
		afterMs = merged[len(merged)-1].OpenTime
	}
	hasNewer := d.microHasNewerClosed(spec, symbol, afterMs)
	return HistoryWindow{Klines: merged, HasMore: hasMore, HasNewer: hasNewer}, true
}

func (d *DashboardServer) microHasNewerClosed(spec TimeframeSpec, symbol string, afterOpenMs int64) bool {
	if afterOpenMs <= 0 {
		return false
	}
	newer, err := data.HasNewerMicroKline(symbol, exchange.SecondTF, afterOpenMs)
	if err != nil {
		log.Printf("[Dashboard] micro hasNewer %s: %v", symbol, err)
	}
	if newer {
		return true
	}
	nowMs := time.Now().UnixMilli()
	for _, k := range d.ramKlines(spec.ID, 8) {
		if k.OpenTime > afterOpenMs && isClosedMicroKline(k, nowMs) {
			return true
		}
	}
	return false
}

func isClosedMicroKline(k exchange.Kline, nowMs int64) bool {
	if k.CloseTime > 0 {
		return nowMs > k.CloseTime
	}
	return nowMs > k.OpenTime+999
}

// MICRO-2A: sparse 1s history is micro_klines ∪ RAM overlay; never Cap/Ensure/REST.

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
		if k.OpenTime <= endTimeMs {
			out = append(out, k)
		}
	}
	return out
}

// filterKlinesAfterOpenMs keeps bars with OpenTime > startTimeMs (exclusive).
func filterKlinesAfterOpenMs(klines []exchange.Kline, startTimeMs int64) []exchange.Kline {
	if len(klines) == 0 || startTimeMs <= 0 {
		return klines
	}
	out := make([]exchange.Kline, 0, len(klines))
	for _, k := range klines {
		if k.OpenTime > startTimeMs {
			out = append(out, k)
		}
	}
	return out
}

func (d *DashboardServer) getDerivedWindow(ctx context.Context, q HistoryWindowQuery, wantChildBars int) (HistoryWindow, bool) {
	e, ok := exchange.TimeframeByName(q.Spec.ID)
	if !ok || e.Parent == "" {
		return HistoryWindow{}, false
	}
	parentSpec, err := ResolveTimeframe(e.Parent)
	if err != nil {
		return HistoryWindow{}, false
	}
	parentNeed, err := exchange.ParentBarsNeeded(wantChildBars, e.Name)
	if err != nil {
		return HistoryWindow{}, false
	}
	pwin, pok := d.GetWindow(ctx, HistoryWindowQuery{
		Spec:        parentSpec,
		EndTimeMs:   q.EndTimeMs,
		CandleLimit: parentNeed,
	})
	if !pok || len(pwin.Klines) == 0 {
		return HistoryWindow{}, false
	}
	closed, _, ferr := exchange.FoldClosedChildren(pwin.Klines, e.Name, false)
	if ferr != nil || len(closed) == 0 {
		return HistoryWindow{}, false
	}
	capEnd := resolveClosedBarBoundary(q.EndTimeMs, e.Name)
	closed = filterKlinesUntilOpenMs(closed, capEnd)
	if len(closed) == 0 {
		return HistoryWindow{}, false
	}
	if wantChildBars > 0 && len(closed) > wantChildBars {
		closed = closed[len(closed)-wantChildBars:]
	}
	return HistoryWindow{Klines: closed, HasMore: pwin.HasMore}, true
}
