package market

// RSXSettingsEqual reports whether two RSX settings normalize to the same values.
func RSXSettingsEqual(a, b RSXSettings) bool {
	na := NormalizeRSXSettings(a)
	nb := NormalizeRSXSettings(b)
	return na.Length == nb.Length &&
		na.SignalLength == nb.SignalLength &&
		normalizeRSXSource(na.Source) == normalizeRSXSource(nb.Source) &&
		na.DivLookback == nb.DivLookback &&
		na.PivotRadius == nb.PivotRadius &&
		na.MinPriceDeltaRatio == nb.MinPriceDeltaRatio &&
		na.MinOscDelta == nb.MinOscDelta
}
