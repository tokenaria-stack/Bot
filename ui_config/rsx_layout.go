package ui_config

import (
	"encoding/json"

	"trading_bot/core"
)

// RSXComponents returns DDR bindings for Jurik RSX and its signal line.
// ADR-022: scaleContribution is per-component (bounded anchor vs ignore peers).
func RSXComponents() []core.UIComponent {
	return []core.UIComponent{
		{
			ID:         "line_rsx",
			Pane:       "pane_osc",
			HostID:     "rsx",
			Kind:       "line",
			DataMode:   "scalar",
			Slot:       core.SlotJurikRSX,
			RenderOpts: json.RawMessage(`{"color":"#E1D2B5","lineWidth":2,"title":"RSX","scaleContribution":{"type":"bounded","min":-5,"max":105}}`),
		},
		{
			ID:         "line_rsx_signal",
			Pane:       "pane_osc",
			HostID:     "rsx",
			Kind:       "line",
			DataMode:   "scalar",
			Slot:       core.SlotJurikSignal,
			RenderOpts: json.RawMessage(`{"color":"#8B9BB4","lineWidth":1,"title":"RSX Signal","scaleContribution":{"type":"ignore"}}`),
		},
		{
			// Shot 9I: Projector packs SlotDivState → LWC markers; DAG never knows colors/shapes.
			ID:         "ann_rsx_div",
			Pane:       "pane_osc",
			HostID:     "rsx",
			Kind:       "marker",
			DataMode:   "annotations",
			Slot:       core.SlotDivState,
			RenderOpts: json.RawMessage(`{"title":"RSX Div"}`),
		},
	}
}
