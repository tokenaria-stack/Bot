package market

import (
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/exchange"
)

func testDemandFrame(t *testing.T, n int) *Frame {
	t.Helper()
	return NewFrame(syntheticKlines(n), "15s", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
}

func appendClosedBar(f *Frame, i int) {
	start := int64(1_700_000_000_000)
	step := int64(60_000)
	open := start + int64(i)*step
	price := 100.0 + float64(i)*0.01
	f.UpdateKlineTick(exchange.Kline{
		OpenTime:  open,
		CloseTime: open + step - 1,
		Open:      price,
		High:      price + 1,
		Low:       price - 1,
		Close:     price + 0.5,
		Volume:    10,
	}, true)
}

func formingBar(i int, close float64) exchange.Kline {
	start := int64(1_700_000_000_000)
	step := int64(60_000)
	open := start + int64(i)*step
	price := 100.0 + float64(i)*0.01
	return exchange.Kline{
		OpenTime:  open,
		CloseTime: open + step - 1,
		Open:      price,
		High:      math.Max(price+1, close),
		Low:       math.Min(price-1, close),
		Close:     close,
		Volume:    10,
	}
}

func TestWozduhDemand_ChartOnlyZero(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := testDemandFrame(t, 80)
	mask, streams, wakes := f.WozduhLiveStats()
	if mask != 0 {
		t.Fatalf("ChartOnly unused mask=%#b want 0", mask)
	}
	if streams != 0 {
		t.Fatalf("ChartOnly unused streams=%d want 0", streams)
	}
	if wakes != 0 {
		t.Fatalf("wakes=%d", wakes)
	}
	if !math.IsNaN(f.WozduhSlot(core.SlotWozduhFast)) {
		t.Fatal("unused woz_fast must be NaN")
	}
	if !math.IsNaN(f.WozduhSlot(core.SlotWozduhVolCross)) {
		t.Fatal("VolCross must not be mandatory")
	}
}

func TestWozduhDemand_LiveMandatoryFastSlow(t *testing.T) {
	withEngineMode(t, EngineModeLive)
	f := testDemandFrame(t, 80)
	want := nodes.WozduhBitVolBase | nodes.WozduhBitWt11 | nodes.WozduhBitWt22
	mask, s0, _ := f.WozduhLiveStats()
	if mask != want {
		t.Fatalf("Live unused mask=%#b want %#b", mask, want)
	}
	if math.IsNaN(f.WozduhSlot(core.SlotWozduhFast)) || math.IsNaN(f.WozduhSlot(core.SlotWozduhSlow)) {
		t.Fatal("Live shadow woz_fast/slow must stay finite")
	}
	if !math.IsNaN(f.WozduhSlot(core.SlotWozduhVolCross)) {
		t.Fatal("VolCross must not be mandatory in Live")
	}
	for i := 80; i < 90; i++ {
		appendClosedBar(f, i)
	}
	_, s1, _ := f.WozduhLiveStats()
	if s1 <= s0 {
		t.Fatalf("Live internal streams must still Update: %d → %d", s0, s1)
	}
}

func TestWozduhDemand_SleepStopsAndNaNs(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := testDemandFrame(t, 80)
	f.SetWozduhDemand(nodes.WozduhDefaultVisibleMask())
	_, streams0, _ := f.WozduhLiveStats()
	for i := 80; i < 100; i++ {
		appendClosedBar(f, i)
	}
	_, streams1, _ := f.WozduhLiveStats()
	if streams1 <= streams0 {
		t.Fatalf("expected live streams to grow, before=%d after=%d", streams0, streams1)
	}
	f.SetWozduhDemand(0)
	if !math.IsNaN(f.WozduhSlot(core.SlotWozduhFast)) {
		t.Fatal("sleep must NaN immediately")
	}
	_, mid, _ := f.WozduhLiveStats()
	for i := 100; i < 130; i++ {
		appendClosedBar(f, i)
	}
	_, end, _ := f.WozduhLiveStats()
	if end != mid {
		t.Fatalf("sleeping streams still updating: %d → %d", mid, end)
	}
}

func TestWozduhDemand_RepeatedDemandNoWake(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := testDemandFrame(t, 80)
	f.SetWozduhDemand(nodes.WozduhDefaultVisibleMask())
	_, _, w1 := f.WozduhLiveStats()
	f.SetWozduhDemand(nodes.WozduhDefaultVisibleMask())
	f.SetWozduhDemand(nodes.WozduhDefaultVisibleMask())
	_, _, w2 := f.WozduhLiveStats()
	if w2 != w1 {
		t.Fatalf("repeated identical demand rebuilt: wakes %d → %d", w1, w2)
	}
}

func TestWozduhDemand_SharedBasePreserved(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := testDemandFrame(t, 120)
	f.SetWozduhDemand(nodes.WozduhMaskForPlots([]string{"woz_rsi_price"}))
	ptr := f.WozduhOrangePtr()
	if ptr == nil {
		t.Fatal("orange pointer")
	}
	for i := 120; i < 200; i++ {
		appendClosedBar(f, i)
	}
	f.SetWozduhDemand(nodes.WozduhMaskForPlots([]string{"woz_rsi_price", "woz_ema_rsi"}))
	if f.WozduhOrangePtr() != ptr {
		t.Fatal("live orangeRsi must not be replaced when GreenEMA wakes")
	}
	if math.IsNaN(f.WozduhSlot(core.SlotWozduhEmaRsi)) {
		t.Fatal("woken ema must be finite")
	}
}

func TestWozduhDemand_SharedVolBasePreserved(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := testDemandFrame(t, 120)
	f.SetWozduhDemand(nodes.WozduhMaskForPlots([]string{"woz_fast"}))
	ptr := f.WozduhWt11Ptr()
	if ptr == nil {
		t.Fatal("wt11 pointer")
	}
	for i := 120; i < 180; i++ {
		appendClosedBar(f, i)
	}
	f.SetWozduhDemand(nodes.WozduhMaskForPlots([]string{"woz_fast", "woz_slow"}))
	if f.WozduhWt11Ptr() != ptr {
		t.Fatal("live wt11 must not be replaced when Wt22 wakes")
	}
	if math.IsNaN(f.WozduhSlot(core.SlotWozduhSlow)) {
		t.Fatal("woken woz_slow must be finite")
	}
}

func TestWozduhDemand_WakeMatchesFreshReplay(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	klines := syntheticKlines(180)
	f := NewFrame(klines, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	f.SetWozduhDemand(nodes.WozduhMaskForPlots([]string{"woz_rsi_price"}))
	for i := 180; i < 280; i++ {
		appendClosedBar(f, i)
	}
	f.SetWozduhDemand(nodes.WozduhMaskForPlots([]string{"woz_rsi_price", "woz_ema_rsi"}))
	got := f.WozduhSlot(core.SlotWozduhEmaRsi)
	closed := f.GetKlines()
	if n := len(closed); n > 0 && f.LastCommittedOpenTime() != closed[n-1].OpenTime {
		closed = closed[:n-1]
	}
	ref := nodes.NewWozduhNodeMasked(nodes.WozduhBitOrangeBase | nodes.WozduhBitGreenEMA)
	replayWozduhClosedBars(ref, closed)
	want := ref.Slot(core.SlotWozduhEmaRsi)
	if got != want && !(math.IsNaN(got) && math.IsNaN(want)) {
		t.Fatalf("wake vs fresh replay: got %v want %v", got, want)
	}
}

func TestWozduhDemand_WakeVsAlwaysOnEpsilon(t *testing.T) {
	// Live birth runs VolBase|Wt11|Wt22 over every init bar (older than the
	// retained dagHistoryCap window). ChartOnly wake rebuilds only that window.
	// Bit-exact equality is not required (contract B).
	n := dagHistoryCap + 80
	withEngineMode(t, EngineModeLive)
	always := NewFrame(syntheticKlines(n), "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	want := always.WozduhSlot(core.SlotWozduhFast)

	SetEngineMode(EngineModeChartOnly)
	woken := NewFrame(syntheticKlines(n), "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	woken.SetWozduhDemand(nodes.WozduhMaskForPlots([]string{"woz_fast", "woz_slow"}))
	got := woken.WozduhSlot(core.SlotWozduhFast)
	if math.IsNaN(got) || math.IsNaN(want) {
		t.Fatal("woz_fast must be finite")
	}
	if math.Abs(got-want) > dagShadowEpsilon {
		t.Fatalf("wake vs always-on |Δ|=%g > dagShadowEpsilon", math.Abs(got-want))
	}
}

func TestWozduhDemand_InternalClosureMatchesShadowReads(t *testing.T) {
	want := nodes.WozduhBitVolBase | nodes.WozduhBitWt11 | nodes.WozduhBitWt22
	if got := nodes.WozduhMaskForPlots([]string{"woz_fast", "woz_slow"}); got != want {
		t.Fatalf("shadow woz_fast/slow closure %#b want %#b", got, want)
	}
	withEngineMode(t, EngineModeChartOnly)
	if wozduhInternalMask() != 0 {
		t.Fatal("ChartOnly internalMask must be 0")
	}
	SetEngineMode(EngineModeLive)
	if wozduhInternalMask() != want {
		t.Fatalf("Live internalMask %#b want %#b", wozduhInternalMask(), want)
	}
}

func TestWozduhDemand_NilSlotsAll(t *testing.T) {
	if nodes.WozduhMaskFromClientSubscriptions([][]string{nil, {"woz_fast"}}) != nodes.WozduhMaskAll {
		t.Fatal("any unfiltered client forces all")
	}
	if nodes.WozduhMaskFromClientSubscriptions([][]string{{"nope"}}) != 0 {
		t.Fatal("unknown IDs contribute 0")
	}
}

func TestWozduhDemand_VolCrossWakesCoherentPrev(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := testDemandFrame(t, 120)
	f.SetWozduhDemand(nodes.WozduhMaskForPlots([]string{"woz_fast", "woz_slow"}))
	wt := f.WozduhWt11Ptr()
	f.SetWozduhDemand(nodes.WozduhMaskForPlots([]string{"woz_fast", "woz_slow", "woz_vol_cross"}))
	if f.WozduhWt11Ptr() != wt {
		t.Fatal("VolCross wake must not replace live wt11")
	}
	cross := f.WozduhSlot(core.SlotWozduhVolCross)
	if math.IsNaN(cross) {
		t.Fatal("woken VolCross must be finite (0/-1/1)")
	}
}

func TestWozduhDemand_FormingWakeUsesClosedBaseline(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := testDemandFrame(t, 80)
	f.SetWozduhDemand(nodes.WozduhMaskForPlots([]string{"woz_rsi_price"}))
	f.UpdateKlineTick(formingBar(80, 101.2), false)
	_, _, w0 := f.WozduhLiveStats()
	f.SetWozduhDemand(nodes.WozduhMaskForPlots([]string{"woz_rsi_price", "woz_ema_rsi"}))
	_, _, w1 := f.WozduhLiveStats()
	if w1 != w0+1 {
		t.Fatalf("forming wake rebuilds=%d want 1", w1-w0)
	}
	if math.IsNaN(f.WozduhSlot(core.SlotWozduhEmaRsi)) {
		t.Fatal("wake must install closed baseline before the next tick")
	}
	f.UpdateKlineTick(formingBar(80, 101.4), false)
	if math.IsNaN(f.WozduhSlot(core.SlotWozduhEmaRsi)) {
		t.Fatal("next forming tick must evaluate woken ema")
	}
}

func TestWozduhDemand_Measure(t *testing.T) {
	const extra = 20
	tick := func(f *Frame, from, to int) {
		for i := from; i < to; i++ {
			appendClosedBar(f, i)
		}
	}

	withEngineMode(t, EngineModeLive)
	full := NewFrame(syntheticKlines(40), "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	full.SetWozduhDemand(nodes.WozduhMaskAll)
	_, s0, _ := full.WozduhLiveStats()
	tick(full, 40, 40+extra)
	_, s1, _ := full.WozduhLiveStats()
	fullPer := (s1 - s0) / extra

	def := NewFrame(syntheticKlines(40), "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	def.SetWozduhDemand(nodes.WozduhDefaultVisibleMask())
	_, d0, _ := def.WozduhLiveStats()
	tick(def, 40, 40+extra)
	_, d1, _ := def.WozduhLiveStats()
	defPer := (d1 - d0) / extra

	liveUnused := NewFrame(syntheticKlines(40), "15s", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	_, lu0, _ := liveUnused.WozduhLiveStats()
	tick(liveUnused, 40, 40+extra)
	_, lu1, _ := liveUnused.WozduhLiveStats()
	liveUnusedPer := (lu1 - lu0) / extra

	SetEngineMode(EngineModeChartOnly)
	zero := NewFrame(syntheticKlines(40), "15s", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	_, z0, _ := zero.WozduhLiveStats()
	tick(zero, 40, 40+extra)
	_, z1, _ := zero.WozduhLiveStats()

	seconds := []string{"1s", "5s", "10s", "15s", "30s", "45s"}
	for _, tf := range seconds {
		sf := NewFrame(syntheticKlines(20), tf, ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
		_, a, _ := sf.WozduhLiveStats()
		tick(sf, 20, 20+extra)
		_, b, _ := sf.WozduhLiveStats()
		t.Logf("ChartOnly unused %s Wozduh streams: %d → %d (Δ %d)", tf, a, b, b-a)
		if a != b {
			t.Fatalf("%s unused still computing: %d → %d", tf, a, b)
		}
	}

	t.Logf("persistent Wozduh streams/update: ALL=%d default-visible=%d Live-unused=%d ChartOnly-unused=%d",
		fullPer, defPer, liveUnusedPer, z1-z0)
	if fullPer != 18 {
		t.Fatalf("ALL streams/bar=%d want 18", fullPer)
	}
	if defPer != 6 {
		t.Fatalf("default streams/bar=%d want 6", defPer)
	}
	if liveUnusedPer != 4 {
		t.Fatalf("Live unused streams/bar=%d want 4 (VolBase×2 + Wt11 + Wt22)", liveUnusedPer)
	}
	if z1 != z0 {
		t.Fatalf("unused seconds Frame still computing: %d → %d", z0, z1)
	}
}
