package indicators

// TVDivergenceFacts replays the canonical RSTVState and returns rsx_tv_div facts only.
func TVDivergenceFacts(closes, rsx []float64, openTimesMs []int64, lookback int) []IndicatorFactEvent {
	return filterRSTVFacts(ReplayRSTVFacts(openTimesMs, closes, rsx, lookback), FactSourceRSXTVDiv)
}

// TVPivotFacts replays the canonical RSTVState and returns rsx_tv_pivot facts only.
func TVPivotFacts(closes, rsx []float64, openTimesMs []int64, lookback int) []IndicatorFactEvent {
	return filterRSTVFacts(ReplayRSTVFacts(openTimesMs, closes, rsx, lookback), FactSourceRSXTVPivot)
}
