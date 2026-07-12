package ui_config

import (
	"encoding/json"

	"trading_bot/core"
)

// WozduhComponents returns DDR bindings for Wozduh volume RSI fast/slow lines.
func WozduhComponents() []core.UIComponent {
	return []core.UIComponent{
		{
			ID:         "woz_fast",
			Pane:       "pane_osc",
			HostID:     "wozduh",
			Kind:       "line",
			DataMode:   "scalar",
			Slot:       core.SlotWozduhFast,
			RenderOpts: json.RawMessage(`{"color":"blue","lineWidth":2,"title":"wt11 (Blue)"}`),
		},
		{
			ID:         "woz_slow",
			Pane:       "pane_osc",
			HostID:     "wozduh",
			Kind:       "line",
			DataMode:   "scalar",
			Slot:       core.SlotWozduhSlow,
			RenderOpts: json.RawMessage(`{"color":"aqua","lineWidth":2,"title":"wt22 (Aqua)"}`),
		},
	}
}
