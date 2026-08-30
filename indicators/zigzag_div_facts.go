package indicators

// ZigZagSwingSample is one confirmed same-type swing used for RSX ZigZag divergence.
// RSX must be the oscillator at the swing bar, not at confirmation.
type ZigZagSwingSample struct {
	IsHigh bool
	Price  float64
	RSX    float64
}

// ClassifyZigZagDivergence returns the factual geometry of two same-type swings.
// Equal price or equal RSX is not a divergence. Different high/low families are ignored.
func ClassifyZigZagDivergence(prev, latest ZigZagSwingSample) (direction, pattern string, ok bool) {
	if prev.IsHigh != latest.IsHigh {
		return "", "", false
	}
	if prev.Price == latest.Price || prev.RSX == latest.RSX {
		return "", "", false
	}
	if latest.IsHigh {
		if latest.Price > prev.Price && latest.RSX < prev.RSX {
			return FactDirBearish, FactPatternRegular, true
		}
		if latest.Price < prev.Price && latest.RSX > prev.RSX {
			return FactDirBearish, FactPatternHidden, true
		}
		return "", "", false
	}
	if latest.Price < prev.Price && latest.RSX > prev.RSX {
		return FactDirBullish, FactPatternRegular, true
	}
	if latest.Price > prev.Price && latest.RSX < prev.RSX {
		return FactDirBullish, FactPatternHidden, true
	}
	return "", "", false
}

// ZigZagDivFact builds an rsx_zz_div event. Times are closed-bar OpenTime ms.
func ZigZagDivFact(direction, pattern string, confirmedAt, anchorAt int64, anchorRSX, anchorPrice float64) (IndicatorFactEvent, bool) {
	if direction != FactDirBullish && direction != FactDirBearish {
		return IndicatorFactEvent{}, false
	}
	if pattern != FactPatternRegular && pattern != FactPatternHidden {
		return IndicatorFactEvent{}, false
	}
	if confirmedAt <= 0 || anchorAt <= 0 || confirmedAt <= anchorAt {
		return IndicatorFactEvent{}, false
	}
	return IndicatorFactEvent{
		Source:      FactSourceRSXZZDiv,
		Direction:   direction,
		Pattern:     pattern,
		ConfirmedAt: confirmedAt,
		AnchorAt:    anchorAt,
		AnchorValue: anchorRSX,
		AnchorPrice: anchorPrice,
	}, true
}
