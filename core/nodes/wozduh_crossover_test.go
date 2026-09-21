package nodes_test

import (
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/server/wire"
)

func TestWozduhCrossoverPairs_FourApproved(t *testing.T) {
	t.Parallel()
	pairs := nodes.WozduhCrossoverPairs()
	if len(pairs) != 4 {
		t.Fatalf("pairs=%d want 4", len(pairs))
	}
	want := []struct {
		id, a, b string
	}{
		{nodes.WozduhXoverVolEma12XEma5, "woz_vol_rsi_ema12", "woz_vol_rsi_ema5"},
		{nodes.WozduhXoverVwemaXEma5, "woz_rsi_hl2_vwema", "woz_vol_rsi_ema5"},
		{nodes.WozduhXoverVwemaXEma12, "woz_rsi_hl2_vwema", "woz_vol_rsi_ema12"},
		{nodes.WozduhXoverVwemaXEma5ChanMid, "woz_rsi_hl2_vwema", "woz_vol_rsi_ema5_chan_mid"},
	}
	for i, w := range want {
		if pairs[i].ID != w.id || pairs[i].PlotA != w.a || pairs[i].PlotB != w.b {
			t.Fatalf("pair %d = %+v want %s %s×%s", i, pairs[i], w.id, w.a, w.b)
		}
	}
}

func TestDetectCrossoverEdge_UpDownEquality(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		prevA, prevB, a, b float64
		want               string
	}{
		{"up from below", 10, 11, 12, 11, nodes.WozduhXoverSideUp},
		{"up from equal", 11, 11, 12, 11, nodes.WozduhXoverSideUp},
		{"no up still equal", 11, 11, 11, 11, ""},
		{"no up still below", 10, 11, 11, 11, ""},
		{"down from above", 12, 11, 10, 11, nodes.WozduhXoverSideDown},
		{"down from equal", 11, 11, 10, 11, nodes.WozduhXoverSideDown},
		{"no down still equal", 11, 11, 11, 11, ""},
		{"no down still above", 12, 11, 11, 11, ""},
		{"nan prev", math.NaN(), 11, 12, 11, ""},
		{"nan curr", 10, 11, math.NaN(), 11, ""},
		{"inf", 10, 11, math.Inf(1), 11, ""},
	}
	for _, tc := range cases {
		got := nodes.DetectCrossoverEdge(tc.prevA, tc.prevB, tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestScanWozduhCrossovers_YIsB_ClosedColumns(t *testing.T) {
	t.Parallel()
	absent := wire.HistoryAbsent
	plots := map[string][]float64{
		"woz_vol_rsi_ema12": {10, 12, 9},
		"woz_vol_rsi_ema5":  {12, 11, 10},
	}
	times := []int64{100, 200, 300}
	ev := nodes.ScanWozduhCrossovers(plots, times, absent)
	if len(ev) != 2 {
		t.Fatalf("events=%d want 2: %+v", len(ev), ev)
	}
	if ev[0].Pair != nodes.WozduhXoverVolEma12XEma5 || ev[0].Side != nodes.WozduhXoverSideUp || ev[0].Time != 200 || ev[0].Y != 11 {
		t.Fatalf("first %+v", ev[0])
	}
	if ev[1].Side != nodes.WozduhXoverSideDown || ev[1].Time != 300 || ev[1].Y != 10 {
		t.Fatalf("second %+v", ev[1])
	}
}

func TestScanWozduhCrossovers_SkipsAbsentBreaks(t *testing.T) {
	t.Parallel()
	absent := wire.HistoryAbsent
	plots := map[string][]float64{
		"woz_vol_rsi_ema12": {10, absent, 13},
		"woz_vol_rsi_ema5":  {12, 11, 12},
	}
	times := []int64{1, 2, 3}
	ev := nodes.ScanWozduhCrossovers(plots, times, absent)
	if len(ev) != 0 {
		t.Fatalf("gap must not fire across absent, got %+v", ev)
	}
}

func TestWozduhNode_CrossoverClosedBarOnly(t *testing.T) {
	t.Parallel()
	bus := core.NewBus(8)
	n := nodes.NewWozduhNode()
	n.Init(bus)

	// Closed bar 0: A < B, commit prev.
	bus.Cur.Set(core.SlotWozduhVolRsiEma12, 10)
	bus.Cur.Set(core.SlotWozduhVolRsiEma5, 12)
	// TickUpdate overwrites OHLCV then Update() which may NaN unused... full node Update computes real values.
	// Drive SaveState directly after planting slots so we do not depend on live RSI paths.
	n.SaveState()
	if hits := n.LastClosedCrossovers(); len(hits) != 0 {
		t.Fatalf("first close must seed prev, got %+v", hits)
	}

	// Forming: Restore then plant a crossing without Save.
	n.RestoreState()
	bus.Cur.Set(core.SlotWozduhVolRsiEma12, 13)
	bus.Cur.Set(core.SlotWozduhVolRsiEma5, 12)
	if hits := n.LastClosedCrossovers(); len(hits) != 0 {
		t.Fatalf("forming must not emit, leftover %+v", hits)
	}

	// Closed cross up.
	n.RestoreState()
	bus.Cur.Set(core.SlotWozduhVolRsiEma12, 13)
	bus.Cur.Set(core.SlotWozduhVolRsiEma5, 12)
	n.SaveState()
	hits := n.LastClosedCrossovers()
	if len(hits) == 0 {
		t.Fatal("closed cross missing")
	}
	found := false
	for _, h := range hits {
		if h.Pair == nodes.WozduhXoverVolEma12XEma5 {
			found = true
			if h.Side != nodes.WozduhXoverSideUp || h.Y != 12 {
				t.Fatalf("hit %+v", h)
			}
		}
	}
	if !found {
		t.Fatalf("pair1 missing: %+v", hits)
	}

	// Restore forming poison attempt: A goes back below; no Save.
	n.RestoreState()
	bus.Cur.Set(core.SlotWozduhVolRsiEma12, 8)
	bus.Cur.Set(core.SlotWozduhVolRsiEma5, 12)
	if hits := n.LastClosedCrossovers(); len(hits) != 0 {
		t.Fatal("Restore must drop last closed hits so forming ticks cannot re-pack them")
	}

	// Next close still uses restored prev (13 vs 12), not the forming 8.
	n.RestoreState()
	bus.Cur.Set(core.SlotWozduhVolRsiEma12, 8)
	bus.Cur.Set(core.SlotWozduhVolRsiEma5, 12)
	n.SaveState()
	hits = n.LastClosedCrossovers()
	foundDown := false
	for _, h := range hits {
		if h.Pair == nodes.WozduhXoverVolEma12XEma5 && h.Side == nodes.WozduhXoverSideDown {
			foundDown = true
			if h.Y != 12 {
				t.Fatalf("down Y %+v", h)
			}
		}
	}
	if !foundDown {
		t.Fatalf("expected down from restored prev, got %+v", hits)
	}
}
