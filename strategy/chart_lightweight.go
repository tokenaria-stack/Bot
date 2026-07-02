package strategy

// chartLightweightMatrix enables only RSX for REST chart replay (no MTF/geometry/trendlines).
func chartLightweightMatrix() ScoringMatrix {
	return ScoringMatrix{UseRSX: true}
}

// RSXSettingsEqual reports whether two RSX settings normalize to the same values.
func RSXSettingsEqual(a, b RSXSettings) bool {
	na := NormalizeRSXSettings(a)
	nb := NormalizeRSXSettings(b)
	return na.Length == nb.Length &&
		na.SignalLength == nb.SignalLength &&
		normalizeRSXSource(na.Source) == normalizeRSXSource(nb.Source) &&
		normalizeRSXDivMethod(na.DivMethod) == normalizeRSXDivMethod(nb.DivMethod) &&
		na.DivLookback == nb.DivLookback &&
		na.PivotRadius == nb.PivotRadius &&
		na.MinPriceDeltaRatio == nb.MinPriceDeltaRatio &&
		na.MinOscDelta == nb.MinOscDelta
}
