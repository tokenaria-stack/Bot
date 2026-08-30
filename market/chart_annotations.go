package market

import "strings"

// ChartAnnotation is one native Lightweight Charts marker bound to a dashboard pane.
type ChartAnnotation struct {
	Time     int64  `json:"time"`
	Pane     string `json:"pane"` // "price", "rsx", "wozduh"
	Label    string `json:"label"`
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

// ExportAllAnnotations collects chart markers from indicator subsystems for [fromBar..toBar].
func (a *Frame) ExportAllAnnotations(fromBar, toBar int) []ChartAnnotation {
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

	out := make([]ChartAnnotation, 0, 8)
	out = append(out, a.exportWozduhAnnotationsLocked(fromBar, toBar)...)
	return out
}

// exportWozduhAnnotationsLocked is a stub for future VolCross / spike markers on the wozduh pane.
func (a *Frame) exportWozduhAnnotationsLocked(fromBar, toBar int) []ChartAnnotation {
	_ = fromBar
	_ = toBar
	return nil
}
