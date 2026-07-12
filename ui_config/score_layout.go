package ui_config

import (
	"encoding/json"

	"trading_bot/core"
)

// ScoreComponents returns DDR bindings for DAG divergence and total score slots.
func ScoreComponents() []core.UIComponent {
	return []core.UIComponent{
		{
			ID:         "score_div_macro",
			Pane:       "pane_score",
			HostID:     "wozduh",
			Kind:       "histogram",
			DataMode:   "scalar",
			Slot:       core.SlotDivScore,
			RenderOpts: json.RawMessage(`{"color":"#089981","title":"Macro Div"}`),
		},
		{
			ID:         "score_div_micro",
			Pane:       "pane_score",
			HostID:     "wozduh",
			Kind:       "histogram",
			DataMode:   "scalar",
			Slot:       core.SlotMicroDivScore,
			RenderOpts: json.RawMessage(`{"color":"#f7931a","title":"Micro Div"}`),
		},
		{
			ID:         "score_total",
			Pane:       "pane_score",
			HostID:     "wozduh",
			Kind:       "line",
			DataMode:   "scalar",
			Slot:       core.SlotTotalScore,
			RenderOpts: json.RawMessage(`{"color":"#2962ff","lineWidth":2,"title":"Total Score"}`),
		},
	}
}
