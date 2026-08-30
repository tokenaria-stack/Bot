package wire

import (
	"math"

	"trading_bot/core"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

// Annotation is one Lightweight Charts marker ready for the client (no indicator math).
// JSON shape matches the historical ChartAnnotation wire contract.
type Annotation struct {
	Time     int64  `json:"time"`
	Pane     string `json:"pane"` // "price", "rsx", "wozduh"
	Label    string `json:"label"`
	Color    string `json:"color"`
	Position string `json:"position"` // "aboveBar", "belowBar", "inBar"
	Shape    string `json:"shape"`    // "arrowUp", "arrowDown", "circle"
	Source   string `json:"source,omitempty"`
}

const (
	rsxDivBullColor = "#00e676"
	rsxDivBearColor = "#ff1744"
	rsxPivotColor   = "#2979ff"
)

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

// AnnotationFromFact projects a factual event into a pane marker.
// Time is AnchorAt (visual). Knowledge time stays on the event (ConfirmedAt).
func AnnotationFromFact(ev indicators.IndicatorFactEvent, pane string) (Annotation, bool) {
	if ev.AnchorAt <= 0 {
		return Annotation{}, false
	}
	color, position, shape, ok := factPresentation(ev.Source, ev.Direction)
	if !ok {
		return Annotation{}, false
	}
	if pane == "" {
		pane = "rsx"
	}
	return Annotation{
		Time:     exchange.ChartTimeSec(ev.AnchorAt),
		Pane:     pane,
		Label:    "",
		Color:    color,
		Position: position,
		Shape:    shape,
		Source:   ev.Source,
	}, true
}

func factPresentation(source, direction string) (color, position, shape string, ok bool) {
	switch source {
	case indicators.FactSourceRSXTVDiv:
		switch direction {
		case indicators.FactDirBullish:
			return rsxDivBullColor, "belowBar", "arrowUp", true
		case indicators.FactDirBearish:
			return rsxDivBearColor, "aboveBar", "arrowDown", true
		}
	case indicators.FactSourceRSXTVPivot:
		switch direction {
		case indicators.FactDirPivotHigh:
			return rsxPivotColor, "aboveBar", "arrowDown", true
		case indicators.FactDirPivotLow:
			return rsxPivotColor, "belowBar", "arrowUp", true
		}
	}
	return "", "", "", false
}

// AnnotationFromDivState does not publish ZigZag DivState as RSX TV facts.
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
