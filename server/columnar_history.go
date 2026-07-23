package server

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"trading_bot/core"
	"trading_bot/exchange"
	"trading_bot/market"
	"trading_bot/server/wire"
)

type columnarCandles struct {
	Open   []float64 `json:"open"`
	High   []float64 `json:"high"`
	Low    []float64 `json:"low"`
	Close  []float64 `json:"close"`
	Volume []float64 `json:"volume"`
}

type columnarHistoryResponse struct {
	Format        string               `json:"format"`
	Status        string               `json:"status"`
	Timeframe     string               `json:"timeframe"`
	WarmupDropped int                  `json:"warmupDropped"`
	Added         int                  `json:"added"`
	Times         []int64              `json:"times"`
	Candles       columnarCandles      `json:"candles"`
	Plots         map[string][]float64 `json:"plots"`
	Annotations   []wire.Annotation    `json:"annotations"`
	Sentinel      float64              `json:"sentinel"`
	HasMore       bool                 `json:"hasMore"`
	// ProjCont is a temporary ADR-015 probe (safe to ignore in consumers).
	ProjCont *projectionContinuityDiag `json:"projCont,omitempty"`
}

// projectionContinuityDiag — ADR-015 scientific probe (History vs store vs first WS).
type projectionContinuityDiag struct {
	ClosedBars       int     `json:"closedBars"`
	ProjectedForming bool    `json:"projectedForming"`
	ProjectionMode   string  `json:"projectionMode,omitempty"` // none|append|overwrite
	TimesLen         int     `json:"timesLen"`
	PlotsLen         int     `json:"plotsLen"`
	LastOpenSec      int64   `json:"lastOpenSec"`
	LastRSX          float64 `json:"lastRSX"`
	FrameCurOpenSec  int64   `json:"frameCurOpenSec,omitempty"`
	FrameCurRSX      float64 `json:"frameCurRSX"`
	FrameCurPresent  bool    `json:"frameCurPresent"`
}

func parseSlotsParam(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// dropFormingTip removes the last kline when it has not yet closed (Shot 11A History Tip Protocol).
// Law: last bar belongs to History (closed) XOR Live (forming) — never both.
// Forming predicate: CloseTime > 0 && nowMs <= CloseTime.
// Empty input / sole forming bar → nil/empty slice (caller must treat as unavailable).
func dropFormingTip(klines []exchange.Kline, nowMs int64) []exchange.Kline {
	n := len(klines)
	if n == 0 {
		return klines
	}
	last := klines[n-1]
	if last.CloseTime > 0 && nowMs <= last.CloseTime {
		if n == 1 {
			return nil
		}
		return klines[:n-1]
	}
	return klines
}

func (d *DashboardServer) buildColumnarHistoryPayload(
	ctx context.Context,
	klines []exchange.Kline,
	candleLimit int,
	warmupBars int,
	rsxSettings market.RSXSettings,
	slotIDs []string,
	hasMore bool,
	timeframe string,
	binanceInterval string,
) (columnarHistoryResponse, bool) {
	_ = ctx
	_ = binanceInterval
	if d == nil || d.projector == nil || len(klines) == 0 {
		return columnarHistoryResponse{}, false
	}

	// Shot 11A / ADR-010: strip forming tip before Replay (History stays closed-only).
	// Viewport may re-attach Frame's forming tip after projection (TV Model 2).
	klines = dropFormingTip(klines, time.Now().UnixMilli())
	if len(klines) == 0 {
		return columnarHistoryResponse{}, false
	}

	trimBars := historyWarmupTrim(len(klines), candleLimit, warmupBars)

	// Drop leading warmup bars before display window; client never sees warmup prefix.
	display := klines
	if trimBars > 0 && len(display) > trimBars {
		display = display[trimBars:]
	}
	if candleLimit > 0 && len(display) > candleLimit {
		display = display[len(display)-candleLimit:]
	}
	if len(display) == 0 {
		return columnarHistoryResponse{}, false
	}

	// Closed-only stream: ReplayDAGKlines must never see the forming tip.
	hist := market.ReplayDAGKlines(klines, rsxSettings)
	times := columnarTimesFromKlines(display)
	plots, sentinel := d.projector.BuildHistoryColumnsFiltered(hist, times, slotIDs)
	annotations := d.projector.BuildHistoryAnnotations(hist, times)
	if annotations == nil {
		annotations = []wire.Annotation{}
	}

	n := len(display)
	candles := columnarCandles{
		Open:   make([]float64, n),
		High:   make([]float64, n),
		Low:    make([]float64, n),
		Close:  make([]float64, n),
		Volume: make([]float64, n),
	}
	for i, k := range display {
		candles.Open[i] = k.Open
		candles.High[i] = k.High
		candles.Low[i] = k.Low
		candles.Close[i] = k.Close
		candles.Volume[i] = k.Volume
	}

	if !columnarLenInvariant(times, candles, plots) {
		return columnarHistoryResponse{}, false
	}

	resp := columnarHistoryResponse{
		Format:        "columnar",
		Status:        "ready",
		Timeframe:     timeframe,
		WarmupDropped: trimBars,
		Added:         n,
		Times:         times,
		Candles:       candles,
		Plots:         plots,
		Annotations:   annotations,
		Sentinel:      sentinel,
		HasMore:       hasMore,
	}
	closedBars := len(resp.Times)
	// ADR-010 / B2.2: APPEND new forming bar or OVERWRITE Cap tip with Frame Cur.
	mode := d.projectViewportFormingTip(&resp, timeframe, binanceInterval)
	if !columnarLenInvariant(resp.Times, resp.Candles, resp.Plots) {
		return columnarHistoryResponse{}, false
	}
	resp.ProjCont = d.buildProjectionContinuityDiag(&resp, timeframe, binanceInterval, closedBars, mode)
	logProjectionContinuity(resp.ProjCont, timeframe)
	return resp, true
}

func tipRSXFromPlots(plots map[string][]float64) float64 {
	if plots == nil {
		return 0
	}
	col := plots["line_rsx"]
	if len(col) == 0 {
		col = plots["jurik_rsx"]
	}
	if len(col) == 0 {
		return 0
	}
	return col[len(col)-1]
}

func plotColumnLen(plots map[string][]float64) int {
	if plots == nil {
		return 0
	}
	if col, ok := plots["line_rsx"]; ok {
		return len(col)
	}
	for _, col := range plots {
		return len(col)
	}
	return 0
}

func (d *DashboardServer) buildProjectionContinuityDiag(
	resp *columnarHistoryResponse,
	timeframe, binanceInterval string,
	closedBars int,
	mode viewportProjectionMode,
) *projectionContinuityDiag {
	if resp == nil || len(resp.Times) == 0 {
		return nil
	}
	diag := &projectionContinuityDiag{
		ClosedBars:       closedBars,
		ProjectedForming: mode == viewportProjAppend || mode == viewportProjOverwrite,
		ProjectionMode:   string(mode),
		TimesLen:         len(resp.Times),
		PlotsLen:         plotColumnLen(resp.Plots),
		LastOpenSec:      resp.Times[len(resp.Times)-1],
		LastRSX:          tipRSXFromPlots(resp.Plots),
		FrameCurRSX:      0,
	}
	frame := d.frameForTimeframe(timeframe)
	if frame == nil && binanceInterval != "" && binanceInterval != timeframe {
		frame = d.frameForTimeframe(binanceInterval)
	}
	if frame == nil {
		return diag
	}
	raw := frame.GetKlines()
	if len(raw) == 0 {
		return diag
	}
	tip := exchange.NormalizeKline(raw[len(raw)-1])
	diag.FrameCurOpenSec = exchange.ChartTimeSec(tip.OpenTime)
	if dag := frame.DAGTickFrame(); dag != nil {
		diag.FrameCurPresent = true
		diag.FrameCurRSX = dag.Get(core.SlotJurikRSX)
	}
	return diag
}

func logProjectionContinuity(diag *projectionContinuityDiag, tf string) {
	if diag == nil {
		return
	}
	log.Printf("[ProjCont] tf=%s mode=%s closedBars=%d projectedForming=%v timesLen=%d plotsLen=%d "+
		"lastOpen=%d lastRSX=%.8f frameCurPresent=%v frameCurOpen=%d frameCurRSX=%.8f",
		tf, diag.ProjectionMode, diag.ClosedBars, diag.ProjectedForming, diag.TimesLen, diag.PlotsLen,
		diag.LastOpenSec, diag.LastRSX, diag.FrameCurPresent, diag.FrameCurOpenSec, diag.FrameCurRSX)
}

// filterAnnotationsByDisplayTimes keeps markers whose time is an exact member of display times.
func filterAnnotationsByDisplayTimes(annotations []wire.Annotation, times []int64) []wire.Annotation {
	if len(annotations) == 0 || len(times) == 0 {
		return []wire.Annotation{}
	}
	allowed := make(map[int64]struct{}, len(times))
	for _, t := range times {
		allowed[t] = struct{}{}
	}
	out := make([]wire.Annotation, 0, len(annotations))
	for _, ann := range annotations {
		if _, ok := allowed[ann.Time]; ok {
			out = append(out, ann)
		}
	}
	return out
}

func columnarTimesFromKlines(klines []exchange.Kline) []int64 {
	times := make([]int64, len(klines))
	for i, k := range klines {
		times[i] = exchange.ChartTimeSec(k.OpenTime)
	}
	return times
}

func columnarLenInvariant(times []int64, candles columnarCandles, plots map[string][]float64) bool {
	n := len(times)
	if n == 0 {
		return false
	}
	if len(candles.Open) != n || len(candles.High) != n || len(candles.Low) != n ||
		len(candles.Close) != n || len(candles.Volume) != n {
		return false
	}
	for _, col := range plots {
		if len(col) != n {
			return false
		}
	}
	return true
}

// columnarSentinel exposes wire sentinel for tests.
func columnarSentinel() float64 {
	return wire.HistoryAbsent
}

func (d *DashboardServer) writeColumnarHistory(
	w http.ResponseWriter,
	r *http.Request,
	spec TimeframeSpec,
	endTimeMs, endTimeSec int64,
	rsxSettings market.RSXSettings,
	candleLimit int,
	slotIDs []string,
) {
	if d.projector == nil {
		http.Error(w, "ui projector unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := requestCtxErr(r.Context()); err != nil {
		return
	}
	if symbol := strings.TrimSpace(r.URL.Query().Get("symbol")); symbol != "" {
		if exchange.NormalizeFuturesSymbol(symbol) != d.symbol {
			http.Error(w, "symbol mismatch", http.StatusBadRequest)
			return
		}
	}

	warmup := market.IndicatorWarmupBars
	resolvedEndMs := endTimeMs
	if resolvedEndMs <= 0 {
		resolvedEndMs = historyEndTimeToMs(endTimeSec)
	}

	win, okWin := d.GetWindow(r.Context(), HistoryWindowQuery{
		Spec:        spec,
		EndTimeMs:   resolvedEndMs,
		CandleLimit: candleLimit,
	})
	if err := requestCtxErr(r.Context()); err != nil {
		return
	}
	if !okWin || len(win.Klines) == 0 {
		http.Error(w, "no historical data available", http.StatusServiceUnavailable)
		return
	}

	resp, ok := d.buildColumnarHistoryPayload(
		r.Context(),
		win.Klines,
		candleLimit,
		warmup,
		rsxSettings,
		slotIDs,
		win.HasMore,
		spec.ID,
		spec.BinanceInterval,
	)
	if !ok {
		log.Printf("[Dashboard] columnar history empty for %s %s (%d klines)", d.symbol, spec.BinanceInterval, len(win.Klines))
		http.Error(w, "history replay empty", http.StatusServiceUnavailable)
		return
	}
	if err := requestCtxErr(r.Context()); err != nil {
		return
	}
	// Real data-plane probe (#67): last-closed GetWindow vs Frame — log only, no clamp.
	d.logTipSSOTProbe(r.Context(), spec.ID, candleLimit)
	writeJSON(w, resp)
}
