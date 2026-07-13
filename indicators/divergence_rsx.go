package indicators

import (
	"fmt"
	"strings"
)

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

// RSXMarkerHit is one RSX divergence marker before chart placement.
type RSXMarkerHit struct {
	PivotBar   int
	DisplayBar int
	Label      string
	PeakType   PeakType // PeakHigh / PeakLow for pivot "P" marker styling
}

// DivAnnotation is a pane-ready divergence marker (time filled by strategy layer).
type DivAnnotation struct {
	BarIndex int
	Label    string
	Color    string
	Position string
	Shape    string
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

// ScanRSXMarkers runs a full-series RSX divergence scan (stateless, reads from DataBus).
func ScanRSXMarkers(bus DataBus, cfg RSXScanConfig) []RSXMarkerHit {
	if bus == nil {
		return nil
	}
	cfg = NormalizeRSXScanConfig(cfg)
	prices := bus.RSXPriceSeries()
	closes := bus.CloseSeries()
	osc := bus.JurikSeries()
	switch cfg.Mode {
	case RSXScanFractal:
		return scanRSXFractalHits(prices, osc, cfg)
	default:
		return scanRSXTVHits(closes, osc, cfg.Lookback)
	}
}

// scanRSXMarkersFromSlices is a low-level helper for tests and batch adapters.
func scanRSXMarkersFromSlices(prices, closes, osc []float64, cfg RSXScanConfig) []RSXMarkerHit {
	cfg = NormalizeRSXScanConfig(cfg)
	switch cfg.Mode {
	case RSXScanFractal:
		return scanRSXFractalHits(prices, osc, cfg)
	default:
		return scanRSXTVHits(closes, osc, cfg.Lookback)
	}
}

// RSXHitAtDisplayBar returns the strongest marker hit visible on displayBar, if any.
// Streaming path: O(lookback) windowed scan — not a full-series ScanRSXMarkers.
func RSXHitAtDisplayBar(bus DataBus, displayBar int, cfg RSXScanConfig) RSXMarkerHit {
	if displayBar < 0 || bus == nil {
		return RSXMarkerHit{}
	}
	cfg = NormalizeRSXScanConfig(cfg)
	closes := bus.CloseSeries()
	osc := bus.JurikSeries()
	switch cfg.Mode {
	case RSXScanFractal:
		prices := bus.RSXPriceSeries()
		return rsxFractalHitAtDisplayBar(prices, osc, displayBar, cfg)
	default:
		return rsxTVHitAtDisplayBar(closes, osc, displayBar, cfg.Lookback)
	}
}

// RSXLabelAtDisplayBar returns the marker label visible on displayBar, if any.
func RSXLabelAtDisplayBar(bus DataBus, displayBar int, cfg RSXScanConfig) string {
	return RSXHitAtDisplayBar(bus, displayBar, cfg).Label
}

// RSXDivAnnotation builds a styled annotation for one RSX marker label.
func RSXDivAnnotation(displayBar int, label string) DivAnnotation {
	return RSXDivAnnotationFromHit(RSXMarkerHit{DisplayBar: displayBar, Label: label})
}

// RSXDivAnnotationFromHit builds a pane-ready annotation using pivot peak type for "P" markers.
func RSXDivAnnotationFromHit(hit RSXMarkerHit) DivAnnotation {
	displayBar := hit.DisplayBar
	label := hit.Label
	var color, position, shape string
	if label == "P" {
		color, position, shape = rsxPivotAnnotationStyle(hit.PeakType == PeakHigh)
	} else {
		color, position, shape = rsxAnnotationStyle(label)
	}
	return DivAnnotation{
		BarIndex: displayBar,
		Label:    label,
		Color:    color,
		Position: position,
		Shape:    shape,
	}
}

func rsxPivotAnnotationStyle(isHigh bool) (color, position, shape string) {
	if isHigh {
		return "#2962FF", "aboveBar", "arrowDown"
	}
	return "#2962FF", "belowBar", "arrowUp"
}

// AnalyzeWithRSX combines ZigZag macro/micro scoring with RSX marker detection at displayBar.
func (e *SmartDivergenceEngine) AnalyzeWithRSX(
	bus DataBus,
	displayBar int,
) (combined DivSignal, annotation *DivAnnotation) {
	macro := e.AnalyzeMacro()
	microScore := e.AnalyzeMicroCombined()
	combined = combineRSXDivSignals(macro, microScore)

	hit := RSXHitAtDisplayBar(bus, displayBar, e.rsxConfig)
	if hit.Label == "" {
		return combined, nil
	}
	ann := RSXDivAnnotationFromHit(hit)
	return combined, &ann
}

// ScanRSX runs a full-series RSX divergence scan using the engine configuration.
func (e *SmartDivergenceEngine) ScanRSX(bus DataBus) []RSXMarkerHit {
	if e == nil {
		return nil
	}
	return ScanRSXMarkers(bus, e.rsxConfig)
}

// RSXLabelAtDisplayBar returns the marker label visible on displayBar.
func (e *SmartDivergenceEngine) RSXLabelAtDisplayBar(bus DataBus, displayBar int) string {
	if e == nil {
		return ""
	}
	return RSXLabelAtDisplayBar(bus, displayBar, e.rsxConfig)
}

func joinDivDescriptions(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "; ")
}

func combineRSXDivSignals(macro DivSignal, microScore int) DivSignal {
	score := macro.Score + microScore
	if score > 100 {
		score = 100
	}
	if score < -100 {
		score = -100
	}
	desc := macro.Description
	if microScore != 0 {
		microPart := fmt.Sprintf("Micro (%+d)", microScore)
		if desc != "" {
			desc = strings.Join([]string{desc, microPart}, "; ")
		} else {
			desc = microPart
		}
	}
	return DivSignal{Score: score, Description: desc}
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

func rsxAnnotationStyle(label string) (color, position, shape string) {
	switch label {
	case "S", "SS":
		return "#ef5350", "aboveBar", "arrowDown"
	case "L", "LL":
		return "#26a69a", "belowBar", "arrowUp"
	default:
		return "#2962ff", "belowBar", "circle"
	}
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

func BearishRSXMarkerLabel(div DivergenceResult) string {
	return bearishRSXMarkerLabel(div)
}

func BullishRSXMarkerLabel(div DivergenceResult) string {
	return bullishRSXMarkerLabel(div)
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

func scanRSXFractalHits(prices, rsx []float64, cfg RSXScanConfig) []RSXMarkerHit {
	radius := cfg.PivotRadius
	if len(rsx) < radius*2+1 || len(prices) != len(rsx) {
		return nil
	}

	var hits []RSXMarkerHit
	lastPivotHigh := -1
	lastPivotLow := -1

	for i := radius; i+radius < len(rsx); i++ {
		switch {
		case isRSXPivotHigh(rsx, i, radius):
			marker := ""
			if lastPivotHigh >= 0 && i-lastPivotHigh <= cfg.Lookback {
				div := checkRSXPivotDivergence(prices, rsx, lastPivotHigh, i, PeakHigh, cfg)
				if div.Direction == Bearish {
					marker = bearishRSXMarkerLabel(div)
				}
			}
			if marker == "" && isRSXMacroPivotHigh(rsx, i, cfg.MacroPivotRadius) {
				marker = "P"
			}
			if marker != "" {
				hits = append(hits, RSXMarkerHit{
					PivotBar:   i,
					DisplayBar: rsxDisplayBar(i, marker, cfg),
					Label:      marker,
					PeakType:   PeakHigh,
				})
			}
			lastPivotHigh = i

		case isRSXPivotLow(rsx, i, radius):
			marker := ""
			if lastPivotLow >= 0 && i-lastPivotLow <= cfg.Lookback {
				div := checkRSXPivotDivergence(prices, rsx, lastPivotLow, i, PeakLow, cfg)
				if div.Direction == Bullish {
					marker = bullishRSXMarkerLabel(div)
				}
			}
			if marker == "" && isRSXMacroPivotLow(rsx, i, cfg.MacroPivotRadius) {
				marker = "P"
			}
			if marker != "" {
				hits = append(hits, RSXMarkerHit{
					PivotBar:   i,
					DisplayBar: rsxDisplayBar(i, marker, cfg),
					Label:      marker,
					PeakType:   PeakLow,
				})
			}
			lastPivotLow = i
		}
	}
	return hits
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

func bearishRSXMarkerLabel(div DivergenceResult) string {
	if div.Class == ClassA || div.Class == ClassC {
		return "SS"
	}
	return "S"
}

func bullishRSXMarkerLabel(div DivergenceResult) string {
	if div.Class == ClassA || div.Class == ClassC {
		return "LL"
	}
	return "L"
}

// rsxTVHitAtDisplayBar evaluates TV divergence markers for one display bar (O(lookback)).
func rsxTVHitAtDisplayBar(closes, rsx []float64, displayBar, lookback int) RSXMarkerHit {
	n := len(rsx)
	if displayBar < 2 || displayBar >= n || len(closes) != n {
		return RSXMarkerHit{}
	}
	if lookback <= 0 {
		lookback = DefaultRSXLookback
	}
	start := displayBar - 3*lookback
	if start < 0 {
		start = 0
	}

	var maxClose, maxRSX, minClose, minRSX float64
	var hasMax, hasMin bool
	var maxClosePrev, maxClosePrev2, minClosePrev, minClosePrev2 float64
	cfg := RSXScanConfig{Mode: RSXScanTV}

	for i := start; i <= displayBar; i++ {
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

		maxClosePrev2, maxClosePrev = maxClosePrev, maxClose
		minClosePrev2, minClosePrev = minClosePrev, minClose

		if i != displayBar || i < 2 {
			continue
		}

		best := RSXMarkerHit{}
		bestStrength := -1
		if maxClosePrev > maxClosePrev2 &&
			rsx[i-1] < maxRSX &&
			rsx[i] <= rsx[i-1] {
			pivot := i - 1
			hit := RSXMarkerHit{
				PivotBar:   pivot,
				DisplayBar: rsxDisplayBar(pivot, "S", cfg),
				Label:      "S",
			}
			if hit.DisplayBar == displayBar {
				st := rsxTradingMarkerStrength(hit.Label)
				if st > bestStrength {
					best = hit
					bestStrength = st
				}
			}
		}
		if minClosePrev < minClosePrev2 &&
			rsx[i-1] > minRSX &&
			rsx[i] >= rsx[i-1] {
			pivot := i - 1
			hit := RSXMarkerHit{
				PivotBar:   pivot,
				DisplayBar: rsxDisplayBar(pivot, "L", cfg),
				Label:      "L",
			}
			if hit.DisplayBar == displayBar {
				st := rsxTradingMarkerStrength(hit.Label)
				if st > bestStrength {
					best = hit
					bestStrength = st
				}
			}
		}
		return best
	}
	return RSXMarkerHit{}
}

// rsxFractalHitAtDisplayBar finds fractal-mode markers visible on displayBar (O(lookback)).
func rsxFractalHitAtDisplayBar(prices, rsx []float64, displayBar int, cfg RSXScanConfig) RSXMarkerHit {
	radius := cfg.PivotRadius
	if radius <= 0 {
		radius = DefaultRSXPivotRadius
	}
	n := len(rsx)
	if n < radius*2+1 || len(prices) != n || displayBar < 0 {
		return RSXMarkerHit{}
	}
	start := displayBar - cfg.Lookback - cfg.MacroPivotRadius
	if start < radius {
		start = radius
	}
	end := displayBar + radius
	if end > n-radius-1 {
		end = n - radius - 1
	}

	lastPivotHigh := -1
	lastPivotLow := -1
	for i := radius; i < start; i++ {
		if isRSXPivotHigh(rsx, i, radius) {
			lastPivotHigh = i
		}
		if isRSXPivotLow(rsx, i, radius) {
			lastPivotLow = i
		}
	}

	best := RSXMarkerHit{}
	bestStrength := -1
	for i := start; i <= end; i++ {
		switch {
		case isRSXPivotHigh(rsx, i, radius):
			marker := ""
			if lastPivotHigh >= 0 && i-lastPivotHigh <= cfg.Lookback {
				div := checkRSXPivotDivergence(prices, rsx, lastPivotHigh, i, PeakHigh, cfg)
				if div.Direction == Bearish {
					marker = bearishRSXMarkerLabel(div)
				}
			}
			if marker == "" && isRSXMacroPivotHigh(rsx, i, cfg.MacroPivotRadius) {
				marker = "P"
			}
			if marker != "" {
				hit := RSXMarkerHit{
					PivotBar:   i,
					DisplayBar: rsxDisplayBar(i, marker, cfg),
					Label:      marker,
					PeakType:   PeakHigh,
				}
				if hit.DisplayBar == displayBar {
					st := rsxTradingMarkerStrength(hit.Label)
					if st > bestStrength {
						best = hit
						bestStrength = st
					}
				}
			}
			lastPivotHigh = i

		case isRSXPivotLow(rsx, i, radius):
			marker := ""
			if lastPivotLow >= 0 && i-lastPivotLow <= cfg.Lookback {
				div := checkRSXPivotDivergence(prices, rsx, lastPivotLow, i, PeakLow, cfg)
				if div.Direction == Bullish {
					marker = bullishRSXMarkerLabel(div)
				}
			}
			if marker == "" && isRSXMacroPivotLow(rsx, i, cfg.MacroPivotRadius) {
				marker = "P"
			}
			if marker != "" {
				hit := RSXMarkerHit{
					PivotBar:   i,
					DisplayBar: rsxDisplayBar(i, marker, cfg),
					Label:      marker,
					PeakType:   PeakLow,
				}
				if hit.DisplayBar == displayBar {
					st := rsxTradingMarkerStrength(hit.Label)
					if st > bestStrength {
						best = hit
						bestStrength = st
					}
				}
			}
			lastPivotLow = i
		}
	}
	return best
}

func scanRSXTVHits(closes, rsx []float64, lookback int) []RSXMarkerHit {
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
			if maxCloseHist[i-1] > maxCloseHist[i-2] &&
				rsx[i-1] < maxRSX &&
				rsx[i] <= rsx[i-1] {
				pivot := i - 1
				hits = append(hits, RSXMarkerHit{
					PivotBar:   pivot,
					DisplayBar: rsxDisplayBar(pivot, "S", cfg),
					Label:      "S",
				})
			}
			if minCloseHist[i-1] < minCloseHist[i-2] &&
				rsx[i-1] > minRSX &&
				rsx[i] >= rsx[i-1] {
				pivot := i - 1
				hits = append(hits, RSXMarkerHit{
					PivotBar:   pivot,
					DisplayBar: rsxDisplayBar(pivot, "L", cfg),
					Label:      "L",
				})
			}
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
