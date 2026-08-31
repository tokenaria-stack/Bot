package market

import (
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

func zzOscillatingKlines(n int) []exchange.Kline {
	klines := make([]exchange.Kline, n)
	base := int64(1_700_000_000_000)
	for i := range klines {
		px := 50000 + 140*math.Sin(float64(i)/7.0)
		klines[i] = exchange.Kline{
			OpenTime:  base + int64(i)*60_000,
			CloseTime: base + int64(i)*60_000 + 59_999,
			Open:      px,
			High:      px + 55,
			Low:       px - 55,
			Close:     px + 10*math.Sin(float64(i)/3.0),
			Volume:    100,
		}
	}
	return klines
}

func zzTestKlines(n int) []exchange.Kline {
	klines := make([]exchange.Kline, n)
	base := int64(1_700_000_000_000)
	for i := range klines {
		wave := float64(i%20) - 10
		px := 50000 + wave*25 + float64(i)*2
		klines[i] = exchange.Kline{
			OpenTime:  base + int64(i)*60_000,
			CloseTime: base + int64(i)*60_000 + 59_999,
			Open:      px,
			High:      px + 40,
			Low:       px - 40,
			Close:     px + 8,
			Volume:    100,
		}
	}
	return klines
}

func TestZZDivFacts_ReplayMatchesLiveClosedWalk(t *testing.T) {
	klines := zzTestKlines(160)
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivLookback: 30}
	fromReplay := ReplayClosedBars(klines, settings).ZZFacts

	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	for _, k := range klines {
		frame.UpdateKlineTick(k, true)
	}
	fromLive := frame.RSTZZFactsSnapshot()
	if len(fromReplay) != len(fromLive) {
		t.Fatalf("replay %d live %d", len(fromReplay), len(fromLive))
	}
	for i := range fromReplay {
		if fromReplay[i] != fromLive[i] {
			t.Fatalf("i=%d replay=%+v live=%+v", i, fromReplay[i], fromLive[i])
		}
	}
	if len(fromReplay) == 0 {
		t.Fatal("expected at least one ZigZag divergence fact on this series")
	}
	for _, ev := range fromReplay {
		if ev.Source != indicators.FactSourceRSXZZDiv {
			t.Fatalf("source %q", ev.Source)
		}
		if ev.Pattern != indicators.FactPatternRegular && ev.Pattern != indicators.FactPatternHidden {
			t.Fatalf("pattern %q", ev.Pattern)
		}
		if ev.Direction != indicators.FactDirBullish && ev.Direction != indicators.FactDirBearish {
			t.Fatalf("direction %q", ev.Direction)
		}
		if ev.ConfirmedAt <= ev.AnchorAt {
			t.Fatalf("ConfirmedAt must be after AnchorAt: %+v", ev)
		}
	}
}

func TestZZDivFacts_NoNewSwingNoNewFacts(t *testing.T) {
	klines := zzTestKlines(120)
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"}
	cap := core.ValidateHistoryCap(len(klines))
	runner := newDAGRunner(cap, settings)
	var col ZZDivFactCollector
	var facts []indicators.IndicatorFactEvent
	for i, k := range klines {
		runner.TickUpdate(k.Open, k.High, k.Low, k.Close, k.Volume, i, true)
		if ev, ok := col.ObserveClosed(runner.Bus(), klines, i); ok {
			facts = append(facts, ev)
		}
	}
	n := len(facts)
	calls := col.ClassifyCalls()
	if n == 0 {
		t.Fatal("need a producing series before idle walk")
	}
	for i := 0; i < 20; i++ {
		if _, ok := col.ObserveClosed(runner.Bus(), klines, len(klines)-1); ok {
			t.Fatalf("idle re-scan emitted fact after %d (had %d)", i, n)
		}
	}
	if col.ClassifyCalls() != calls {
		t.Fatalf("idle bars classified %d → %d", calls, col.ClassifyCalls())
	}
	if len(facts) != n {
		t.Fatal("fact list grew without a new swing")
	}
}

func TestZZDivFacts_RSXFromSwingBarNotConfirmBar(t *testing.T) {
	klines := zzTestKlines(80)
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"}
	cap := core.ValidateHistoryCap(len(klines))
	runner := newDAGRunner(cap, settings)
	var col ZZDivFactCollector
	checked := 0
	for i, k := range klines {
		runner.TickUpdate(k.Open, k.High, k.Low, k.Close, k.Volume, i, true)
		ev, ok := col.ObserveClosed(runner.Bus(), klines, i)
		if !ok {
			continue
		}
		hist := runner.Bus().Hist
		swings := runner.Bus().Events.GetLast(8)
		if len(swings) < 1 {
			t.Fatal("missing swing")
		}
		latest := swings[0]
		atAnchor := histRSXFromNewest(hist, i, latest.BarIndex)
		if ev.AnchorValue != atAnchor {
			t.Fatalf("AnchorValue=%v swingBar RSX=%v", ev.AnchorValue, atAnchor)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("need at least one ZZ fact to prove RSX-at-anchor")
	}
}

func TestZZDivFacts_PublishedIndependently(t *testing.T) {
	klines := zzTestKlines(80)
	got := ReplayClosedBars(klines, RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", PivotRadius: 2}).ZZFacts
	if len(got) == 0 {
		t.Fatal("ZZ facts must publish independently of FE visibility")
	}
}

func TestValueAtBar_AlignedWithHistorySlotSeries(t *testing.T) {
	klines := zzTestKlines(40)
	hist := ReplayDAGKlines(klines, RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	series := historySlotSeries(hist, core.SlotJurikRSX)
	for i := range series {
		got := hist.ValueAtBar(core.SlotJurikRSX, i)
		if got != series[i] {
			t.Fatalf("bar %d ValueAtBar=%v series=%v", i, got, series[i])
		}
	}
}

func TestZZDivFacts_OverCapParity(t *testing.T) {
	const wrapCap = 64
	n := wrapCap + 220
	klines := zzOscillatingKlines(n)
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"}
	ref := ReplayClosedBars(klines, settings).ZZFacts
	wrapped := replayClosedBarsCap(klines, settings, wrapCap, nodes.WozduhMaskAll).ZZFacts
	if len(ref) == 0 {
		t.Fatal("reference walk produced no ZZ facts")
	}
	if len(ref) != len(wrapped) {
		t.Fatalf("uncapped %d wrapped %d", len(ref), len(wrapped))
	}
	wrapOpen := klines[wrapCap].OpenTime
	afterWrap := 0
	for i := range ref {
		if ref[i] != wrapped[i] {
			t.Fatalf("i=%d ref=%+v wrapped=%+v", i, ref[i], wrapped[i])
		}
		if wrapped[i].ConfirmedAt >= wrapOpen {
			afterWrap++
		}
	}
	if afterWrap == 0 {
		t.Fatalf("need at least one ZZ fact confirmed after wrap; last=%+v wrapOpen=%d nFacts=%d", ref[len(ref)-1], wrapOpen, len(ref))
	}

	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	long := zzOscillatingKlines(dagHistoryCap + 80)
	for _, k := range long {
		frame.UpdateKlineTick(k, true)
	}
	live := frame.RSTZZFactsSnapshot()
	refLive := ReplayClosedBars(long, settings).ZZFacts
	if len(live) != len(refLive) {
		t.Fatalf("live %d ref %d", len(live), len(refLive))
	}
	for i := range refLive {
		if live[i] != refLive[i] {
			t.Fatalf("live i=%d %+v ref %+v", i, live[i], refLive[i])
		}
	}
}

func TestZZHistory_SingleDAGWalk(t *testing.T) {
	klines := zzTestKlines(80)
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"}
	before := dagRunnerBorn()
	replay := ReplayClosedBars(klines, settings)
	if dagRunnerBorn()-before != 1 {
		t.Fatalf("ReplayClosedBars must allocate exactly one DAG runner, grew %d", dagRunnerBorn()-before)
	}
	if replay.Hist == nil || len(replay.ZZFacts) == 0 {
		t.Fatal("one walk must yield hist and ZZ facts")
	}
}
