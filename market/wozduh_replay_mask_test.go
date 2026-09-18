package market

import (
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/core/nodes"
)

func TestReplayClosedBarsMasked_ParityAndRSX(t *testing.T) {
	t.Parallel()
	klines := syntheticKlines(400)
	rsx := NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	full := ReplayClosedBars(klines, rsx)
	masked := ReplayClosedBarsMasked(klines, rsx, nodes.WozduhMaskForPlots([]string{
		"woz_rsi_close", "woz_rsi_close_ema7", "woz_rsi_rsi_close", "woz_vol_rsi_ema12", "woz_vol_rsi_ema5",
		"woz_rsi_hl2", "woz_macd_rsi_close", "woz_rsi_hl2_vwema", "woz_rsi_ad",
		"woz_rsi_close_chan_up", "woz_vol_rsi_ema5_chan_mid",
	}))
	if full.Hist == nil || masked.Hist == nil {
		t.Fatal("hist missing")
	}
	if full.Hist.Count() != masked.Hist.Count() {
		t.Fatalf("count full=%d masked=%d", full.Hist.Count(), masked.Hist.Count())
	}
	n := full.Hist.Count()
	slots := []core.Slot{
		core.SlotJurikRSX, core.SlotJurikSignal,
		core.SlotWozduhRsiClose, core.SlotWozduhRsiCloseEma7, core.SlotWozduhRsiRsiClose,
		core.SlotWozduhVolRsiEma12, core.SlotWozduhVolRsiEma5, core.SlotWozduhRsiHl2,
		core.SlotWozduhMacdRsiClose, core.SlotWozduhRsiHl2Vwema, core.SlotWozduhRsiAd,
		core.SlotWozduhRsiCloseChanUp, core.SlotWozduhVolRsiEma5ChanMid,
	}
	for lookback := 1; lookback <= n; lookback++ {
		for _, s := range slots {
			a := full.Hist.Get(s, lookback)
			b := masked.Hist.Get(s, lookback)
			if a != b && !(math.IsNaN(a) && math.IsNaN(b)) {
				t.Fatalf("slot %d lookback %d: full=%v masked=%v", s, lookback, a, b)
			}
		}
	}
	if len(full.ZZFacts) != len(masked.ZZFacts) {
		t.Fatalf("ZZ facts full=%d masked=%d", len(full.ZZFacts), len(masked.ZZFacts))
	}
}

func TestReplayClosedBarsMasked_ZeroWozduh(t *testing.T) {
	t.Parallel()
	klines := syntheticKlines(200)
	rsx := NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	full := ReplayClosedBars(klines, rsx)
	zero := ReplayClosedBarsMasked(klines, rsx, 0)
	n := full.Hist.Count()
	for lookback := 1; lookback <= n; lookback++ {
		a := full.Hist.Get(core.SlotJurikRSX, lookback)
		b := zero.Hist.Get(core.SlotJurikRSX, lookback)
		if a != b && !(math.IsNaN(a) && math.IsNaN(b)) {
			t.Fatalf("RSX lookback %d: full=%v zero=%v", lookback, a, b)
		}
		woz := zero.Hist.Get(core.SlotWozduhVolRsiEma12, lookback)
		if !math.IsNaN(woz) {
			t.Fatalf("zero-mask woz_vol_rsi_ema12 lookback %d = %v want NaN", lookback, woz)
		}
	}
	if len(full.ZZFacts) != len(zero.ZZFacts) {
		t.Fatalf("ZZ facts changed under woz mask 0: %d vs %d", len(full.ZZFacts), len(zero.ZZFacts))
	}
}

func TestReplayClosedBars_DefaultComputeAll(t *testing.T) {
	t.Parallel()
	klines := syntheticKlines(80)
	rsx := NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	a := ReplayClosedBars(klines, rsx)
	b := ReplayClosedBarsMasked(klines, rsx, nodes.WozduhMaskAll)
	n := a.Hist.Count()
	for lookback := 1; lookback <= n; lookback++ {
		x := a.Hist.Get(core.SlotWozduhMacdRsiClose, lookback)
		y := b.Hist.Get(core.SlotWozduhMacdRsiClose, lookback)
		if x != y && !(math.IsNaN(x) && math.IsNaN(y)) {
			t.Fatalf("compute-all mismatch lookback %d %v vs %v", lookback, x, y)
		}
	}
}
