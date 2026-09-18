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
		{"woz_rsi_close", nodes.WozduhBitRsiClose},
		{"woz_rsi_close_ema7", nodes.WozduhBitRsiClose | nodes.WozduhBitRsiCloseEma7},
		{"woz_rsi_rsi_close", nodes.WozduhBitRsiClose | nodes.WozduhBitRsiOfRsi},
		{"woz_rsi_close_chan_up", nodes.WozduhBitRsiClose | nodes.WozduhBitRsiCloseChan},
		{"woz_rsi_close_chan_mid", nodes.WozduhBitRsiClose | nodes.WozduhBitRsiCloseChan},
		{"woz_rsi_close_chan_dn", nodes.WozduhBitRsiClose | nodes.WozduhBitRsiCloseChan},
		{"woz_vol_rsi_ema12", nodes.WozduhBitVolBase | nodes.WozduhBitVolRsiEma12},
		{"woz_vol_rsi_ema5", nodes.WozduhBitVolBase | nodes.WozduhBitVolRsiEma5},
		{"woz_vol_rsi_ema5_chan_up", nodes.WozduhBitVolBase | nodes.WozduhBitVolRsiEma5 | nodes.WozduhBitVolRsiEma5Chan},
		{"woz_vol_rsi_ema5_chan_mid", nodes.WozduhBitVolBase | nodes.WozduhBitVolRsiEma5 | nodes.WozduhBitVolRsiEma5Chan},
		{"woz_vol_rsi_ema5_chan_dn", nodes.WozduhBitVolBase | nodes.WozduhBitVolRsiEma5 | nodes.WozduhBitVolRsiEma5Chan},
		{"woz_vol_cross", nodes.WozduhBitVolBase | nodes.WozduhBitVolRsiEma12 | nodes.WozduhBitVolRsiEma5 | nodes.WozduhBitVolCrossPair},
		{"woz_rsi_hl2", nodes.WozduhBitRsiHl2},
		{"woz_macd_rsi_close", nodes.WozduhBitMacdRsiClose},
		{"woz_rsi_hl2_vwema", nodes.WozduhBitRsiHl2Vwema},
		{"woz_rsi_ad", nodes.WozduhBitRsiAd},
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
	if got := nodes.WozduhMaskForPlots([]string{"line_rsx", "line_rsx_signal", "nope", "woz_vol_rsi_ema5_chan"}); got != 0 {
		t.Fatalf("non-wozduh/unknown/compose must be 0, got %#b", got)
	}
	if nodes.WozduhMaskForPlots(nil) != 0 {
		t.Fatal("nil ids must be 0 (callers use WozduhMaskAll for empty history slots)")
	}
}

func TestWozduhDefaultVisibleMask(t *testing.T) {
	t.Parallel()
	want := nodes.WozduhBitRsiClose | nodes.WozduhBitVolBase | nodes.WozduhBitVolRsiEma12 | nodes.WozduhBitVolRsiEma5 | nodes.WozduhBitRsiHl2
	if got := nodes.WozduhDefaultVisibleMask(); got != want {
		t.Fatalf("default visible %#b want %#b", got, want)
	}
}

func TestWozduhMaskFromClientSubscriptions(t *testing.T) {
	t.Parallel()
	if nodes.WozduhMaskFromClientSubscriptions(nil) != 0 {
		t.Fatal("no clients must be 0")
	}
	if nodes.WozduhMaskFromClientSubscriptions([][]string{nil}) != nodes.WozduhMaskAll {
		t.Fatal("nil slots must be all")
	}
	if nodes.WozduhMaskFromClientSubscriptions([][]string{{}}) != nodes.WozduhMaskAll {
		t.Fatal("empty slots must be all")
	}
	a := nodes.WozduhMaskForPlots([]string{"woz_vol_rsi_ema12"})
	b := nodes.WozduhMaskForPlots([]string{"woz_rsi_close_chan_up"})
	got := nodes.WozduhMaskFromClientSubscriptions([][]string{
		{"woz_vol_rsi_ema12"},
		{"woz_rsi_close_chan_up"},
	})
	if got != a|b {
		t.Fatalf("union %#b want %#b", got, a|b)
	}
	if nodes.WozduhMaskFromClientSubscriptions([][]string{{"nope", "line_rsx"}}) != 0 {
		t.Fatal("unknown/non-wozduh must contribute 0")
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
	rsiCloseEma7 := nodes.WozduhMaskForPlots([]string{"woz_rsi_close", "woz_rsi_close_ema7"})
	n := nodes.NewWozduhNodeMasked(rsiCloseEma7)
	feedWoz(t, n, bars)
	// RSI(close) + EMA7 once per bar (not doubled).
	if n.StreamUpdates() != 2*bars {
		t.Fatalf("rsiClose+ema7 streams=%d want %d", n.StreamUpdates(), 2*bars)
	}

	volPair := nodes.WozduhMaskForPlots([]string{"woz_vol_rsi_ema12", "woz_vol_rsi_ema5"})
	n2 := nodes.NewWozduhNodeMasked(volPair)
	feedWoz(t, n2, bars)
	// volVwema + volRsi + ema12 + ema5
	if n2.StreamUpdates() != 4*bars {
		t.Fatalf("vol shared streams=%d want %d", n2.StreamUpdates(), 4*bars)
	}
}

func TestWozduhMasked_FailClosedNaN(t *testing.T) {
	t.Parallel()
	n := nodes.NewWozduhNodeMasked(nodes.WozduhMaskForPlots([]string{"woz_vol_rsi_ema12"}))
	bus := core.NewBus(64)
	n.Init(bus)
	bus.Cur.Set(core.SlotPriceHigh, 101)
	bus.Cur.Set(core.SlotPriceLow, 99)
	bus.Cur.Set(core.SlotPriceClose, 100)
	bus.Cur.Set(core.SlotVolume, 10)
	n.Update()
	inactive := []core.Slot{
		core.SlotWozduhRsiClose, core.SlotWozduhRsiCloseEma7, core.SlotWozduhRsiRsiClose,
		core.SlotWozduhRsiHl2, core.SlotWozduhMacdRsiClose, core.SlotWozduhVolRsiEma5,
		core.SlotWozduhRsiAd, core.SlotWozduhRsiHl2Vwema,
		core.SlotWozduhVolRsiEma5ChanMid, core.SlotWozduhRsiCloseChanMid, core.SlotWozduhVolCross,
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
		{"woz_rsi_close", core.SlotWozduhRsiClose},
		{"woz_rsi_close_ema7", core.SlotWozduhRsiCloseEma7},
		{"woz_rsi_rsi_close", core.SlotWozduhRsiRsiClose},
		{"woz_vol_rsi_ema12", core.SlotWozduhVolRsiEma12},
		{"woz_vol_rsi_ema5", core.SlotWozduhVolRsiEma5},
		{"woz_rsi_hl2", core.SlotWozduhRsiHl2},
		{"woz_macd_rsi_close", core.SlotWozduhMacdRsiClose},
		{"woz_rsi_hl2_vwema", core.SlotWozduhRsiHl2Vwema},
		{"woz_rsi_ad", core.SlotWozduhRsiAd},
		{"woz_rsi_close_chan_up", core.SlotWozduhRsiCloseChanUp},
		{"woz_vol_rsi_ema5_chan_mid", core.SlotWozduhVolRsiEma5ChanMid},
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

func TestWozduhBitVolBase_IsComputeMaskNotBinanceTakerBuyV(t *testing.T) {
	t.Parallel()
	// WozduhBitVolBase gates volume-RSI work. It is not Binance kline "V".
	if nodes.WozduhBitVolBase == 0 {
		t.Fatal("mask bit")
	}
	if nodes.WozduhMaskForPlots([]string{"woz_vol_rsi_ema5"})&nodes.WozduhBitVolBase == 0 {
		t.Fatal("woz_vol_rsi_ema5 needs VolBase compute")
	}
}
