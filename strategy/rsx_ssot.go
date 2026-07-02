package strategy

import (
	"trading_bot/exchange"
	"trading_bot/indicators"
)

// RSXScanConfigFromSettings maps dashboard RSX settings to engine scan configuration.
func RSXScanConfigFromSettings(s RSXSettings) indicators.RSXScanConfig {
	mode := indicators.RSXScanTV
	if normalizeRSXDivMethod(s.DivMethod) == "fractal" {
		mode = indicators.RSXScanFractal
	}
	lookback := s.DivLookback
	if lookback <= 0 {
		lookback = RSXLookbackDefault
	}
	pivotRadius := s.PivotRadius
	if mode == indicators.RSXScanFractal {
		if pivotRadius <= 0 {
			pivotRadius = DefaultRSXPivotRadius
		}
	} else {
		pivotRadius = 0
	}
	return indicators.NormalizeRSXScanConfig(indicators.RSXScanConfig{
		Mode:               mode,
		Lookback:           lookback,
		PivotRadius:        pivotRadius,
		MinPriceDeltaRatio: s.MinPriceDeltaRatio,
		MinOscDelta:        s.MinOscDelta,
	})
}

func rsxScanConfigFromSettings(s RSXSettings) indicators.RSXScanConfig {
	return RSXScanConfigFromSettings(s)
}

func (a *Marker) rsxScanConfigLocked() indicators.RSXScanConfig {
	return a.divEngine.RSXConfig()
}

func (a *Marker) appendRSXAnnotationLocked(ann ChartAnnotation) {
	for i, existing := range a.Annotations {
		if existing.Time == ann.Time && existing.Pane == ann.Pane {
			if rsxMarkerChartStrength(ann.Label) > rsxMarkerChartStrength(existing.Label) {
				a.Annotations[i] = ann
			}
			return
		}
	}
	a.Annotations = append(a.Annotations, ann)
}

func (a *Marker) chartAnnotationFromDivAnn(divAnn indicators.DivAnnotation) ChartAnnotation {
	timeSec := int64(0)
	if divAnn.BarIndex >= 0 && divAnn.BarIndex < len(a.klines) {
		timeSec = a.klines[divAnn.BarIndex].OpenTime / 1000
	}
	return ChartAnnotation{
		Time:     timeSec,
		Pane:     normalizeAnnotationPane("rsx"),
		Label:    divAnn.Label,
		Color:    divAnn.Color,
		Position: divAnn.Position,
		Shape:    divAnn.Shape,
	}
}

func (a *Marker) rsxTradingMarkerAtCurrentBarLocked() string {
	if len(a.klines) == 0 || a.divEngine == nil {
		return ""
	}
	barIndex := len(a.klines) - 1
	return a.divEngine.RSXLabelAtDisplayBar(a, barIndex)
}

func (a *Marker) rebuildRSXAnnotationsLocked() {
	a.Annotations = a.Annotations[:0]
	if len(a.JurikLines) == 0 || a.divEngine == nil {
		return
	}
	for _, hit := range a.divEngine.ScanRSX(a) {
		if hit.Label == "" || hit.DisplayBar < 0 || hit.DisplayBar >= len(a.klines) {
			continue
		}
		a.appendRSXAnnotationLocked(a.chartAnnotationFromDivAnn(indicators.RSXDivAnnotationFromHit(hit)))
	}
}

func (a *Marker) barIndexFromTimeSec(timeSec int64) int {
	for i, k := range a.klines {
		if k.OpenTime/1000 == timeSec {
			return i
		}
	}
	return -1
}

// BuildRSXAnnotationsFromSeries builds RSX chart annotations via unified streaming replay.
func BuildRSXAnnotationsFromSeries(klines []exchange.Kline, rsxValues []float64, settings RSXSettings) []ChartAnnotation {
	_ = rsxValues // legacy param; annotations come from walk-forward scoring factors.
	cfg := ChartStreamingReplayConfig(settings, "")
	result := RunStreamingReplay(nil, klines, cfg)
	return result.Annotations
}

func chartAnnotationsFromRSXHits(klines []exchange.Kline, hits []indicators.RSXMarkerHit) []ChartAnnotation {
	type scored struct {
		hit indicators.RSXMarkerHit
		st  int
	}
	byDisplay := make(map[int]scored)
	for _, hit := range hits {
		if hit.Label == "" || hit.DisplayBar < 0 || hit.DisplayBar >= len(klines) {
			continue
		}
		st := rsxMarkerChartStrength(hit.Label)
		if prev, ok := byDisplay[hit.DisplayBar]; !ok || st > prev.st {
			byDisplay[hit.DisplayBar] = scored{hit: hit, st: st}
		}
	}

	out := make([]ChartAnnotation, 0, len(byDisplay))
	for _, sc := range byDisplay {
		ann := chartAnnotationFromDivAnnStatic(klines, indicators.RSXDivAnnotationFromHit(sc.hit))
		out = append(out, ann)
	}
	return out
}

func chartAnnotationFromDivAnnStatic(klines []exchange.Kline, divAnn indicators.DivAnnotation) ChartAnnotation {
	timeSec := int64(0)
	if divAnn.BarIndex >= 0 && divAnn.BarIndex < len(klines) {
		timeSec = klines[divAnn.BarIndex].OpenTime / 1000
	}
	return ChartAnnotation{
		Time:     timeSec,
		Pane:     normalizeAnnotationPane("rsx"),
		Label:    divAnn.Label,
		Color:    divAnn.Color,
		Position: divAnn.Position,
		Shape:    divAnn.Shape,
	}
}
