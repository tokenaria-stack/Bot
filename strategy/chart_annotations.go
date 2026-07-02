package strategy

import "strings"

// ChartAnnotation is one native Lightweight Charts marker bound to a dashboard pane.
type ChartAnnotation struct {
	Time     int64  `json:"time"`
	Pane     string `json:"pane"`     // "price", "rsx", "wozduh"
	Label    string `json:"label"`    // "L", "S", "SS", "LL", "P", …
	Color    string `json:"color"`
	Position string `json:"position"` // "aboveBar", "belowBar", "inBar"
	Shape    string `json:"shape"`    // "arrowUp", "arrowDown", "circle"
}

func normalizeAnnotationPane(pane string) string {
	switch strings.ToLower(strings.TrimSpace(pane)) {
	case "price", "wozduh":
		return strings.ToLower(strings.TrimSpace(pane))
	default:
		return "rsx"
	}
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

func rsxMarkerChartStrength(marker string) int {
	if st, ok := rsxTradingMarkerStrength[marker]; ok {
		return st
	}
	if marker == "P" {
		return 0
	}
	return -1
}

// ChartMarkerAt returns the honest RSX label for barIndex (confirmation / display bar).
func (a *Marker) ChartMarkerAt(barIndex int) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if barIndex == a.cachedRSXMarkerBar && a.cachedRSXMarkerLabel != "" {
		return a.cachedRSXMarkerLabel
	}
	if a.divEngine == nil {
		return ""
	}
	return a.divEngine.RSXLabelAtDisplayBar(a, barIndex)
}

// ExportAllAnnotations collects chart markers from all indicator subsystems for [fromBar..toBar].
func (a *Marker) ExportAllAnnotations(fromBar, toBar int) []ChartAnnotation {
	a.mu.RLock()
	defer a.mu.RUnlock()

	n := len(a.klines)
	if n == 0 {
		return nil
	}
	if fromBar < 0 {
		fromBar = 0
	}
	if toBar < 0 || toBar >= n {
		toBar = n - 1
	}
	if fromBar > toBar {
		return nil
	}

	out := make([]ChartAnnotation, 0, 32)
	out = append(out, a.exportRSXAnnotationsLocked(fromBar, toBar)...)
	out = append(out, a.exportWozduhAnnotationsLocked(fromBar, toBar)...)
	return out
}

func (a *Marker) exportRSXAnnotationsLocked(fromBar, toBar int) []ChartAnnotation {
	out := make([]ChartAnnotation, 0, len(a.Annotations))
	for _, ann := range a.Annotations {
		if ann.Pane != normalizeAnnotationPane("rsx") {
			continue
		}
		barIdx := a.barIndexFromTimeSec(ann.Time)
		if barIdx < fromBar || barIdx > toBar {
			continue
		}
		out = append(out, ann)
	}
	return out
}

// exportWozduhAnnotationsLocked is a stub for future VolCross / spike markers on the wozduh pane.
func (a *Marker) exportWozduhAnnotationsLocked(fromBar, toBar int) []ChartAnnotation {
	_ = fromBar
	_ = toBar
	return nil
}
