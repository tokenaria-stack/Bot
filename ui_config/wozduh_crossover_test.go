package ui_config

import (
	"testing"

	"trading_bot/core/nodes"
)

func TestWozduhCrossoverFactory_MatchesPairs(t *testing.T) {
	pairs := nodes.WozduhCrossoverPairs()
	factory := WozduhCrossoverFactory()
	if len(factory) != len(pairs) {
		t.Fatalf("factory %d pairs %d", len(factory), len(pairs))
	}
	shapes := map[string]string{
		pairs[0].ID: WozduhCrossoverShapeCircle,
		pairs[1].ID: WozduhCrossoverShapeSquare,
		pairs[2].ID: WozduhCrossoverShapeTriangle,
		pairs[3].ID: WozduhCrossoverShapeStar4,
	}
	if !factory[0].Visible {
		t.Fatal("pair 1 must default visible")
	}
	for i, row := range factory {
		if row.ID != pairs[i].ID || row.PlotA != pairs[i].PlotA || row.PlotB != pairs[i].PlotB {
			t.Fatalf("row %d %+v vs %+v", i, row, pairs[i])
		}
		if row.Shape != shapes[row.ID] {
			t.Fatalf("%s shape %s", row.ID, row.Shape)
		}
		if i > 0 && row.Visible {
			t.Fatalf("%s should default off", row.ID)
		}
	}
}

func TestWozduhCrossoverFactory_NotDdrComponents(t *testing.T) {
	for _, c := range WozduhComponents() {
		if c.Kind == "crossover" || c.DataMode == "events" {
			t.Fatalf("do not register crossover as DDR: %+v", c)
		}
		if c.ID == nodes.WozduhXoverVolEma12XEma5 || c.ID == "woz_vol_cross" {
			t.Fatalf("volcross/crossover id leaked into DDR: %s", c.ID)
		}
	}
}
