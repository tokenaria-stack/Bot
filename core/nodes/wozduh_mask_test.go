package nodes_test

import (
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/core/nodes"
)

func TestWozduhMaskForPlots_Closure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id   string
		want nodes.WozduhMask
	}{
		{"woz_rsi_price", nodes.WozduhBitOrangeBase},
		{"woz_ema_rsi", nodes.WozduhBitOrangeBase | nodes.WozduhBitGreenEMA},
		{"woz_rsi_rsi", nodes.WozduhBitOrangeBase | nodes.WozduhBitRsiOfRsi},
		{"woz_price_chan_up", nodes.WozduhBitOrangeBase | nodes.WozduhBitPriceChannel},
		{"woz_price_chan_mid", nodes.WozduhBitOrangeBase | nodes.WozduhBitPriceChannel},
		{"woz_price_chan_dn", nodes.WozduhBitOrangeBase | nodes.WozduhBitPriceChannel},
		{"woz_fast", nodes.WozduhBitVolBase | nodes.WozduhBitWt11},
		{"woz_slow", nodes.WozduhBitVolBase | nodes.WozduhBitWt22},
		{"woz_vol_chan_up", nodes.WozduhBitVolBase | nodes.WozduhBitWt22 | nodes.WozduhBitVolChannel},
		{"woz_vol_chan_mid", nodes.WozduhBitVolBase | nodes.WozduhBitWt22 | nodes.WozduhBitVolChannel},
		{"woz_vol_chan_dn", nodes.WozduhBitVolBase | nodes.WozduhBitWt22 | nodes.WozduhBitVolChannel},
		{"woz_vol_cross", nodes.WozduhBitVolBase | nodes.WozduhBitWt11 | nodes.WozduhBitWt22 | nodes.WozduhBitVolCrossPair},
		{"woz_rsi_hl2", nodes.WozduhBitRedRSI},
		{"woz_macd_rsi", nodes.WozduhBitBlackMACD},
		{"woz_rsi_hl2_vol", nodes.WozduhBitNavyRSI},
		{"woz_rsi_ad", nodes.WozduhBitADRSI},
	}
	for _, c := range cases {
		got := nodes.WozduhMaskForPlots([]string{c.id})
		if got != c.want {
			t.Fatalf("%s: got %#b want %#b", c.id, got, c.want)
		}
	}
}

func TestWozduhMaskForPlots_UnknownAndNonWozduh(t *testing.T) {
	t.Parallel()
	if got := nodes.WozduhMaskForPlots([]string{"line_rsx", "line_rsx_signal", "nope", "woz_vol_chan"}); got != 0 {
		t.Fatalf("non-wozduh/unknown/compose must be 0, got %#b", got)
	}
	if nodes.WozduhMaskForPlots(nil) != 0 {
		t.Fatal("nil ids must be 0 (callers use WozduhMaskAll for empty history slots)")
	}
}

func TestWozduhDefaultVisibleMask(t *testing.T) {
	t.Parallel()
	want := nodes.WozduhBitOrangeBase | nodes.WozduhBitVolBase | nodes.WozduhBitWt11 | nodes.WozduhBitWt22 | nodes.WozduhBitRedRSI
	if got := nodes.WozduhDefaultVisibleMask(); got != want {
		t.Fatalf("default visible %#b want %#b", got, want)
	}
}

func TestWozduhMask_ZeroNotAll(t *testing.T) {
	t.Parallel()
	if nodes.WozduhMask(0) == nodes.WozduhMaskAll {
		t.Fatal("zero must not equal compute-all")
	}
	if nodes.NewWozduhNode().Mask() != nodes.WozduhMaskAll {
		t.Fatal("NewWozduhNode must be compute-all")
	}
}

func feedWoz(t *testing.T, node *nodes.WozduhNode, bars int) {
	t.Helper()
	bus := core.NewBus(64)
	node.Init(bus)
	for i := 0; i < bars; i++ {
		base := 100.0 + float64(i)*0.35 + math.Sin(float64(i)/7)*1.2
		bus.Cur.Set(core.SlotPriceOpen, base)
		bus.Cur.Set(core.SlotPriceHigh, base+0.8)
		bus.Cur.Set(core.SlotPriceLow, base-0.7)
		bus.Cur.Set(core.SlotPriceClose, base+math.Cos(float64(i)/5)*0.55)
		bus.Cur.Set(core.SlotVolume, 1000+float64(i)*3.5)
		node.Update()
	}
}

func TestWozduhMasked_SharedBaseOnce(t *testing.T) {
	t.Parallel()
	const bars = 80
	orangeGreen := nodes.WozduhMaskForPlots([]string{"woz_rsi_price", "woz_ema_rsi"})
	n := nodes.NewWozduhNodeMasked(orangeGreen)
	feedWoz(t, n, bars)
	// orange RSI + green EMA once per bar (not doubled).
	if n.StreamUpdates() != 2*bars {
		t.Fatalf("orange+green streams=%d want %d", n.StreamUpdates(), 2*bars)
	}

	volPair := nodes.WozduhMaskForPlots([]string{"woz_fast", "woz_slow"})
	n2 := nodes.NewWozduhNodeMasked(volPair)
	feedWoz(t, n2, bars)
	// volVwap + volRsi + wt11 + wt22
	if n2.StreamUpdates() != 4*bars {
		t.Fatalf("vol shared streams=%d want %d", n2.StreamUpdates(), 4*bars)
	}
}

func TestWozduhMasked_FailClosedNaN(t *testing.T) {
	t.Parallel()
	n := nodes.NewWozduhNodeMasked(nodes.WozduhMaskForPlots([]string{"woz_fast"}))
	bus := core.NewBus(64)
	n.Init(bus)
	bus.Cur.Set(core.SlotPriceHigh, 101)
	bus.Cur.Set(core.SlotPriceLow, 99)
	bus.Cur.Set(core.SlotPriceClose, 100)
	bus.Cur.Set(core.SlotVolume, 10)
	n.Update()
	inactive := []core.Slot{
		core.SlotWozduhRsiPrice, core.SlotWozduhEmaRsi, core.SlotWozduhRsiRsi,
		core.SlotWozduhRsiHl2, core.SlotWozduhMacdRsi, core.SlotWozduhSlow,
		core.SlotWozduhRsiAd, core.SlotWozduhRsiHl2Vol,
		core.SlotWozduhVolChanMid, core.SlotWozduhPriceChanMid, core.SlotWozduhVolCross,
	}
	for _, s := range inactive {
		v := bus.Cur.Get(s)
		if !math.IsNaN(v) {
			t.Fatalf("inactive slot %d want NaN got %v", s, v)
		}
	}
}

func TestWozduhMasked_ZeroMaskNoStreams(t *testing.T) {
	t.Parallel()
	n := nodes.NewWozduhNodeMasked(0)
	feedWoz(t, n, 100)
	if n.StreamUpdates() != 0 {
		t.Fatalf("mask 0 streams=%d", n.StreamUpdates())
	}
}

func TestWozduhMasked_ParityVsAll(t *testing.T) {
	t.Parallel()
	const bars = 200
	outputs := []struct {
		id   string
		slot core.Slot
	}{
		{"woz_rsi_price", core.SlotWozduhRsiPrice},
		{"woz_ema_rsi", core.SlotWozduhEmaRsi},
		{"woz_rsi_rsi", core.SlotWozduhRsiRsi},
		{"woz_fast", core.SlotWozduhFast},
		{"woz_slow", core.SlotWozduhSlow},
		{"woz_rsi_hl2", core.SlotWozduhRsiHl2},
		{"woz_macd_rsi", core.SlotWozduhMacdRsi},
		{"woz_rsi_hl2_vol", core.SlotWozduhRsiHl2Vol},
		{"woz_rsi_ad", core.SlotWozduhRsiAd},
		{"woz_price_chan_up", core.SlotWozduhPriceChanUp},
		{"woz_vol_chan_mid", core.SlotWozduhVolChanMid},
		{"woz_vol_cross", core.SlotWozduhVolCross},
	}

	fullBus := core.NewBus(256)
	full := nodes.NewWozduhNode()
	full.Init(fullBus)

	fullSnaps := make([][]float64, bars)

	drive := func(node *nodes.WozduhNode, bus *core.Bus, i int) {
		base := 100.0 + float64(i)*0.35 + math.Sin(float64(i)/7)*1.2
		bus.Cur.Set(core.SlotPriceOpen, base)
		bus.Cur.Set(core.SlotPriceHigh, base+0.8)
		bus.Cur.Set(core.SlotPriceLow, base-0.7)
		bus.Cur.Set(core.SlotPriceClose, base+math.Cos(float64(i)/5)*0.55)
		bus.Cur.Set(core.SlotVolume, 1000+float64(i)*3.5)
		node.Update()
	}

	for i := 0; i < bars; i++ {
		drive(full, fullBus, i)
		row := make([]float64, len(outputs))
		for j, o := range outputs {
			row[j] = fullBus.Cur.Get(o.slot)
		}
		fullSnaps[i] = row
	}

	for j, o := range outputs {
		mask := nodes.WozduhMaskForPlots([]string{o.id})
		n := nodes.NewWozduhNodeMasked(mask)
		bus := core.NewBus(256)
		n.Init(bus)
		for i := 0; i < bars; i++ {
			drive(n, bus, i)
			got := bus.Cur.Get(o.slot)
			want := fullSnaps[i][j]
			if got != want && !(math.IsNaN(got) && math.IsNaN(want)) {
				t.Fatalf("%s bar %d: got %v want %v", o.id, i, got, want)
			}
		}
	}
}

func TestWozduhMasked_Measure3000(t *testing.T) {
	const bars = 3000
	run := func(mask nodes.WozduhMask) int {
		n := nodes.NewWozduhNodeMasked(mask)
		feedWoz(t, n, bars)
		return n.StreamUpdates()
	}
	full := run(nodes.WozduhMaskAll)
	def := run(nodes.WozduhDefaultVisibleMask())
	zero := run(0)
	if full != 18*bars {
		t.Fatalf("FULL streams=%d want %d", full, 18*bars)
	}
	if def != 6*bars {
		t.Fatalf("default-visible streams=%d want %d", def, 6*bars)
	}
	if zero != 0 {
		t.Fatalf("zero streams=%d", zero)
	}
	t.Logf("3000-bar Wozduh stream Updates: FULL=%d default-visible=%d RSX-only=%d (%.0f%% of full at default)",
		full, def, zero, 100*float64(def)/float64(full))
}
