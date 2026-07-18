package wire

import (
	"math"

	"trading_bot/core"
)

// Annotation is one Lightweight Charts marker ready for the client (no indicator math).
// JSON shape matches the historical ChartAnnotation wire contract.
type Annotation struct {
	Time     int64  `json:"time"`
	Pane     string `json:"pane"`  // "price", "rsx", "wozduh"
	Label    string `json:"label"` // "L", "S", "SS", "LL", …
	Color    string `json:"color"`
	Position string `json:"position"` // "aboveBar", "belowBar", "inBar"
	Shape    string `json:"shape"`    // "arrowUp", "arrowDown", "circle"
}

// DivStateLabel maps SlotDivState enum values to RSX marker labels.
func DivStateLabel(state float64) string {
	switch state {
	case core.DivStateS:
		return "S"
	case core.DivStateSS:
		return "SS"
	case core.DivStateL:
		return "L"
	case core.DivStateLL:
		return "LL"
	default:
		return ""
	}
}

// AnnotationStyleFromLabel returns LWC visual props for a divergence label.
// Projector owns color/shape; DAG never does.
func AnnotationStyleFromLabel(label string) (color, position, shape string) {
	switch label {
	case "S", "SS":
		return "#ef5350", "aboveBar", "arrowDown"
	case "L", "LL":
		return "#26a69a", "belowBar", "arrowUp"
	default:
		return "#2962ff", "belowBar", "circle"
	}
}

// AnnotationFromDivState is a Phase F no-op: SlotDivState math remains in the DAG,
// but L/LL/S/SS labels are not published to the chart wire until a new strategy surface exists.
func AnnotationFromDivState(timeSec int64, state float64, pane string) (Annotation, bool) {
	_ = timeSec
	_ = state
	_ = pane
	return Annotation{}, false
}

func divStateActive(state float64) bool {
	if math.IsNaN(state) || math.IsInf(state, 0) {
		return false
	}
	return state != core.DivStateNone
}
