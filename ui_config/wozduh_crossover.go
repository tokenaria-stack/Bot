package ui_config

import "trading_bot/core/nodes"

// WozduhCrossoverShape is the closed paint-shape enum for crossover dots.
const (
	WozduhCrossoverShapeCircle   = "circle"
	WozduhCrossoverShapeSquare   = "square"
	WozduhCrossoverShapeTriangle = "triangle"
	WozduhCrossoverShapeStar4    = "star4"
)

// WozduhCrossoverPaint is factory chrome for one pair. Sparse user prefs override these.
type WozduhCrossoverPaint struct {
	ID           string
	Title        string
	PlotA        string
	PlotB        string
	Visible      bool
	Shape        string
	Size         float64
	UpFill       string
	DownFill     string
	OutlineColor string
	OutlineWidth float64
	OutlineStyle string
}

// WozduhCrossoverFactory is the canonical factory for WOZDUH-CROSSOVER-PAINT-1.
// Pair 1 matches the historical Pine cross(wt11, wt22) default-on microscope.
// Pairs 2–4 default off so demand does not wake extra streams until enabled.
func WozduhCrossoverFactory() []WozduhCrossoverPaint {
	pairs := nodes.WozduhCrossoverPairs()
	return []WozduhCrossoverPaint{
		{
			ID: pairs[0].ID, Title: "Volume RSI EMA12 × EMA5",
			PlotA: pairs[0].PlotA, PlotB: pairs[0].PlotB,
			Visible: true, Shape: WozduhCrossoverShapeCircle, Size: 8,
			UpFill: "#00E676", DownFill: "#FF1744",
			OutlineColor: "#000000", OutlineWidth: 1, OutlineStyle: "solid",
		},
		{
			ID: pairs[1].ID, Title: "RSI VWEMA(HL2) × Volume RSI EMA5",
			PlotA: pairs[1].PlotA, PlotB: pairs[1].PlotB,
			Visible: false, Shape: WozduhCrossoverShapeSquare, Size: 8,
			UpFill: "#00E676", DownFill: "#FF1744",
			OutlineColor: "#000000", OutlineWidth: 1, OutlineStyle: "solid",
		},
		{
			ID: pairs[2].ID, Title: "RSI VWEMA(HL2) × Volume RSI EMA12",
			PlotA: pairs[2].PlotA, PlotB: pairs[2].PlotB,
			Visible: false, Shape: WozduhCrossoverShapeTriangle, Size: 8,
			UpFill: "#00E676", DownFill: "#FF1744",
			OutlineColor: "#000000", OutlineWidth: 1, OutlineStyle: "solid",
		},
		{
			ID: pairs[3].ID, Title: "RSI VWEMA(HL2) × EMA5 channel mid",
			PlotA: pairs[3].PlotA, PlotB: pairs[3].PlotB,
			Visible: false, Shape: WozduhCrossoverShapeStar4, Size: 8,
			UpFill: "#00E676", DownFill: "#FF1744",
			OutlineColor: "#000000", OutlineWidth: 1, OutlineStyle: "solid",
		},
	}
}
