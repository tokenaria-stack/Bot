package market

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

func (a *Frame) rsxScanConfigLocked() indicators.RSXScanConfig {
	return a.divEngine.RSXConfig()
}

// appendRSXAnnotationLocked is a Phase F no-op (RSX trading labels not published).
func (a *Frame) appendRSXAnnotationLocked(ann ChartAnnotation) {
	_ = a
	_ = ann
}

func (a *Frame) rebuildRSXAnnotationsLocked() {
	a.Annotations = a.Annotations[:0]
}

func (a *Frame) barIndexFromTimeSec(timeSec int64) int {
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

// chartAnnotationsFromRSXHits is a Phase F no-op: divergence math stays in indicators/,
// but trading labels are not delivered to the chart surface.
func chartAnnotationsFromRSXHits(klines []exchange.Kline, hits []indicators.RSXMarkerHit) []ChartAnnotation {
	_ = klines
	_ = hits
	return nil
}
