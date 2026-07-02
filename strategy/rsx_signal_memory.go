package strategy

// rsxTradingMarkerStrength ranks actionable RSX divergence labels.
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

// RSXTradingMarkerAtBar returns the actionable RSX marker on barIndex, or "" if none.
func RSXTradingMarkerAtBar(points []RSXPoint, barIndex int) string {
	if barIndex < 0 || barIndex >= len(points) {
		return ""
	}
	marker := points[barIndex].Marker
	if IsRSXTradingMarker(marker) {
		return marker
	}
	return ""
}
