package indicators

import "sort"

// FractalFacts walks each closed bar with FractalFactsAt (confirm-bar law).
func FractalFacts(prices, rsx []float64, openTimesMs []int64, cfg RSXScanConfig) []IndicatorFactEvent {
	n := len(rsx)
	if n == 0 || len(prices) != n || len(openTimesMs) != n {
		return nil
	}
	cfg = NormalizeRSXScanConfig(cfg)
	cfg.Mode = RSXScanFractal
	out := make([]IndicatorFactEvent, 0)
	for i := 0; i < n; i++ {
		out = append(out, FractalFactsAt(prices, rsx, openTimesMs, cfg, i)...)
	}
	return out
}

// FractalFactsAt emits facts whose first knowable closed bar is displayBar.
// Work is bounded: only pivots that confirm on this bar
// (displayBar-PivotRadius and/or displayBar-MacroPivotRadius), plus an O(lookback)
// search for the previous same-type fractal. It does not rescan 0..displayBar.
func FractalFactsAt(prices, rsx []float64, openTimesMs []int64, cfg RSXScanConfig, displayBar int) []IndicatorFactEvent {
	n := displayBar + 1
	if displayBar < 0 || n > len(rsx) || len(prices) < n || len(openTimesMs) < n {
		return nil
	}
	cfg = NormalizeRSXScanConfig(cfg)
	cfg.Mode = RSXScanFractal
	prices = prices[:n]
	rsx = rsx[:n]
	openTimesMs = openTimesMs[:n]
	radius := cfg.PivotRadius
	if radius <= 0 {
		radius = DefaultRSXPivotRadius
	}
	if n < radius*2+1 {
		return nil
	}

	out := make([]IndicatorFactEvent, 0, 2)
	for _, pivot := range fractalConfirmPivots(displayBar, radius, cfg.MacroPivotRadius) {
		if pivot < radius || pivot+radius >= n {
			continue
		}
		switch {
		case isRSXPivotHigh(rsx, pivot, radius):
			last := previousRSXFractalPivot(rsx, pivot, radius, cfg.Lookback, true)
			hit := fractalHitAtPivot(prices, rsx, last, pivot, PeakHigh, cfg)
			if ev, ok := factFromFractalHitIfConfirm(hit, prices, rsx, openTimesMs, displayBar); ok {
				out = append(out, ev)
			}
		case isRSXPivotLow(rsx, pivot, radius):
			last := previousRSXFractalPivot(rsx, pivot, radius, cfg.Lookback, false)
			hit := fractalHitAtPivot(prices, rsx, last, pivot, PeakLow, cfg)
			if ev, ok := factFromFractalHitIfConfirm(hit, prices, rsx, openTimesMs, displayBar); ok {
				out = append(out, ev)
			}
		}
	}
	return out
}

func fractalConfirmPivots(displayBar, radius, macroRadius int) []int {
	cands := make([]int, 0, 2)
	if radius > 0 {
		cands = append(cands, displayBar-radius)
	}
	if macroRadius > 0 && displayBar-macroRadius != displayBar-radius {
		cands = append(cands, displayBar-macroRadius)
	}
	sort.Ints(cands)
	return cands
}

func factFromFractalHitIfConfirm(hit RSXMarkerHit, prices, rsx []float64, openTimesMs []int64, displayBar int) (IndicatorFactEvent, bool) {
	if hit.PivotBar < 0 || hit.DisplayBar != displayBar {
		return IndicatorFactEvent{}, false
	}
	return factFromFractalHit(hit, prices, rsx, openTimesMs)
}

func factFromFractalHit(hit RSXMarkerHit, prices, rsx []float64, openTimesMs []int64) (IndicatorFactEvent, bool) {
	pivot := hit.PivotBar
	display := hit.DisplayBar
	if pivot < 0 || display < 0 || pivot >= len(openTimesMs) || display >= len(openTimesMs) {
		return IndicatorFactEvent{}, false
	}
	if display <= pivot {
		return IndicatorFactEvent{}, false
	}
	anchorAt := openTimesMs[pivot]
	confirmedAt := openTimesMs[display]
	if confirmedAt <= anchorAt {
		return IndicatorFactEvent{}, false
	}
	if hit.IsPivot {
		dir := FactDirPivotHigh
		if hit.PeakType == PeakLow {
			dir = FactDirPivotLow
		} else if hit.PeakType != PeakHigh {
			return IndicatorFactEvent{}, false
		}
		return IndicatorFactEvent{
			Source:      FactSourceRSXFractalPivot,
			Direction:   dir,
			ConfirmedAt: confirmedAt,
			AnchorAt:    anchorAt,
			AnchorValue: rsx[pivot],
			AnchorPrice: prices[pivot],
		}, true
	}
	pattern, ok := classicClassPattern(hit.Class)
	if !ok {
		return IndicatorFactEvent{}, false
	}
	dir := ""
	switch hit.DivDir {
	case Bullish:
		dir = FactDirBullish
	case Bearish:
		dir = FactDirBearish
	default:
		return IndicatorFactEvent{}, false
	}
	return IndicatorFactEvent{
		Source:      FactSourceRSXFractalDiv,
		Direction:   dir,
		Pattern:     pattern,
		ConfirmedAt: confirmedAt,
		AnchorAt:    anchorAt,
		AnchorValue: rsx[pivot],
		AnchorPrice: prices[pivot],
	}, true
}

func classicClassPattern(c DivClass) (string, bool) {
	switch c {
	case ClassA:
		return FactPatternClassA, true
	case ClassB:
		return FactPatternClassB, true
	case ClassC:
		return FactPatternClassC, true
	default:
		return "", false
	}
}
