package indicators

import "math"

// TVPivotFacts walks closed bars with the Pine TV pivot law (rolling max_rsi/min_rsi,
// 2-bar right confirmation, plot offset=-2). Not fractal P / ZigZag.
func TVPivotFacts(closes, rsx []float64, openTimesMs []int64, lookback int) []IndicatorFactEvent {
	n := len(rsx)
	if n < 4 || len(closes) != n || len(openTimesMs) != n {
		return nil
	}
	out := make([]IndicatorFactEvent, 0)
	for i := 3; i < n; i++ {
		out = append(out, TVPivotFactsAt(closes, rsx, openTimesMs, i, lookback)...)
	}
	return out
}

// TVPivotFactsAt maps Pine pivoth/pivotl at the confirming bar onto fact events.
// ConfirmedAt = displayBar OpenTime; AnchorAt = displayBar-2 OpenTime.
func TVPivotFactsAt(closes, rsx []float64, openTimesMs []int64, displayBar, lookback int) []IndicatorFactEvent {
	n := displayBar + 1
	if displayBar < 3 || n > len(rsx) || len(closes) < n || len(openTimesMs) < n {
		return nil
	}
	maxRSI, minRSI := tvMaxMinRSISeries(closes[:n], rsx[:n], lookback)
	return tvPivotsFromRoll(maxRSI, minRSI, rsx[:n], closes[:n], openTimesMs[:n], displayBar)
}

func tvPivotsFromRoll(maxRSI, minRSI, rsx, closes []float64, openTimesMs []int64, i int) []IndicatorFactEvent {
	if i < 3 || i >= len(maxRSI) {
		return nil
	}
	out := make([]IndicatorFactEvent, 0, 2)
	if tvPinePivot(maxRSI, i) {
		if ev, ok := tvPivotEvent(FactDirPivotHigh, rsx, closes, openTimesMs, i); ok {
			out = append(out, ev)
		}
	}
	if tvPinePivot(minRSI, i) {
		if ev, ok := tvPivotEvent(FactDirPivotLow, rsx, closes, openTimesMs, i); ok {
			out = append(out, ev)
		}
	}
	return out
}

func tvPinePivot(roll []float64, i int) bool {
	if i < 3 || i >= len(roll) {
		return false
	}
	a, b, c := roll[i], roll[i-2], roll[i-3]
	if math.IsNaN(a) || math.IsNaN(b) || math.IsNaN(c) {
		return false
	}
	return a == b && b != c
}

func tvPivotEvent(dir string, rsx, closes []float64, openTimesMs []int64, confirmBar int) (IndicatorFactEvent, bool) {
	anchorBar := confirmBar - 2
	if anchorBar < 0 || confirmBar >= len(openTimesMs) {
		return IndicatorFactEvent{}, false
	}
	anchorAt := openTimesMs[anchorBar]
	confirmedAt := openTimesMs[confirmBar]
	if confirmedAt <= anchorAt {
		return IndicatorFactEvent{}, false
	}
	return IndicatorFactEvent{
		Source:      FactSourceRSXTVPivot,
		Direction:   dir,
		ConfirmedAt: confirmedAt,
		AnchorAt:    anchorAt,
		AnchorValue: rsx[anchorBar],
		AnchorPrice: closes[anchorBar],
	}, true
}

// tvMaxMinRSISeries is the Pine rolling max_rsi / min_rsi path used by TV pivots
// (same hb/lb + carry/update order as the TV divergence scanner).
func tvMaxMinRSISeries(closes, rsx []float64, lookback int) (maxRSI, minRSI []float64) {
	n := len(rsx)
	if n == 0 || len(closes) != n {
		return nil, nil
	}
	if lookback <= 0 {
		lookback = DefaultRSXLookback
	}
	maxRSI = make([]float64, n)
	minRSI = make([]float64, n)
	var maxRSX, minRSX float64
	var hasMax, hasMin bool
	for i := 0; i < n; i++ {
		hb := highestBarsAgo(rsx, i, lookback)
		lb := lowestBarsAgo(rsx, i, lookback)
		if hb == 0 {
			maxRSX = rsx[i]
			hasMax = true
		} else if !hasMax {
			maxRSX = rsx[i]
			hasMax = true
		}
		if lb == 0 {
			minRSX = rsx[i]
			hasMin = true
		} else if !hasMin {
			minRSX = rsx[i]
			hasMin = true
		}
		if rsx[i] > maxRSX {
			maxRSX = rsx[i]
		}
		if rsx[i] < minRSX {
			minRSX = rsx[i]
		}
		maxRSI[i] = maxRSX
		minRSI[i] = minRSX
	}
	return maxRSI, minRSI
}

// TVFactsAt returns divergence and/or pivot facts confirmed on displayBar.
func TVFactsAt(closes, rsx []float64, openTimesMs []int64, displayBar, lookback int) []IndicatorFactEvent {
	var divs []IndicatorFactEvent
	if ev, ok := TVDivergenceFactAt(closes, rsx, openTimesMs, displayBar, lookback); ok {
		divs = []IndicatorFactEvent{ev}
	}
	return MergeTVFacts(divs, TVPivotFactsAt(closes, rsx, openTimesMs, displayBar, lookback))
}
