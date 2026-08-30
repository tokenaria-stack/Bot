package market

import (
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

func (a *Frame) rebuildRSXAnnotationsLocked() {
	a.Annotations = a.Annotations[:0]
}
