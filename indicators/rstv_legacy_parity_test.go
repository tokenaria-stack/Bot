package indicators

import "testing"

// Frozen pre-ONE-BRAIN full Everget walk (div only). Production scanRSXTVHits is now a wrapper.
func legacyScanRSXTVHits(closes, rsx []float64, lookback int) []RSXMarkerHit {
	n := len(rsx)
	if n < 3 || len(closes) != n {
		return nil
	}
	if lookback <= 0 {
		lookback = DefaultRSXLookback
	}
	var hits []RSXMarkerHit
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
			cfg := RSXScanConfig{Mode: RSXScanTV}
			if maxCloseHist[i-1] > maxCloseHist[i-2] && rsx[i-1] < maxRSX && rsx[i] <= rsx[i-1] {
				pivot := i - 1
				hits = append(hits, RSXMarkerHit{PivotBar: pivot, DisplayBar: rsxDisplayBar(pivot, "S", cfg), Label: "S"})
			}
			if minCloseHist[i-1] < minCloseHist[i-2] && rsx[i-1] > minRSX && rsx[i] >= rsx[i-1] {
				pivot := i - 1
				hits = append(hits, RSXMarkerHit{PivotBar: pivot, DisplayBar: rsxDisplayBar(pivot, "L", cfg), Label: "L"})
			}
		}
	}
	return hits
}

func legacyTVMaxMinRSISeries(closes, rsx []float64, lookback int) (maxRSI, minRSI []float64) {
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

func TestRSTV_MatchesLegacyFullScan(t *testing.T) {
	t.Parallel()
	series := []struct {
		closes, rsx []float64
		opens       []int64
		lookback    int
	}{
		func() struct {
			closes, rsx []float64
			opens       []int64
			lookback    int
		} {
			c, r, o := rstvForcedSeries()
			return struct {
				closes, rsx []float64
				opens       []int64
				lookback    int
			}{c, r, o, 90}
		}(),
		func() struct {
			closes, rsx []float64
			opens       []int64
			lookback    int
		} {
			c, r, o := rstvPivotPeakSeries()
			return struct {
				closes, rsx []float64
				opens       []int64
				lookback    int
			}{c, r, o, 90}
		}(),
	}
	n := 200
	closes := make([]float64, n)
	rsx := make([]float64, n)
	opens := make([]int64, n)
	for i := 0; i < n; i++ {
		wave := float64(i%40) - 20
		closes[i] = 100 + wave*0.5
		rsx[i] = 50 + wave
		opens[i] = 1_700_000_000_000 + int64(i)*60_000
	}
	series = append(series, struct {
		closes, rsx []float64
		opens       []int64
		lookback    int
	}{closes, rsx, opens, 30})

	for si, s := range series {
		legacyHits := legacyScanRSXTVHits(s.closes, s.rsx, s.lookback)
		canonHits := scanRSXTVHits(s.closes, s.rsx, s.lookback)
		if len(legacyHits) != len(canonHits) {
			t.Fatalf("series %d div hits legacy=%d canon=%d", si, len(legacyHits), len(canonHits))
		}
		for i := range legacyHits {
			if legacyHits[i] != canonHits[i] {
				t.Fatalf("series %d hit %d legacy=%+v canon=%+v", si, i, legacyHits[i], canonHits[i])
			}
		}
		maxRSI, minRSI := legacyTVMaxMinRSISeries(s.closes, s.rsx, s.lookback)
		var legacyPivots []IndicatorFactEvent
		for i := 3; i < len(s.rsx); i++ {
			if maxRSI[i] == maxRSI[i-2] && maxRSI[i-2] != maxRSI[i-3] {
				legacyPivots = append(legacyPivots, IndicatorFactEvent{
					Source: FactSourceRSXTVPivot, Direction: FactDirPivotHigh,
					ConfirmedAt: s.opens[i], AnchorAt: s.opens[i-2],
					AnchorValue: s.rsx[i-2], AnchorPrice: s.closes[i-2],
				})
			}
			if minRSI[i] == minRSI[i-2] && minRSI[i-2] != minRSI[i-3] {
				legacyPivots = append(legacyPivots, IndicatorFactEvent{
					Source: FactSourceRSXTVPivot, Direction: FactDirPivotLow,
					ConfirmedAt: s.opens[i], AnchorAt: s.opens[i-2],
					AnchorValue: s.rsx[i-2], AnchorPrice: s.closes[i-2],
				})
			}
		}
		canonPivots := TVPivotFacts(s.closes, s.rsx, s.opens, s.lookback)
		if len(legacyPivots) != len(canonPivots) {
			t.Fatalf("series %d pivots legacy=%d canon=%d", si, len(legacyPivots), len(canonPivots))
		}
		for i := range legacyPivots {
			if legacyPivots[i] != canonPivots[i] {
				t.Fatalf("series %d pivot %d\nlegacy=%+v\ncanon=%+v", si, i, legacyPivots[i], canonPivots[i])
			}
		}
	}
}
