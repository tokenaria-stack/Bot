package market

import (
	"trading_bot/indicators"
)

// RSXScanConfigFromSettings maps dashboard RSX settings to the fractal scanner
// used by ScanRSXMarkers / tests. TV and ZigZag facts use their own paths.
func RSXScanConfigFromSettings(s RSXSettings) indicators.RSXScanConfig {
	return fractalScanConfigFromSettings(s)
}

func rsxScanConfigFromSettings(s RSXSettings) indicators.RSXScanConfig {
	return RSXScanConfigFromSettings(s)
}

func (a *Frame) rsxScanConfigLocked() indicators.RSXScanConfig {
	return RSXScanConfigFromSettings(a.effectiveRSXSettings())
}

func (a *Frame) rebuildRSXAnnotationsLocked() {
	a.Annotations = a.Annotations[:0]
	a.rebuildRSTVFactsLocked()
	a.rebuildRSTFractalFactsLocked()
}
