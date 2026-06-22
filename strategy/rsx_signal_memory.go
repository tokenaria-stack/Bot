package strategy

import "trading_bot/exchange"

// RSXSignalMemoryBars is how many recent bars to scan for confirmed RSX trading markers
// after the 2-bar pivot confirmation lag (rsxPivotRadius).
const RSXSignalMemoryBars = 3

var rsxTradingMarkerStrength = map[string]int{
	"L":  1,
	"LL": 2,
	"S":  1,
	"SS": 2,
}

// IsRSXTradingMarker reports whether marker is an actionable RSX entry signal (not P).
func IsRSXTradingMarker(marker string) bool {
	_, ok := rsxTradingMarkerStrength[marker]
	return ok
}

// IsStrongRSXReversalMarker reports powerful divergence markers that may bypass macro filter.
func IsStrongRSXReversalMarker(marker string) bool {
	return marker == "LL" || marker == "SS"
}

// RecentRSXTradingMarker returns the strongest recent L/LL/S/SS marker within memoryBars
// ending at the latest bar (inclusive). Prefers LL/SS over L/S, then the most recent bar.
func RecentRSXTradingMarker(points []RSXPoint, memoryBars int) string {
	if len(points) == 0 {
		return ""
	}
	if memoryBars <= 0 {
		memoryBars = RSXSignalMemoryBars
	}
	from := len(points) - memoryBars
	if from < 0 {
		from = 0
	}
	return bestRSXTradingMarker(points, from, len(points)-1)
}

// RecentRSXTradingMarkerFromSeries batch-computes RSX chart points and scans recent bars.
func RecentRSXTradingMarkerFromSeries(klines []exchange.Kline, rsxValues []float64, divLookback, memoryBars int) string {
	points := BuildRSXChart(klines, rsxValues, divLookback)
	return RecentRSXTradingMarker(points, memoryBars)
}

func bestRSXTradingMarker(points []RSXPoint, from, to int) string {
	best := ""
	bestStrength := 0
	for i := to; i >= from; i-- {
		m := points[i].Marker
		strength, ok := rsxTradingMarkerStrength[m]
		if !ok {
			continue
		}
		if strength > bestStrength {
			best = m
			bestStrength = strength
		}
	}
	return best
}
