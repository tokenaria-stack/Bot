package indicators

// FractalFacts walks closed bars with scanRSXFractalHits and emits one event per
// confirmed fractal divergence or macro pivot. Independent of TV / ZigZag.
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

// FractalFactsAt maps fractal hits whose DisplayBar equals displayBar onto fact events.
// ConfirmedAt = OpenTime of the first closed bar that completes the fractal window.
// AnchorAt = OpenTime of the fractal pivot bar.
func FractalFactsAt(prices, rsx []float64, openTimesMs []int64, cfg RSXScanConfig, displayBar int) []IndicatorFactEvent {
	n := displayBar + 1
	if displayBar < 0 || n > len(rsx) || len(prices) < n || len(openTimesMs) < n {
		return nil
	}
	cfg = NormalizeRSXScanConfig(cfg)
	cfg.Mode = RSXScanFractal
	hits := scanRSXFractalHits(prices[:n], rsx[:n], cfg)
	out := make([]IndicatorFactEvent, 0)
	for _, hit := range hits {
		if hit.DisplayBar != displayBar {
			continue
		}
		ev, ok := factFromFractalHit(hit, prices[:n], rsx[:n], openTimesMs[:n])
		if ok {
			out = append(out, ev)
		}
	}
	return out
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
