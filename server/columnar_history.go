package server

import (
	"context"
	"log"
	"net/http"
	"strings"

	"trading_bot/exchange"
	"trading_bot/server/wire"
	"trading_bot/strategy"
)

type columnarCandles struct {
	Open   []float64 `json:"open"`
	High   []float64 `json:"high"`
	Low    []float64 `json:"low"`
	Close  []float64 `json:"close"`
	Volume []float64 `json:"volume"`
}

type columnarHistoryResponse struct {
	Format        string                     `json:"format"`
	Status        string                     `json:"status"`
	Timeframe     string                     `json:"timeframe"`
	WarmupDropped int                        `json:"warmupDropped"`
	Added         int                        `json:"added"`
	Times         []int64                    `json:"times"`
	Candles       columnarCandles            `json:"candles"`
	Plots         map[string][]float64       `json:"plots"`
	Annotations   []strategy.ChartAnnotation `json:"annotations"`
	Sentinel      float64                    `json:"sentinel"`
	HasMore       bool                       `json:"hasMore"`
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

func (d *DashboardServer) buildColumnarHistoryPayload(
	ctx context.Context,
	klines []exchange.Kline,
	candleLimit int,
	warmupBars int,
	rsxSettings strategy.RSXSettings,
	slotIDs []string,
	hasMore bool,
	timeframe string,
	binanceInterval string,
) (columnarHistoryResponse, bool) {
	if d == nil || d.projector == nil || len(klines) == 0 {
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

	hist := strategy.ReplayDAGKlines(klines, rsxSettings)
	plots, sentinel := d.projector.BuildHistoryColumnsFiltered(hist, columnarTimesFromKlines(display), slotIDs)

	n := len(display)
	times := make([]int64, n)
	candles := columnarCandles{
		Open:   make([]float64, n),
		High:   make([]float64, n),
		Low:    make([]float64, n),
		Close:  make([]float64, n),
		Volume: make([]float64, n),
	}
	for i, k := range display {
		times[i] = exchange.ChartTimeSec(k.OpenTime)
		candles.Open[i] = k.Open
		candles.High[i] = k.High
		candles.Low[i] = k.Low
		candles.Close[i] = k.Close
		candles.Volume[i] = k.Volume
	}

	if !columnarLenInvariant(times, candles, plots) {
		return columnarHistoryResponse{}, false
	}

	legacyAnns := legacyChartAnnotationsFromKlines(ctx, klines, trimBars, binanceInterval, rsxSettings)
	annotations := filterAnnotationsByDisplayTimes(legacyAnns, times)
	if annotations == nil {
		annotations = []strategy.ChartAnnotation{}
	}

	return columnarHistoryResponse{
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
	}, true
}

// legacyChartAnnotationsFromKlines runs Falcon streaming replay on the same kline window
// used for DAG columnar output and returns warmup-trimmed RSX divergence markers.
func legacyChartAnnotationsFromKlines(
	ctx context.Context,
	klines []exchange.Kline,
	trimBars int,
	interval string,
	settings strategy.RSXSettings,
) []strategy.ChartAnnotation {
	if len(klines) == 0 {
		return nil
	}
	if err := requestCtxErr(ctx); err != nil {
		return nil
	}
	cfg := strategy.ChartStreamingReplayConfig(settings, interval)
	acc := strategy.NewStreamingReplayAccumulatorCtx(ctx, klines, cfg)
	if err := acc.LastReplayErr(); err != nil {
		return nil
	}
	_, _, annotations := chartSeriesFromReplayResult(acc.Result(), true)
	return trimAnnotations(annotations, trimBars, klines)
}

// filterAnnotationsByDisplayTimes keeps markers whose time is an exact member of display times.
func filterAnnotationsByDisplayTimes(annotations []strategy.ChartAnnotation, times []int64) []strategy.ChartAnnotation {
	if len(annotations) == 0 || len(times) == 0 {
		return []strategy.ChartAnnotation{}
	}
	allowed := make(map[int64]struct{}, len(times))
	for _, t := range times {
		allowed[t] = struct{}{}
	}
	out := make([]strategy.ChartAnnotation, 0, len(annotations))
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
	rsxSettings strategy.RSXSettings,
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

	warmup := strategy.IndicatorWarmupBars
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
	log.Printf("[Dashboard] columnar history %s %s: %d bars (from %d klines) slots=%d anns=%d hasMore=%v",
		d.symbol, spec.BinanceInterval, resp.Added, len(win.Klines), len(resp.Plots), len(resp.Annotations), resp.HasMore)
	writeJSON(w, resp)
}
