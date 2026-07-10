package server

import (
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
	klines []exchange.Kline,
	candleLimit int,
	warmupBars int,
	rsxSettings strategy.RSXSettings,
	slotIDs []string,
	hasMore bool,
	timeframe string,
) (columnarHistoryResponse, bool) {
	if d == nil || d.projector == nil || len(klines) == 0 {
		return columnarHistoryResponse{}, false
	}

	trimBars := historyWarmupTrim(len(klines), candleLimit, warmupBars)

	display := klines
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

	return columnarHistoryResponse{
		Format:        "columnar",
		Status:        "ready",
		Timeframe:     timeframe,
		WarmupDropped: trimBars,
		Added:         n,
		Times:         times,
		Candles:       candles,
		Plots:         plots,
		Annotations:   []strategy.ChartAnnotation{},
		Sentinel:      sentinel,
		HasMore:       hasMore,
	}, true
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
	var klines []exchange.Kline
	var hasMore bool

	if spec.Kind == TFRAMOnly {
		klines = d.ramKlines(spec.ID, candleLimit+warmup)
		hasMore = false
	} else {
		resolvedEndMs := endTimeMs
		if resolvedEndMs <= 0 {
			resolvedEndMs = historyEndTimeToMs(endTimeSec)
		}
		klines = d.loadRESTKlinesFromStore(r.Context(), spec, resolvedEndMs, candleLimit, true)
		if err := requestCtxErr(r.Context()); err != nil {
			return
		}
		if len(klines) == 0 {
			http.Error(w, "no historical data available", http.StatusServiceUnavailable)
			return
		}
		hasMore = d.sqliteHasBarsBefore(spec.BinanceInterval, exchange.ChartTimeSec(klines[0].OpenTime)*1000)
	}

	resp, ok := d.buildColumnarHistoryPayload(klines, candleLimit, warmup, rsxSettings, slotIDs, hasMore, spec.ID)
	if !ok {
		log.Printf("[Dashboard] columnar history empty for %s %s (%d klines)", d.symbol, spec.BinanceInterval, len(klines))
		http.Error(w, "history replay empty", http.StatusServiceUnavailable)
		return
	}
	if err := requestCtxErr(r.Context()); err != nil {
		return
	}
	log.Printf("[Dashboard] columnar history %s %s: %d bars (from %d klines) slots=%d hasMore=%v",
		d.symbol, spec.BinanceInterval, resp.Added, len(klines), len(resp.Plots), resp.HasMore)
	writeJSON(w, resp)
}
