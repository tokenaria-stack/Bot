package ui_config

import (
	"encoding/json"

	"trading_bot/core"
)

// RSXComponents returns DDR bindings for Jurik RSX and its signal line.
func RSXComponents() []core.UIComponent {
	return []core.UIComponent{
		{
			ID:         "line_rsx",
			Pane:       "pane_osc",
			HostID:     "rsx",
			Kind:       "line",
			DataMode:   "scalar",
			Slot:       core.SlotJurikRSX,
			RenderOpts: json.RawMessage(`{"color":"#E1D2B5","lineWidth":2,"title":"RSX"}`),
		},
		{
			ID:         "line_rsx_signal",
			Pane:       "pane_osc",
			HostID:     "rsx",
			Kind:       "line",
			DataMode:   "scalar",
			Slot:       core.SlotJurikSignal,
			RenderOpts: json.RawMessage(`{"color":"#8B9BB4","lineWidth":1,"title":"RSX Signal"}`),
		},
	}
}
