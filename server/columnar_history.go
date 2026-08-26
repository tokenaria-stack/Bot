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
	Code          string               `json:"code,omitempty"`
	Timeframe     string               `json:"timeframe"`
	WarmupDropped int                  `json:"warmupDropped"`
	Added         int                  `json:"added"`
	Times         []int64              `json:"times"`
	Candles       columnarCandles      `json:"candles"`
	Plots         map[string][]float64 `json:"plots"`
	Annotations   []wire.Annotation    `json:"annotations"`
	Sentinel      float64              `json:"sentinel"`
	HasMore       bool                 `json:"hasMore"`
	HasNewer      bool                 `json:"hasNewer"`
	// ProjCont is an opt-in ADR-015 probe (DEBUG_PROJ_CONT=1). Safe to ignore when absent.
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
	hasNewer bool,
	timeframe string,
	binanceInterval string,
) (columnarHistoryResponse, bool) {
	_ = ctx
	_ = binanceInterval
	sparseChild := exchange.IsSparseSecondChild(timeframe)
	if d == nil || d.projector == nil {
		return columnarHistoryResponse{}, false
	}
	if len(klines) == 0 && !sparseChild {
		return columnarHistoryResponse{}, false
	}

	if !sparseChild {
		klines = dropFormingTip(klines, time.Now().UnixMilli())
		if len(klines) == 0 {
			return columnarHistoryResponse{}, false
		}
	}

	if len(klines) == 0 {
		return d.packSparseSecondFormingOnly(timeframe, hasMore, hasNewer)
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
		HasNewer:      hasNewer,
	}
	closedBars := len(resp.Times)
	var mode viewportProjectionMode
	if sparseChild {
		mode = d.projectSparseSecondFormingTip(&resp, timeframe)
	} else {
		mode = d.projectViewportFormingTip(&resp, timeframe, binanceInterval)
	}
	if !columnarLenInvariant(resp.Times, resp.Candles, resp.Plots) {
		return columnarHistoryResponse{}, false
	}
	if DebugProjCont() {
		resp.ProjCont = d.buildProjectionContinuityDiag(&resp, timeframe, binanceInterval, closedBars, mode)
		logProjectionContinuity(resp.ProjCont, timeframe)
	}
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
	tip := raw[len(raw)-1]
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
	endTimeMs, endTimeSec, startTimeMs, startTimeSec int64,
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

	resolvedStartMs := startTimeMs
	if resolvedStartMs <= 0 {
		resolvedStartMs = historyEndTimeToMs(startTimeSec)
	}
	resolvedEndMs := endTimeMs
	if resolvedEndMs <= 0 {
		resolvedEndMs = historyEndTimeToMs(endTimeSec)
	}

	q := HistoryWindowQuery{
		Spec:        spec,
		CandleLimit: candleLimit,
	}
	if resolvedStartMs > 0 {
		q.StartTimeMs = resolvedStartMs
	} else {
		q.EndTimeMs = resolvedEndMs
	}

	warmup := market.IndicatorWarmupBars
	if q.StartTimeMs > 0 {
		warmup = 0
	}

	delivered := d.deliverHistoryWindow(r.Context(), q)
	if err := requestCtxErr(r.Context()); err != nil {
		return
	}
	switch delivered.Kind {
	case historyDeliverExchange:
		writeHistoryFault(w, http.StatusBadGateway, HistoryCodeExchangeFailed, delivered.Err)
		return
	case historyDeliverSQLite:
		writeHistoryFault(w, http.StatusInternalServerError, HistoryCodeSQLiteError, delivered.Err)
		return
	case historyDeliverNoData:
		writeHistoryNoData(w, spec.ID, true, delivered.Code)
		return
	}
	win := delivered.Win
	if len(win.Klines) == 0 {
		if q.StartTimeMs > 0 {
			writeJSON(w, columnarHistoryResponse{
				Format:    "columnar",
				Status:    "ready",
				Timeframe: spec.ID,
				Times:     []int64{},
				HasMore:   win.HasMore,
				HasNewer:  win.HasNewer,
			})
			return
		}
		if !exchange.IsSparseSecondChild(spec.ID) {
			writeHistoryNoData(w, spec.ID, true, HistoryCodeNoData)
			return
		}
	}

	resp, ok := d.buildColumnarHistoryPayload(
		r.Context(),
		win.Klines,
		candleLimit,
		warmup,
		rsxSettings,
		slotIDs,
		win.HasMore,
		win.HasNewer,
		spec.ID,
		spec.BinanceInterval,
	)
	if !ok {
		log.Printf("[Dashboard] columnar history empty for %s %s (%d klines)", d.symbol, spec.BinanceInterval, len(win.Klines))
		writeHistoryNoData(w, spec.ID, true, HistoryCodeNoData)
		return
	}
	if err := requestCtxErr(r.Context()); err != nil {
		return
	}
	// Opt-in TipSSOT probe (DEBUG_TIP_SSOT=1) — dormant after ADR-016.
	if DebugTipSSOT() {
		d.logTipSSOTProbe(r.Context(), spec.ID, candleLimit)
	}
	writeJSON(w, resp)
}
