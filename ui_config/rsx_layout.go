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
			RenderOpts: json.RawMessage(`{"color":"#512DA8","lineWidth":2,"title":"RSX","lastValueVisible":false,"priceLineVisible":false,"scaleContribution":{"type":"bounded","min":-5,"max":105}}`),
		},
		{
			ID:         "line_rsx_signal",
			Pane:       "pane_osc",
			HostID:     "rsx",
			Kind:       "line",
			DataMode:   "scalar",
			Slot:       core.SlotJurikSignal,
			RenderOpts: json.RawMessage(`{"color":"#8B9BB4","lineWidth":1,"title":"RSX Signal","lastValueVisible":false,"priceLineVisible":false,"scaleContribution":{"type":"ignore"}}`),
		},
		{
			// RSX pane marker host. Slot names the pane's scalar owner, not a divergence value.
			ID:         "ann_rsx_div",
			Pane:       "pane_osc",
			HostID:     "rsx",
			Kind:       "marker",
			DataMode:   "annotations",
			Slot:       core.SlotJurikRSX,
			RenderOpts: json.RawMessage(`{"title":"RSX Div"}`),
		},
	}
}
