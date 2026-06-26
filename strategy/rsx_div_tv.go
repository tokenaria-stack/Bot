package strategy

// scanRSXTVMarkers implements TradingView rolling divergence (Jurik RSX Pine div block).
// Price uses close; markers are placed on bar i-1 when divergence is confirmed at bar i (TV offset -1).
func scanRSXTVMarkers(closes, rsx []float64, lookback int) map[int]string {
	markers := make(map[int]string)
	n := len(rsx)
	if n < 3 || len(closes) != n {
		return markers
	}
	if lookback <= 0 {
		lookback = RSXLookbackDefault
	}

	maxCloseHist := make([]float64, n)
	minCloseHist := make([]float64, n)
	var maxClose, maxRSX, minClose, minRSX float64
	var hasMax, hasMin bool

	for i := 0; i < n; i++ {
		hb := highestBarsAgo(rsx, i, lookback)
		lb := lowestBarsAgo(rsx, i, lookback)

		if hb == 0 {
			maxClose = closes[i]
			maxRSX = rsx[i]
			hasMax = true
		} else if !hasMax {
			maxClose = closes[i]
			maxRSX = rsx[i]
			hasMax = true
		}

		if lb == 0 {
			minClose = closes[i]
			minRSX = rsx[i]
			hasMin = true
		} else if !hasMin {
			minClose = closes[i]
			minRSX = rsx[i]
			hasMin = true
		}

		if closes[i] > maxClose {
			maxClose = closes[i]
		}
		if rsx[i] > maxRSX {
			maxRSX = rsx[i]
		}
		if closes[i] < minClose {
			minClose = closes[i]
		}
		if rsx[i] < minRSX {
			minRSX = rsx[i]
		}

		maxCloseHist[i] = maxClose
		minCloseHist[i] = minClose

		if i >= 2 {
			if maxCloseHist[i-1] > maxCloseHist[i-2] &&
				rsx[i-1] < maxRSX &&
				rsx[i] <= rsx[i-1] {
				markers[i-1] = "S"
			}
			if minCloseHist[i-1] < minCloseHist[i-2] &&
				rsx[i-1] > minRSX &&
				rsx[i] >= rsx[i-1] {
				markers[i-1] = "L"
			}
		}
	}
	return markers
}

func highestBarsAgo(values []float64, i, lookback int) int {
	start := i - lookback + 1
	if start < 0 {
		start = 0
	}
	bestIdx := i
	bestVal := values[i]
	for j := start; j <= i; j++ {
		if values[j] > bestVal {
			bestVal = values[j]
			bestIdx = j
		}
	}
	return i - bestIdx
}

func lowestBarsAgo(values []float64, i, lookback int) int {
	start := i - lookback + 1
	if start < 0 {
		start = 0
	}
	bestIdx := i
	bestVal := values[i]
	for j := start; j <= i; j++ {
		if values[j] < bestVal {
			bestVal = values[j]
			bestIdx = j
		}
	}
	return i - bestIdx
}

func (s *rsxMarkerState) appendBarTV(close, rsxVal float64) {
	s.tvCloses = append(s.tvCloses, close)
	s.tvRSX = append(s.tvRSX, rsxVal)

	i := len(s.tvRSX) - 1
	lookback := s.cfg.lookback

	hb := highestBarsAgo(s.tvRSX, i, lookback)
	lb := lowestBarsAgo(s.tvRSX, i, lookback)

	if hb == 0 {
		s.tvMaxClose = close
		s.tvMaxRSX = rsxVal
		s.tvHasMax = true
	} else if !s.tvHasMax {
		s.tvMaxClose = close
		s.tvMaxRSX = rsxVal
		s.tvHasMax = true
	}

	if lb == 0 {
		s.tvMinClose = close
		s.tvMinRSX = rsxVal
		s.tvHasMin = true
	} else if !s.tvHasMin {
		s.tvMinClose = close
		s.tvMinRSX = rsxVal
		s.tvHasMin = true
	}

	if close > s.tvMaxClose {
		s.tvMaxClose = close
	}
	if rsxVal > s.tvMaxRSX {
		s.tvMaxRSX = rsxVal
	}
	if close < s.tvMinClose {
		s.tvMinClose = close
	}
	if rsxVal < s.tvMinRSX {
		s.tvMinRSX = rsxVal
	}

	s.tvMaxCloseHist = append(s.tvMaxCloseHist, s.tvMaxClose)
	s.tvMinCloseHist = append(s.tvMinCloseHist, s.tvMinClose)

	if i >= 2 {
		max1 := s.tvMaxCloseHist[i-1]
		max2 := s.tvMaxCloseHist[i-2]
		min1 := s.tvMinCloseHist[i-1]
		min2 := s.tvMinCloseHist[i-2]
		rsxPrev := s.tvRSX[i-1]
		if max1 > max2 && rsxPrev < s.tvMaxRSX && rsxVal <= rsxPrev {
			s.markers[i-1] = "S"
		}
		if min1 < min2 && rsxPrev > s.tvMinRSX && rsxVal >= rsxPrev {
			s.markers[i-1] = "L"
		}
	}
}
