package indicators

// TVDivergenceFacts walks closed bars with the existing TV rolling detector
// (scanRSXTVHits / rsxTVHitAtDisplayBar) and emits one fact per display bar that
// has a bullish or bearish hit. openTimesMs[i] is kline OpenTime for bar i.
func TVDivergenceFacts(closes, rsx []float64, openTimesMs []int64, lookback int) []IndicatorFactEvent {
	n := len(rsx)
	if n < 3 || len(closes) != n || len(openTimesMs) != n {
		return nil
	}
	out := make([]IndicatorFactEvent, 0)
	for i := 2; i < n; i++ {
		ev, ok := TVDivergenceFactAt(closes, rsx, openTimesMs, i, lookback)
		if ok {
			out = append(out, ev)
		}
	}
	return out
}

// TVDivergenceFactAt maps the windowed TV hit at displayBar onto a fact event.
func TVDivergenceFactAt(closes, rsx []float64, openTimesMs []int64, displayBar, lookback int) (IndicatorFactEvent, bool) {
	n := len(rsx)
	if displayBar < 2 || displayBar >= n || len(closes) != n || len(openTimesMs) != n {
		return IndicatorFactEvent{}, false
	}
	hit := rsxTVHitAtDisplayBar(closes, rsx, displayBar, lookback)
	return factFromTVHit(hit, rsx, closes, openTimesMs)
}

func factFromTVHit(hit RSXMarkerHit, rsx, closes []float64, openTimesMs []int64) (IndicatorFactEvent, bool) {
	dir := tvHitDirection(hit.Label)
	if dir == "" {
		return IndicatorFactEvent{}, false
	}
	pivot := hit.PivotBar
	display := hit.DisplayBar
	if pivot < 0 || display < 0 || pivot >= len(openTimesMs) || display >= len(openTimesMs) {
		return IndicatorFactEvent{}, false
	}
	if display != pivot+1 {
		return IndicatorFactEvent{}, false
	}
	anchorAt := openTimesMs[pivot]
	confirmedAt := openTimesMs[display]
	if confirmedAt <= anchorAt {
		return IndicatorFactEvent{}, false
	}
	return IndicatorFactEvent{
		Source:      FactSourceRSXTVDiv,
		Direction:   dir,
		ConfirmedAt: confirmedAt,
		AnchorAt:    anchorAt,
		AnchorValue: rsx[pivot],
		AnchorPrice: closes[pivot],
	}, true
}

func tvHitDirection(label string) string {
	switch label {
	case "L", "LL":
		return FactDirBullish
	case "S", "SS":
		return FactDirBearish
	default:
		return ""
	}
}
