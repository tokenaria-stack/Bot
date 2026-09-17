package nodes_test

import (
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/core/nodes"
)

const wozduhParityEps = 1e-9

func wozduhSynthOHLCV(i int) (open, high, low, close, volume float64) {
	base := 100.0 + float64(i)*0.35 + math.Sin(float64(i)/7)*1.2
	open = base
	high = base + 0.8 + math.Mod(float64(i), 5)*0.15
	low = base - 0.7 - math.Mod(float64(i*3), 4)*0.1
	close = base + math.Cos(float64(i)/5)*0.55
	volume = 1000.0 + float64(i)*3.5 + math.Mod(float64(i*11), 50)
	return open, high, low, close, volume
}

func tickWozduh(n *nodes.WozduhNode, bus *core.Bus, i int) {
	o, h, l, c, v := wozduhSynthOHLCV(i)
	bus.Cur.Set(core.SlotPriceOpen, o)
	bus.Cur.Set(core.SlotPriceHigh, h)
	bus.Cur.Set(core.SlotPriceLow, l)
	bus.Cur.Set(core.SlotPriceClose, c)
	bus.Cur.Set(core.SlotVolume, v)
	n.Update()
}

func TestWozduh_DeterministicTwinAndFrozenBar20(t *testing.T) {
	t.Parallel()

	aBus := core.NewBus(256)
	bBus := core.NewBus(256)
	a := nodes.NewWozduhNode()
	b := nodes.NewWozduhNode()
	a.Init(aBus)
	b.Init(bBus)

	const bars = 150
	slots := []core.Slot{
		core.SlotWozduhRsiPrice, core.SlotWozduhEmaRsi, core.SlotWozduhRsiRsi, core.SlotWozduhRsiHl2,
		core.SlotWozduhMacdRsi, core.SlotWozduhFast, core.SlotWozduhSlow, core.SlotWozduhRsiAd,
		core.SlotWozduhRsiHl2Vol, core.SlotWozduhVolChanMid, core.SlotWozduhVolChanUp, core.SlotWozduhVolChanDn,
		core.SlotWozduhPriceChanMid, core.SlotWozduhPriceChanUp, core.SlotWozduhPriceChanDn, core.SlotWozduhVolCross,
	}
	for i := 0; i < bars; i++ {
		tickWozduh(a, aBus, i)
		tickWozduh(b, bBus, i)
		for _, slot := range slots {
			got, want := aBus.Cur.Get(slot), bBus.Cur.Get(slot)
			if math.Abs(got-want) > wozduhParityEps && !(math.IsNaN(got) && math.IsNaN(want)) {
				t.Fatalf("bar %d slot %v twin drift got=%v want=%v", i, slot, got, want)
			}
		}
		rsi := aBus.Cur.Get(core.SlotWozduhRsiPrice)
		if !math.IsNaN(rsi) && (rsi < 0 || rsi > 100) {
			t.Fatalf("bar %d RsiPrice %v out of [0,100]", i, rsi)
		}
		cross := aBus.Cur.Get(core.SlotWozduhVolCross)
		if !math.IsNaN(cross) && cross != 0 && cross != 1 && cross != -1 {
			t.Fatalf("bar %d VolCross %v want -1/0/1", i, cross)
		}
	}

	// Frozen fixture: canonical WozduhNode on this synthetic, bar 20 (pre-saturation).
	fixBus := core.NewBus(256)
	fix := nodes.NewWozduhNode()
	fix.Init(fixBus)
	for i := 0; i <= 20; i++ {
		tickWozduh(fix, fixBus, i)
	}
	cur := fixBus.Cur
	gold := []struct {
		name string
		got  float64
		want float64
	}{
		{"RsiPrice", cur.Get(core.SlotWozduhRsiPrice), 100},
		{"EmaRsi", cur.Get(core.SlotWozduhEmaRsi), 99.682878806106601},
		{"wt11", cur.Get(core.SlotWozduhFast), 96.460135347262565},
		{"wt22", cur.Get(core.SlotWozduhSlow), 99.969927134017837},
		{"RsiHl2", cur.Get(core.SlotWozduhRsiHl2), 98.772388121028527},
		{"MacdRsi", cur.Get(core.SlotWozduhMacdRsi), 68.552211722386232},
		{"VolCross", cur.Get(core.SlotWozduhVolCross), 0},
	}
	for _, c := range gold {
		if math.Abs(c.got-c.want) > 1e-9 {
			t.Fatalf("frozen bar 20 %s: got %v want %v", c.name, c.got, c.want)
		}
	}
}
