package indicators

const (
	DefaultRSXMacroPivotRadius   = 7
	DefaultRSXPeakIndexTolerance = 2
	DefaultRSXPivotRadius        = 2
	DefaultRSXLookback           = 90
)

// RSXScanMode selects fractal-pivot or TradingView rolling divergence.
type RSXScanMode int

const (
	RSXScanFractal RSXScanMode = iota
	RSXScanTV
)

// RSXScanConfig holds parameters for stateless RSX divergence scans.
type RSXScanConfig struct {
	Mode               RSXScanMode
	Lookback           int
	PivotRadius        int
	MacroPivotRadius   int
	PeakIndexTolerance int
	MinPriceDeltaRatio float64
	MinOscDelta        float64
}

// RSXMarkerHit is one RSX detector hit before fact/chart placement.
// Label is TV-family only (L/S). Fractal hits use Class / DivDir / IsPivot.
type RSXMarkerHit struct {
	PivotBar   int
	DisplayBar int
	Label      string
	PeakType   PeakType
	Class      DivClass
	DivDir     DivDirection
	IsPivot    bool
}

var rsxTradingMarkerStrengthMap = map[string]int{
	"L":  1,
	"LL": 2,
	"S":  1,
	"SS": 2,
}

// NormalizeRSXScanConfig clamps scan settings to safe defaults.
func NormalizeRSXScanConfig(cfg RSXScanConfig) RSXScanConfig {
	if cfg.Lookback <= 0 {
		cfg.Lookback = DefaultRSXLookback
	}
	if cfg.Mode == RSXScanFractal && cfg.PivotRadius <= 0 {
		cfg.PivotRadius = DefaultRSXPivotRadius
	}
	if cfg.MacroPivotRadius <= 0 {
		cfg.MacroPivotRadius = DefaultRSXMacroPivotRadius
	}
	if cfg.PeakIndexTolerance <= 0 {
		cfg.PeakIndexTolerance = DefaultRSXPeakIndexTolerance
	}
	return cfg
}

func rsxTradingMarkerStrength(label string) int {
	if st, ok := rsxTradingMarkerStrengthMap[label]; ok {
		return st
	}
	if label == "P" {
		return 0
	}
	return -1
}

func rsxDisplayBar(pivotBar int, label string, cfg RSXScanConfig) int {
	if cfg.Mode == RSXScanFractal {
		if label == "P" {
			return pivotBar + cfg.MacroPivotRadius
		}
		return pivotBar + cfg.PivotRadius
	}
	return pivotBar + 1
}

func IsRSXPivotHigh(rsx []float64, i, radius int) bool {
	return isRSXPivotHigh(rsx, i, radius)
}

func IsRSXPivotLow(rsx []float64, i, radius int) bool {
	return isRSXPivotLow(rsx, i, radius)
}

func isRSXPivotHigh(rsx []float64, i, radius int) bool {
	if radius <= 0 {
		radius = DefaultRSXPivotRadius
	}
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

func isRSXPivotLow(rsx []float64, i, radius int) bool {
	if radius <= 0 {
		radius = DefaultRSXPivotRadius
	}
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

func isRSXMacroPivotHigh(rsx []float64, i, macroRadius int) bool {
	if i < macroRadius || i+macroRadius >= len(rsx) {
		return false
	}
	v := rsx[i]
	for j := i - macroRadius; j <= i+macroRadius; j++ {
		if j != i && rsx[j] >= v {
			return false
		}
	}
	return true
}

func isRSXMacroPivotLow(rsx []float64, i, macroRadius int) bool {
	if i < macroRadius || i+macroRadius >= len(rsx) {
		return false
	}
	v := rsx[i]
	for j := i - macroRadius; j <= i+macroRadius; j++ {
		if j != i && rsx[j] <= v {
			return false
		}
	}
	return true
}

func checkRSXPivotDivergence(prices, rsx []float64, idx1, idx2 int, peakType PeakType, cfg RSXScanConfig) DivergenceResult {
	pricePeaks := []Peak{
		{Index: idx1, Value: prices[idx1], Type: peakType},
		{Index: idx2, Value: prices[idx2], Type: peakType},
	}
	oscPeaks := []Peak{
		{Index: idx1, Value: rsx[idx1], Type: peakType},
		{Index: idx2, Value: rsx[idx2], Type: peakType},
	}
	return CheckClassicDivergence(
		pricePeaks, oscPeaks, cfg.PeakIndexTolerance,
		cfg.MinPriceDeltaRatio, cfg.MinOscDelta,
	)
}

func fractalHitAtPivot(prices, rsx []float64, lastPivot, i int, peakType PeakType, cfg RSXScanConfig) RSXMarkerHit {
	empty := RSXMarkerHit{PivotBar: -1}
	wantDir := Bearish
	if peakType == PeakLow {
		wantDir = Bullish
	}
	if lastPivot >= 0 && i-lastPivot <= cfg.Lookback {
		div := checkRSXPivotDivergence(prices, rsx, lastPivot, i, peakType, cfg)
		if div.Direction == wantDir && div.Class != None {
			return RSXMarkerHit{
				PivotBar:   i,
				DisplayBar: rsxFractalConfirmBar(i, false, cfg),
				PeakType:   peakType,
				Class:      div.Class,
				DivDir:     div.Direction,
			}
		}
	}
	macro := isRSXMacroPivotHigh(rsx, i, cfg.MacroPivotRadius)
	if peakType == PeakLow {
		macro = isRSXMacroPivotLow(rsx, i, cfg.MacroPivotRadius)
	}
	if !macro {
		return empty
	}
	return RSXMarkerHit{
		PivotBar:   i,
		DisplayBar: rsxFractalConfirmBar(i, true, cfg),
		PeakType:   peakType,
		IsPivot:    true,
	}
}

func rsxFractalConfirmBar(pivotBar int, isPivot bool, cfg RSXScanConfig) int {
	if isPivot {
		return pivotBar + cfg.MacroPivotRadius
	}
	return pivotBar + cfg.PivotRadius
}

// previousRSXFractalPivot returns the nearest earlier same-type fractal pivot
// within lookback, or -1. Used by FractalFactsAt so live work stays O(lookback).
func previousRSXFractalPivot(rsx []float64, pivot, radius, lookback int, high bool) int {
	if pivot <= radius {
		return -1
	}
	lo := radius
	if lookback > 0 && pivot-lookback > lo {
		lo = pivot - lookback
	}
	for k := pivot - 1; k >= lo; k-- {
		if high {
			if isRSXPivotHigh(rsx, k, radius) {
				return k
			}
			continue
		}
		if isRSXPivotLow(rsx, k, radius) {
			return k
		}
	}
	return -1
}

func scanRSXTVHits(closes, rsx []float64, lookback int) []RSXMarkerHit {
	n := len(rsx)
	if n < 3 || len(closes) != n {
		return nil
	}
	st := NewRSTVState(lookback)
	var hits []RSXMarkerHit
	cfg := RSXScanConfig{Mode: RSXScanTV}
	for i := 0; i < n; i++ {
		evs, err := st.UpdateClosed(int64(i)+1, closes[i], rsx[i])
		if err != nil {
			continue
		}
		for j := 0; j < int(evs.Count); j++ {
			e := evs.Events[j]
			if e.Family != RSTVFamilyDiv {
				continue
			}
			label := "S"
			if e.Direction == FactDirBullish {
				label = "L"
			}
			pivot := i - 1
			hits = append(hits, RSXMarkerHit{
				PivotBar:   pivot,
				DisplayBar: rsxDisplayBar(pivot, label, cfg),
				Label:      label,
			})
		}
	}
	return hits
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
