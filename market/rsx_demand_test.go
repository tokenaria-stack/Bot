package market

import (
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

func demandWaveKlines(n int) []exchange.Kline {
	klines := make([]exchange.Kline, n)
	base := int64(1_700_000_000_000)
	for i := range klines {
		wave := float64(i%40) - 20
		px := 50000 + wave*8 + float64(i)
		klines[i] = exchange.Kline{
			OpenTime:  base + int64(i)*60_000,
			CloseTime: base + int64(i)*60_000 + 59_999,
			Open:      px,
			High:      px + 10,
			Low:       px - 10,
			Close:     px + 5,
			Volume:    100,
		}
	}
	return klines
}

func appendDemandBar(f *Frame, i int) {
	start := int64(1_700_000_000_000)
	step := int64(60_000)
	open := start + int64(i)*step
	wave := float64(i%40) - 20
	px := 50000 + wave*8 + float64(i)
	f.UpdateKlineTick(exchange.Kline{
		OpenTime:  open,
		CloseTime: open + step - 1,
		Open:      px,
		High:      px + 10,
		Low:       px - 10,
		Close:     px + 5,
		Volume:    100,
	}, true)
}

func TestRSXDemand_ChartOnlyMicroZero(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	tfs := []string{"1s", "5s", "10s", "15s", "30s", "45s"}
	for _, tf := range tfs {
		f := NewFrame(syntheticKlines(80), tf, ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
		mask, rsxU, zzU, tv, fr, zz, _, rebuilds := f.RSXLiveStats()
		if mask != 0 {
			t.Fatalf("%s mask=%#b want 0", tf, mask)
		}
		if rsxU != 0 || zzU != 0 || tv != 0 || fr != 0 || zz != 0 || rebuilds != 0 {
			t.Fatalf("%s work rsx=%d zz=%d tv=%d fr=%d zzE=%d rebuilds=%d",
				tf, rsxU, zzU, tv, fr, zz, rebuilds)
		}
		if !math.IsNaN(f.RSXSlot(core.SlotJurikRSX)) || !math.IsNaN(f.RSXSlot(core.SlotJurikSignal)) {
			t.Fatalf("%s Jurik must be NaN when unused", tf)
		}
		woz, wozU, _ := f.WozduhLiveStats()
		if woz != 0 || wozU != 0 {
			t.Fatalf("%s Wozduh must stay 0 (got mask=%#b streams=%d)", tf, woz, wozU)
		}
		baseU := rsxU
		for i := 80; i < 100; i++ {
			appendClosedBar(f, i)
		}
		_, rsx1, zz1, tv1, fr1, zz1c, _, _ := f.RSXLiveStats()
		if rsx1 != baseU || zz1 != 0 || tv1 != 0 || fr1 != 0 || zz1c != 0 {
			t.Fatalf("%s unused still computing rsx=%d zz=%d tv=%d fr=%d zzE=%d",
				tf, rsx1, zz1, tv, fr, zz1c)
		}
		t.Logf("ChartOnly unused %s: RSX=%d ZZ=%d TV=%d Fractal=%d ZZCol=%d Wozduh=0",
			tf, rsx1, zz1, tv1, fr1, zz1c)
	}
}

func TestRSXDemand_LiveInternalCoreOnly(t *testing.T) {
	withEngineMode(t, EngineModeLive)
	f := NewFrame(syntheticKlines(80), "15s", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	mask, rsx0, zzU, tv, fr, zz, _, _ := f.RSXLiveStats()
	if mask != nodes.NeedRSXCore {
		t.Fatalf("Live unused mask=%#b want Core only", mask)
	}
	if zzU != 0 || tv != 0 || fr != 0 || zz != 0 {
		t.Fatalf("Live unused detectors must sleep zz=%d tv=%d fr=%d zzE=%d", zzU, tv, fr, zz)
	}
	if math.IsNaN(f.RSXSlot(core.SlotJurikRSX)) || math.IsNaN(f.RSXSlot(core.SlotJurikSignal)) {
		t.Fatal("Live shadow Jurik must stay finite")
	}
	for i := 80; i < 90; i++ {
		appendClosedBar(f, i)
	}
	_, rsx1, zzUpd, tv1, fr1, zzCol, _, _ := f.RSXLiveStats()
	if rsx1 <= rsx0 {
		t.Fatalf("Live Core must still Update: %d → %d", rsx0, rsx1)
	}
	if zzUpd != 0 || tv1 != 0 || fr1 != 0 || zzCol != 0 {
		t.Fatalf("detectors must stay asleep zz=%d tv=%d fr=%d zzE=%d", zzUpd, tv1, fr1, zzCol)
	}
}

func TestRSXDemand_SleepNaNsAndStops(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := NewFrame(syntheticKlines(80), "15s", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	f.SetRSXDemand(nodes.NeedRSXCore)
	_, u0, _, _, _, _, _, _ := f.RSXLiveStats()
	for i := 80; i < 90; i++ {
		appendClosedBar(f, i)
	}
	_, u1, _, _, _, _, _, _ := f.RSXLiveStats()
	if u1 <= u0 {
		t.Fatalf("Core streams must grow %d → %d", u0, u1)
	}
	f.SetRSXDemand(0)
	if !math.IsNaN(f.RSXSlot(core.SlotJurikRSX)) || !math.IsNaN(f.RSXSlot(core.SlotJurikSignal)) {
		t.Fatal("sleep must NaN RSX slots immediately")
	}
	_, mid, _, _, _, _, _, _ := f.RSXLiveStats()
	for i := 90; i < 110; i++ {
		appendClosedBar(f, i)
	}
	_, end, _, _, _, _, _, _ := f.RSXLiveStats()
	if end != mid {
		t.Fatalf("sleeping Core still updating %d → %d", mid, end)
	}
}

func TestRSXDemand_RepeatedDemandNoReplay(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := NewFrame(syntheticKlines(80), "15s", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	f.SetRSXDemand(nodes.NeedRSXCore)
	_, _, _, _, _, _, _, r0 := f.RSXLiveStats()
	f.SetRSXDemand(nodes.NeedRSXCore)
	f.SetRSXDemand(nodes.NeedRSXCore)
	_, _, _, _, _, _, _, r1 := f.RSXLiveStats()
	if r1 != r0 {
		t.Fatalf("identical demand replayed Core %d → %d", r0, r1)
	}
}

func TestRSXDemand_WakeParityFreshWindow(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	klines := syntheticKlines(120)
	asleep := NewFrame(klines, "15s", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	asleep.SetRSXDemand(nodes.NeedRSXCore)
	fresh := ReplayDAGKlines(klines, NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"}))
	got := asleep.RSXSlot(core.SlotJurikRSX)
	want := fresh.Get(core.SlotJurikRSX, 1)
	if got != want {
		t.Fatalf("wake Jurik=%v fresh=%v", got, want)
	}
	gotS := asleep.RSXSlot(core.SlotJurikSignal)
	wantS := fresh.Get(core.SlotJurikSignal, 1)
	if gotS != wantS {
		t.Fatalf("wake signal=%v fresh=%v", gotS, wantS)
	}
}

func TestRSXDemand_AuthoritativeRSXWhenFamilyWakes(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	n := 160
	f := NewFrame(demandWaveKlines(80), "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	f.SetRSXDemand(nodes.NeedRSXCore)
	for i := 80; i < n; i++ {
		appendDemandBar(f, i)
	}
	ptr := f.RSXJurikPtr()
	_, _, _, _, _, _, _, rebuilds := f.RSXLiveStats()
	f.SetRSXDemand(nodes.NeedRSXCore | nodes.NeedRSFractal)
	if f.RSXJurikPtr() != ptr {
		t.Fatal("family wake must not replace live RSX node/state")
	}
	_, _, _, _, _, _, _, rebuilds2 := f.RSXLiveStats()
	if rebuilds2 != rebuilds {
		t.Fatalf("second Jurik rebuild on family wake: %d → %d", rebuilds, rebuilds2)
	}
	f.mu.RLock()
	closed := f.closedKlinesTailLocked(dagHistoryCap)
	series := f.closedJurikSeriesLocked(closed)
	got := append([]indicators.IndicatorFactEvent(nil), f.rsxFractalFacts...)
	f.mu.RUnlock()
	want := RSTFractalFactsFromClosedSeries(closed, series, f.effectiveRSXSettings())
	if !factSetsEqual(got, want) {
		t.Fatalf("fractal wake facts %d want replay %d", len(got), len(want))
	}
}

func TestRSXDemand_CombinedWakeOneRSXRebuild(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	klines := demandWaveKlines(140)
	f := NewFrame(klines, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	_, _, _, _, _, _, _, r0 := f.RSXLiveStats()
	if r0 != 0 {
		t.Fatalf("unused rebuilds=%d", r0)
	}
	f.SetRSXDemand(nodes.RSXWorkAll)
	_, _, _, _, _, _, _, r1 := f.RSXLiveStats()
	if r1 != 1 {
		t.Fatalf("combined wake must rebuild RSX once, got %d", r1)
	}
	f.mu.RLock()
	closed := f.closedKlinesTailLocked(dagHistoryCap)
	series := f.closedJurikSeriesLocked(closed)
	tv := append([]indicators.IndicatorFactEvent(nil), f.rsxTVFacts...)
	fr := append([]indicators.IndicatorFactEvent(nil), f.rsxFractalFacts...)
	zz := append([]indicators.IndicatorFactEvent(nil), f.rsxZZFacts...)
	f.mu.RUnlock()
	wantTV := RSTVFactsFromClosedSeries(closed, series, f.effectiveRSXSettings().DivLookback)
	wantFr := RSTFractalFactsFromClosedSeries(closed, series, f.effectiveRSXSettings())
	if !factSetsEqual(tv, wantTV) {
		t.Fatalf("TV wake %d want %d", len(tv), len(wantTV))
	}
	if !factSetsEqual(fr, wantFr) {
		t.Fatalf("Fractal wake %d want %d", len(fr), len(wantFr))
	}
	if len(zz) == 0 {
		t.Fatal("expected ZZ facts on combined wake")
	}
}

func TestRSXDemand_FamilySleepStopsAndWakeRebuilds(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := NewFrame(demandWaveKlines(140), "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	f.SetRSXDemand(nodes.NeedRSXCore | nodes.NeedRSFractal)
	_, _, _, _, fr0, _, rev0, _ := f.RSXLiveStats()
	f.SetRSXDemand(nodes.NeedRSXCore)
	before := f.RSTFractalFactsSnapshot()
	if len(before) == 0 {
		t.Fatal("sleep must keep historical facts")
	}
	_, _, _, _, frMid, _, _, _ := f.RSXLiveStats()
	for i := 140; i < 220; i++ {
		appendDemandBar(f, i)
	}
	_, _, _, _, fr1, _, _, _ := f.RSXLiveStats()
	if fr1 != frMid {
		t.Fatalf("asleep fractal still evaluating %d → %d (before sleep %d)", frMid, fr1, fr0)
	}
	afterSleep := f.RSTFractalFactsSnapshot()
	if len(afterSleep) != len(before) {
		t.Fatal("sleep must not delete facts")
	}
	f.SetRSXDemand(nodes.NeedRSXCore | nodes.NeedRSFractal)
	woken := f.RSTFractalFactsSnapshot()
	if len(woken) <= len(afterSleep) {
		t.Fatalf("wake must rematerialize missed facts: had %d now %d", len(afterSleep), len(woken))
	}
	_, _, _, _, _, _, rev1, _ := f.RSXLiveStats()
	if rev1 <= rev0 {
		t.Fatal("changed factual set must bump revision")
	}
	f.mu.RLock()
	closed := f.closedKlinesTailLocked(dagHistoryCap)
	series := f.closedJurikSeriesLocked(closed)
	f.mu.RUnlock()
	want := RSTFractalFactsFromClosedSeries(closed, series, NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"}))
	if !factSetsEqual(woken, want) {
		t.Fatalf("woken fractal %d want %d", len(woken), len(want))
	}
	seen := map[string]int{}
	for _, ev := range woken {
		k := ev.Source + "|" + ev.Direction + "|" + ev.Pattern + "|" + itoa64(ev.ConfirmedAt) + "|" + itoa64(ev.AnchorAt)
		seen[k]++
		if seen[k] > 1 {
			t.Fatalf("duplicate fact %s", k)
		}
	}
}

func TestRSXDemand_IdempotentWakeNoDupAndNoRevIfUnchanged(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := NewFrame(demandWaveKlines(140), "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	f.SetRSXDemand(nodes.NeedRSXCore | nodes.NeedRSTV)
	n0 := len(f.RSTVFactsSnapshot())
	_, _, _, _, _, _, rev0, _ := f.RSXLiveStats()
	f.SetRSXDemand(nodes.NeedRSXCore)
	f.SetRSXDemand(nodes.NeedRSXCore | nodes.NeedRSTV)
	n1 := len(f.RSTVFactsSnapshot())
	_, _, _, _, _, _, rev1, _ := f.RSXLiveStats()
	if n1 != n0 {
		t.Fatalf("idempotent wake changed count %d → %d", n0, n1)
	}
	if rev1 != rev0 {
		t.Fatalf("unchanged set must not bump revision %d → %d", rev0, rev1)
	}
}

func TestRSXDemand_FamilyReplacePreservesOtherSources(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := NewFrame(demandWaveKlines(140), "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	f.SetRSXDemand(nodes.NeedRSXCore | nodes.NeedRSTV | nodes.NeedRSFractal)
	tv := f.RSTVFactsSnapshot()
	if len(tv) == 0 {
		t.Fatal("need TV facts")
	}
	f.SetRSXDemand(nodes.NeedRSXCore | nodes.NeedRSTV)
	f.SetRSXDemand(nodes.NeedRSXCore | nodes.NeedRSTV | nodes.NeedRSFractal)
	tv2 := f.RSTVFactsSnapshot()
	if !factSetsEqual(tv, tv2) {
		t.Fatal("fractal wake mutated TV facts")
	}
}

func TestRSXDemand_TwoZigZagsUnchanged(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	f := NewFrame(syntheticKlines(40), "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	ptr := f.FrameZigZagPtr()
	if ptr == nil {
		t.Fatal("Frame a.zigzag must exist")
	}
	f.SetRSXDemand(nodes.NeedRSXCore | nodes.NeedRSZZ)
	if f.FrameZigZagPtr() != ptr {
		t.Fatal("DAG ZZ demand must not retarget Frame a.zigzag")
	}
	f.SetRSXDemand(0)
	if f.FrameZigZagPtr() != ptr {
		t.Fatal("sleeping DAG ZZ must not touch Frame a.zigzag")
	}
}

func TestRSXDemand_MeasureActiveCases(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	cases := []struct {
		name string
		mask nodes.RSXWorkMask
	}{
		{"rsx_line_only", nodes.NeedRSXCore},
		{"tv_only", nodes.NeedRSXCore | nodes.NeedRSTV},
		{"fractal_only", nodes.NeedRSXCore | nodes.NeedRSFractal},
		{"zz_only", nodes.NeedRSXCore | nodes.NeedRSZZ},
		{"all_families", nodes.RSXWorkAll},
	}
	klines := demandWaveKlines(80)
	for _, tc := range cases {
		f := NewFrame(klines, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
		f.SetRSXDemand(tc.mask)
		_, rsx0, zz0, tv0, fr0, zzE0, _, _ := f.RSXLiveStats()
		for i := 80; i < 90; i++ {
			appendDemandBar(f, i)
		}
		_, rsx1, zz1, tv1, fr1, zzE1, _, _ := f.RSXLiveStats()
		t.Logf("%s incremental: RSX %d→%d ZZ %d→%d TV %d→%d Fractal %d→%d ZZCol %d→%d",
			tc.name, rsx0, rsx1, zz0, zz1, tv0, tv1, fr0, fr1, zzE0, zzE1)
		if tc.mask&nodes.NeedRSXCore != 0 && rsx1 <= rsx0 {
			t.Fatalf("%s Core did not Update", tc.name)
		}
		if tc.mask&nodes.NeedRSTV == 0 && tv1 != tv0 {
			t.Fatalf("%s TV should stay asleep", tc.name)
		}
		if tc.mask&nodes.NeedRSFractal == 0 && fr1 != fr0 {
			t.Fatalf("%s Fractal should stay asleep", tc.name)
		}
		if tc.mask&nodes.NeedRSZZ == 0 && (zz1 != zz0 || zzE1 != zzE0) {
			t.Fatalf("%s DAG ZZ should stay asleep", tc.name)
		}
	}
}

func TestRSXDemand_ConfirmedAtUnchanged(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	klines := demandWaveKlines(140)
	always := NewFrame(klines, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	always.SetRSXDemand(nodes.NeedRSXCore | nodes.NeedRSTV)
	want := always.RSTVFactsSnapshot()
	lazy := NewFrame(klines[:80], "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	lazy.SetRSXDemand(nodes.NeedRSXCore)
	for i := 80; i < 140; i++ {
		lazy.UpdateKlineTick(klines[i], true)
	}
	lazy.SetRSXDemand(nodes.NeedRSXCore | nodes.NeedRSTV)
	got := lazy.RSTVFactsSnapshot()
	if !factSetsEqual(got, want) {
		t.Fatalf("wake ConfirmedAt/tuples drifted: got %d want %d", len(got), len(want))
	}
}
