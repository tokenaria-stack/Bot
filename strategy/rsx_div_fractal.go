package strategy

import (
	"trading_bot/indicators"
)

func isRSXPivotHigh(rsx []float64, i int) bool {
	radius := RSXPivotRadius()
	if i < radius || i+radius >= len(rsx) {
		return false
	}
	v := rsx[i]
	for j := i - radius; j <= i+radius; j++ {
		if j != i && rsx[j] >= v {
			return false
		}
	}
	return true
}

func isRSXPivotLow(rsx []float64, i int) bool {
	radius := RSXPivotRadius()
	if i < radius || i+radius >= len(rsx) {
		return false
	}
	v := rsx[i]
	for j := i - radius; j <= i+radius; j++ {
		if j != i && rsx[j] <= v {
			return false
		}
	}
	return true
}

func isRSXMacroPivotHigh(rsx []float64, i int) bool {
	if i < rsxMacroPivotRadius || i+rsxMacroPivotRadius >= len(rsx) {
		return false
	}
	v := rsx[i]
	for j := i - rsxMacroPivotRadius; j <= i+rsxMacroPivotRadius; j++ {
		if j != i && rsx[j] >= v {
			return false
		}
	}
	return true
}

func isRSXMacroPivotLow(rsx []float64, i int) bool {
	if i < rsxMacroPivotRadius || i+rsxMacroPivotRadius >= len(rsx) {
		return false
	}
	v := rsx[i]
	for j := i - rsxMacroPivotRadius; j <= i+rsxMacroPivotRadius; j++ {
		if j != i && rsx[j] <= v {
			return false
		}
	}
	return true
}

// scanRSXFractalMarkers assigns at most one marker per confirmed RSX pivot (radius on each side).
// Divergence markers (S/L/SS/LL) use lookback; plain P requires a 15-bar macro extremum (±7).
func scanRSXFractalMarkers(prices, rsx []float64, lookback int) map[int]string {
	markers := make(map[int]string)
	radius := RSXPivotRadius()
	if len(rsx) < radius*2+1 {
		return markers
	}
	if lookback <= 0 {
		lookback = GetRSXSettings().DivLookback
	}

	lastPivotHigh := -1
	lastPivotLow := -1

	for i := radius; i+radius < len(rsx); i++ {
		switch {
		case isRSXPivotHigh(rsx, i) && rsx[i] > RSXZoneHigh:
			marker := ""
			if lastPivotHigh >= 0 && i-lastPivotHigh <= lookback {
				div := checkPivotDivergence(prices, rsx, lastPivotHigh, i, indicators.PeakHigh)
				if div.Direction == indicators.Bearish {
					marker = bearishRSXMarker(div)
				}
			}
			if marker == "" && isRSXMacroPivotHigh(rsx, i) {
				marker = "P"
			}
			if marker != "" {
				markers[i] = marker
			}
			lastPivotHigh = i

		case isRSXPivotLow(rsx, i) && rsx[i] < RSXZoneLow:
			marker := ""
			if lastPivotLow >= 0 && i-lastPivotLow <= lookback {
				div := checkPivotDivergence(prices, rsx, lastPivotLow, i, indicators.PeakLow)
				if div.Direction == indicators.Bullish {
					marker = bullishRSXMarker(div)
				}
			}
			if marker == "" && isRSXMacroPivotLow(rsx, i) {
				marker = "P"
			}
			if marker != "" {
				markers[i] = marker
			}
			lastPivotLow = i
		}
	}
	return markers
}

func checkPivotDivergence(prices, rsx []float64, idx1, idx2 int, peakType indicators.PeakType) indicators.DivergenceResult {
	pricePeaks := []indicators.Peak{
		{Index: idx1, Value: prices[idx1], Type: peakType},
		{Index: idx2, Value: prices[idx2], Type: peakType},
	}
	oscPeaks := []indicators.Peak{
		{Index: idx1, Value: rsx[idx1], Type: peakType},
		{Index: idx2, Value: rsx[idx2], Type: peakType},
	}
	return indicators.CheckClassicDivergence(pricePeaks, oscPeaks, rsxPeakIndexTolerance)
}

func bearishRSXMarker(div indicators.DivergenceResult) string {
	if div.Class == indicators.ClassA || div.Class == indicators.ClassC {
		return "SS"
	}
	return "S"
}

func bullishRSXMarker(div indicators.DivergenceResult) string {
	if div.Class == indicators.ClassA || div.Class == indicators.ClassC {
		return "LL"
	}
	return "L"
}

func (s *rsxMarkerState) markFractalPivotAt(i int) {
	radius := RSXPivotRadius()
	if i < radius || i+radius >= len(s.rsx) {
		return
	}

	switch {
	case isRSXPivotHigh(s.rsx, i) && s.rsx[i] > RSXZoneHigh:
		marker := ""
		if s.lastPivotHigh >= 0 && i-s.lastPivotHigh <= s.lookback {
			div := checkPivotDivergence(s.prices, s.rsx, s.lastPivotHigh, i, indicators.PeakHigh)
			if div.Direction == indicators.Bearish {
				marker = bearishRSXMarker(div)
			}
		}
		if marker == "" && i+rsxMacroPivotRadius < len(s.rsx) && isRSXMacroPivotHigh(s.rsx, i) {
			marker = "P"
		}
		if marker != "" {
			s.markers[i] = marker
		}
		s.lastPivotHigh = i

	case isRSXPivotLow(s.rsx, i) && s.rsx[i] < RSXZoneLow:
		marker := ""
		if s.lastPivotLow >= 0 && i-s.lastPivotLow <= s.lookback {
			div := checkPivotDivergence(s.prices, s.rsx, s.lastPivotLow, i, indicators.PeakLow)
			if div.Direction == indicators.Bullish {
				marker = bullishRSXMarker(div)
			}
		}
		if marker == "" && i+rsxMacroPivotRadius < len(s.rsx) && isRSXMacroPivotLow(s.rsx, i) {
			marker = "P"
		}
		if marker != "" {
			s.markers[i] = marker
		}
		s.lastPivotLow = i
	}
}

func (s *rsxMarkerState) tryFractalMacroOnlyMarker(i int) {
	if _, ok := s.markers[i]; ok {
		return
	}
	switch {
	case isRSXPivotHigh(s.rsx, i) && s.rsx[i] > RSXZoneHigh && isRSXMacroPivotHigh(s.rsx, i):
		s.markers[i] = "P"
	case isRSXPivotLow(s.rsx, i) && s.rsx[i] < RSXZoneLow && isRSXMacroPivotLow(s.rsx, i):
		s.markers[i] = "P"
	}
}
