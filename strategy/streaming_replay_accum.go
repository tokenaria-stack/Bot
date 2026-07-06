package strategy

import (
	"context"
	"math"

	"trading_bot/exchange"
)

// StreamingReplayAccumulator holds incremental replay state for REST chart caching.
type StreamingReplayAccumulator struct {
	cfg         StreamingReplayConfig
	marker      *Marker
	chartData   []BacktestChartPoint
	annotations []ChartAnnotation
	prevBlue    float64
	prevBlueReady bool
	prevRSX     float64
	prevRSXReady bool
	processed   int
	firstOpenMs int64
	lastOpenMs  int64
	ctx         context.Context
	lastReplayErr error
}

// LastReplayErr returns the last cancellation/error from processRange (if any).
func (a *StreamingReplayAccumulator) LastReplayErr() error {
	if a == nil {
		return nil
	}
	return a.lastReplayErr
}

// NewStreamingReplayAccumulator starts a fresh accumulator for the given klines.
func NewStreamingReplayAccumulator(klines []exchange.Kline, cfg StreamingReplayConfig) *StreamingReplayAccumulator {
	return NewStreamingReplayAccumulatorCtx(context.Background(), klines, cfg)
}

// NewStreamingReplayAccumulatorCtx is like NewStreamingReplayAccumulator but honors ctx cancellation.
func NewStreamingReplayAccumulatorCtx(ctx context.Context, klines []exchange.Kline, cfg StreamingReplayConfig) *StreamingReplayAccumulator {
	acc := &StreamingReplayAccumulator{cfg: cfg, ctx: ctx}
	acc.resetMarker()
	if len(klines) > 0 {
		acc.firstOpenMs = klines[0].OpenTime
	}
	acc.lastReplayErr = acc.processRange(klines, 0, len(klines))
	return acc
}

// CanExtend reports whether klines continue from the cached tail (same last bar or more bars).
func (a *StreamingReplayAccumulator) CanExtend(settings RSXSettings, klines []exchange.Kline) bool {
	if a == nil || a.processed == 0 || len(klines) == 0 {
		return false
	}
	if !RSXSettingsEqual(a.cfg.RSXSettings, settings) {
		return false
	}
	reqLast := klines[len(klines)-1].OpenTime
	if reqLast < a.lastOpenMs {
		return false
	}
	if reqLast == a.lastOpenMs {
		return len(klines) > a.processed
	}
	// New bars after cached tail — require overlap at lastOpenMs.
	for i := range klines {
		if klines[i].OpenTime == a.lastOpenMs {
			return i+1 == a.processed || len(klines) > a.processed
		}
	}
	return false
}

// TryServeWindow returns a cached slice when the request window ends at the same bar.
func (a *StreamingReplayAccumulator) TryServeWindow(klines []exchange.Kline, settings RSXSettings) (*StreamingReplayResult, bool) {
	if a == nil || len(klines) == 0 || a.processed == 0 {
		return nil, false
	}
	if !RSXSettingsEqual(a.cfg.RSXSettings, settings) {
		return nil, false
	}
	reqLast := klines[len(klines)-1].OpenTime
	if reqLast != a.lastOpenMs || len(a.chartData) < len(klines) {
		return nil, false
	}
	start := len(a.chartData) - len(klines)
	firstSec := exchange.ChartTimeSec(klines[0].OpenTime)
	if a.chartData[start].Time != firstSec {
		return nil, false
	}
	lastSec := exchange.ChartTimeSec(klines[len(klines)-1].OpenTime)
	anns := annotationsInRange(a.annotations, firstSec, lastSec)
	return &StreamingReplayResult{
		ChartPoints: append([]BacktestChartPoint(nil), a.chartData[start:]...),
		Annotations: anns,
	}, true
}

func annotationsInRange(annotations []ChartAnnotation, fromSec, toSec int64) []ChartAnnotation {
	if len(annotations) == 0 {
		return nil
	}
	out := make([]ChartAnnotation, 0, len(annotations))
	for _, ann := range annotations {
		if ann.Time >= fromSec && ann.Time <= toSec {
			out = append(out, ann)
		}
	}
	return out
}

// Extend continues replay from the first new bar after the cached tail.
func (a *StreamingReplayAccumulator) Extend(klines []exchange.Kline) {
	if a == nil {
		return
	}
	from := a.processed
	if from >= len(klines) {
		return
	}
	// Re-align when the request window slid forward but ends at the same cached tail.
	if from > 0 && from <= len(klines) && klines[from-1].OpenTime != a.lastOpenMs {
		for i := range klines {
			if klines[i].OpenTime == a.lastOpenMs {
				from = i + 1
				break
			}
		}
	}
	if from < len(klines) {
		if err := a.processRange(klines, from, len(klines)); err != nil {
			a.lastReplayErr = err
		}
	}
}

// Reset clears state and replays all klines.
func (a *StreamingReplayAccumulator) Reset(klines []exchange.Kline, cfg StreamingReplayConfig) {
	a.cfg = cfg
	a.chartData = a.chartData[:0]
	a.annotations = a.annotations[:0]
	a.processed = 0
	a.firstOpenMs = 0
	a.lastOpenMs = 0
	a.prevBlueReady = false
	a.prevRSXReady = false
	a.resetMarker()
	if len(klines) > 0 {
		a.firstOpenMs = klines[0].OpenTime
	}
	a.lastReplayErr = a.processRange(klines, 0, len(klines))
}

// Result returns the accumulated chart output.
func (a *StreamingReplayAccumulator) Result() *StreamingReplayResult {
	if a == nil {
		return &StreamingReplayResult{}
	}
	return &StreamingReplayResult{
		ChartPoints: append([]BacktestChartPoint(nil), a.chartData...),
		Annotations: append([]ChartAnnotation(nil), a.annotations...),
	}
}

func (a *StreamingReplayAccumulator) resetMarker() {
	a.cfg.RSXSettings = NormalizeRSXSettings(a.cfg.RSXSettings)
	a.marker = NewMarker(nil, nil, a.cfg.Interval, "", a.cfg.ChaosCfg)
	a.marker.ApplyBacktestRSXConfig(a.cfg.RSXSettings)
	a.marker.SetBulkReplayMode(true)
}

func replayCtxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (a *StreamingReplayAccumulator) processRange(klines []exchange.Kline, from, to int) error {
	if a == nil || a.marker == nil || from >= to {
		return nil
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	need := len(a.chartData) + (to - from)
	if cap(a.chartData) < need {
		grow := make([]BacktestChartPoint, len(a.chartData), need)
		copy(grow, a.chartData)
		a.chartData = grow
	}

	for i := from; i < to; i++ {
		if err := replayCtxErr(ctx); err != nil {
			a.lastReplayErr = err
			return err
		}
		kline := klines[i]
		if kline.CloseTime <= 0 {
			kline = exchange.NormalizeKline(kline)
		}
		if !validReplayOHLC(kline.Open, kline.High, kline.Low, kline.Close) {
			continue
		}
		barTimeSec := exchange.ChartTimeSec(kline.OpenTime)

		a.marker.UpdateKlineTick(kline, true)
		falcon := a.marker.FalconSnapshot()

		pt := BacktestChartPoint{
			Time:   barTimeSec,
			Open:   kline.Open,
			High:   kline.High,
			Low:    kline.Low,
			Close:  kline.Close,
			Volume: kline.Volume,
		}
		pt.RSX = falcon.JurikRSX
		pt.Jurik = falcon.JurikRSX
		pt.RSXSignal = falcon.JurikRSXSignal
		populateBacktestPointFromFalcon(&pt, falcon, a.prevBlue, a.prevBlueReady)
		if a.prevRSXReady {
			pt.Color = RSXColor(pt.RSX, a.prevRSX)
		}
		a.prevRSX = pt.RSX
		a.prevRSXReady = true
		a.prevBlue = falcon.BlueLine
		a.prevBlueReady = true

		if len(a.chartData) >= IndicatorWarmupBars-1 {
			a.applyChartAnnotation(&pt, a.marker.BarCount()-1, barTimeSec)
		}

		a.chartData = append(a.chartData, pt)
		a.processed++
		a.lastOpenMs = kline.OpenTime
	}
	return nil
}

func (a *StreamingReplayAccumulator) applyChartAnnotation(pt *BacktestChartPoint, barIndex int, barTimeSec int64) {
	if a.cfg.LightweightMode {
		label := a.marker.ChartMarkerAt(barIndex)
		if ann, ok := rsxAnnotationFromLabel(label, barTimeSec); ok {
			appendStreamingRSXAnnotation(&a.annotations, ann)
			pt.Marker = ann.Label
		}
		return
	}
	if ann, ok := rsxAnnotationFromMarker(a.marker, barTimeSec); ok {
		appendStreamingRSXAnnotation(&a.annotations, ann)
		pt.Marker = ann.Label
	}
}

func rsxAnnotationFromLabel(label string, barTimeSec int64) (ChartAnnotation, bool) {
	if !IsRSXTradingMarker(label) {
		return ChartAnnotation{}, false
	}
	color, position, shape := rsxAnnotationStyle(label)
	return ChartAnnotation{
		Time:     barTimeSec,
		Pane:     normalizeAnnotationPane("rsx"),
		Label:    label,
		Color:    color,
		Position: position,
		Shape:    shape,
	}, true
}

func validReplayOHLC(open, high, low, closePrice float64) bool {
	vals := []float64{open, high, low, closePrice}
	for _, v := range vals {
		if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}
