package ui_config

import "trading_bot/core"

// ScoreComponents is intentionally empty until Score overlay phase remounts
// these series via lazy DDR (not visible:false zombies in LWC).
func ScoreComponents() []core.UIComponent {
	return []core.UIComponent{}
}
