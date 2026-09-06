package market

import (
	"math"
	"testing"

	"trading_bot/core/nodes"
	"trading_bot/exchange"
	"trading_bot/forecast"
	"trading_bot/indicators"
)

func tape1ASettings() RSXSettings {
	return NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivLookback: 30})
}

func tape1APlan(t *testing.T, s RSXSettings) forecast.FeaturePlan {
	t.Helper()
	analysis, err := AnalysisRecipeFromRSXSettings(s, true, false, analysisLogicV2)
	if err != nil {
		t.Fatal(err)
	}
	features, err := forecast.ResolveFeatureRecipe("tape1a", forecast.FeatureRecipeDraft{
		Features: []forecast.FeatureID{
			forecast.FeatureRSXValue,
			forecast.FeatureRSXSignal,
			forecast.FeatureTVBullPresent,
			forecast.FeatureTVBullAge,
		},
	}, "features:v1")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := forecast.BindFeaturePlan(analysis, features, "plan:v1")
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func tape1ATVKlines(n int) []exchange.Kline {
	klines := make([]exchange.Kline, n)
	base := int64(1_700_000_000_000)
	for i := range klines {
		wave := float64(i%40) - 20
		px := 50000 - wave*8 - float64(i)
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

type featureObs struct {
	At    int64
	Ready forecast.Ready
	Vals  [4]float64
}

func observeFill(t *testing.T, ev *FeatureEvaluator) featureObs {
	t.Helper()
	ready, dst, err := ev.FillOwned()
	if err != nil {
		t.Fatal(err)
	}
	ev.frame.mu.RLock()
	var at int64
	if n := len(ev.frame.klines); n > 0 {
		at = ev.frame.klines[n-1].OpenTime
	}
	ev.frame.mu.RUnlock()
	o := featureObs{At: at, Ready: ready}
	if ready {
		copy(o.Vals[:], dst)
	}
	return o
}

func TestFeatureDemand_PersistentUnion(t *testing.T) {
	f := NewFrame(syntheticKlines(80), "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	plan := tape1APlan(t, f.EffectiveRSXSettings())
	forecastBits := RSXWorkFromFeaturePlan(plan)
	if forecastBits&nodes.NeedRSTV == 0 || forecastBits&nodes.NeedRSXCore == 0 {
		t.Fatalf("expected Core|TV forecast bits, got %v", forecastBits)
	}

	f.SetRSXDemand(0)
	f.SetRSXForecastDemand(forecastBits)
	mask, _, _, _, _, _, _, _ := f.RSXLiveStats()
	if mask&nodes.NeedRSXCore == 0 || mask&nodes.NeedRSTV == 0 {
		t.Fatalf("A: chart off + forecast TV must keep Core|TV, mask=%v", mask)
	}

	f.SetRSXForecastDemand(0)
	f.SetRSXDemand(nodes.NeedRSXCore)
	mask, _, _, _, _, _, _, _ = f.RSXLiveStats()
	if mask&nodes.NeedRSXCore == 0 {
		t.Fatalf("B: forecast removed + chart RSX must keep Core, mask=%v", mask)
	}
	if mask&nodes.NeedRSTV != 0 && rsxInternalMask()&nodes.NeedRSTV == 0 {
		t.Fatalf("B: TV must not remain from stale forecast, mask=%v", mask)
	}

	f.SetRSXDemand(0)
	f.SetRSXForecastDemand(nodes.NeedRSXCore)
	f.SetRSXForecastDemand(forecastBits)
	mask, _, _, _, _, _, _, _ = f.RSXLiveStats()
	if mask&nodes.NeedRSTV == 0 {
		t.Fatalf("C: replacing forecast mask must install TV, mask=%v", mask)
	}
	f.SetRSXDemand(nodes.NeedRSXCore)
	mask2, _, _, _, _, _, _, _ := f.RSXLiveStats()
	if mask2&nodes.NeedRSTV == 0 {
		t.Fatalf("client update must not clobber forecast TV, mask=%v", mask2)
	}
}

func TestBindFeatureEvaluator_RefusesAnalysisMismatch(t *testing.T) {
	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(RSXSettings{Length: 21, SignalLength: 9, Source: "hlc3", DivLookback: 30})
	plan := tape1APlan(t, tape1ASettings())
	_, err := BindFeatureEvaluator(frame, plan)
	if err == nil {
		t.Fatal("expected refuse bind on RSX length mismatch")
	}
}

func TestFeatureFill_ReadySeamAndAbsentTV(t *testing.T) {
	settings := tape1ASettings()
	plan := tape1APlan(t, settings)
	klines := syntheticRSXKlines(40)
	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	ev, err := BindFeatureEvaluator(frame, plan)
	if err != nil {
		t.Fatal(err)
	}
	var firstReady int64
	sawNotReady := false
	sawReadyNoTV := false
	for _, k := range klines {
		frame.UpdateKlineTick(k, true)
		o := observeFill(t, ev)
		if !o.Ready {
			sawNotReady = true
			continue
		}
		if firstReady == 0 {
			firstReady = o.At
		}
		if o.Vals[2] == 0 && o.Vals[3] == 0 {
			sawReadyNoTV = true
		}
		if err := forecast.ValidateFeatureVector(o.Vals[:]); err != nil {
			t.Fatal(err)
		}
	}
	if !sawNotReady {
		// Empty Frame before any bar is the NotReady seam; Jurik may be finite on bar 0.
		frame0 := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
		frame0.ApplyBacktestRSXConfig(settings)
		ev0, err := BindFeatureEvaluator(frame0, plan)
		if err != nil {
			t.Fatal(err)
		}
		r0, _, err := ev0.FillOwned()
		if err != nil {
			t.Fatal(err)
		}
		if r0 {
			t.Fatal("expected NotReady with no closed bars")
		}
	}
	if firstReady == 0 {
		t.Fatal("expected a first Ready bar")
	}
	if !sawReadyNoTV {
		t.Fatal("Ready with no bullish TV must yield presence=0")
	}
}

func TestFeatureFill_ConfirmedAtNotAnchorAt(t *testing.T) {
	settings := tape1ASettings()
	plan := tape1APlan(t, settings)
	klines := tape1ATVKlines(120)
	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	if _, err := BindFeatureEvaluator(frame, plan); err != nil {
		t.Fatal(err)
	}
	for _, k := range klines {
		frame.UpdateKlineTick(k, true)
	}
	var bull indicators.IndicatorFactEvent
	found := false
	for _, evf := range frame.RSTVFactsSnapshot() {
		if evf.Source != indicators.FactSourceRSXTVDiv || evf.Direction != indicators.FactDirBullish {
			continue
		}
		if !found || evf.ConfirmedAt < bull.ConfirmedAt {
			bull = evf
			found = true
		}
	}
	if !found {
		mask, _, _, tvEvals, _, _, _, _ := frame.RSXLiveStats()
		t.Fatalf("fixture produced no bullish TV divergence mask=%v tvEvals=%d facts=%d", mask, tvEvals, len(frame.RSTVFactsSnapshot()))
	}
	if bull.ConfirmedAt <= bull.AnchorAt {
		t.Fatalf("expected ConfirmedAt > AnchorAt: %+v", bull)
	}

	live := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	live.ApplyBacktestRSXConfig(settings)
	ev2, err := BindFeatureEvaluator(live, plan)
	if err != nil {
		t.Fatal(err)
	}
	var before, atC, after featureObs
	for _, k := range klines {
		live.UpdateKlineTick(k, true)
		o := observeFill(t, ev2)
		if o.Ready && k.OpenTime < bull.ConfirmedAt && o.Vals[2] != 0 {
			t.Fatalf("lookahead: present=1 at At=%d before ConfirmedAt=%d", k.OpenTime, bull.ConfirmedAt)
		}
		switch k.OpenTime {
		case bull.AnchorAt:
			before = o
		case bull.ConfirmedAt:
			atC = o
		case bull.ConfirmedAt + 60_000:
			after = o
		}
	}
	_ = before
	if !atC.Ready || atC.Vals[2] != 1 || atC.Vals[3] != 0 {
		t.Fatalf("confirm bar present/age=%v/%v", atC.Vals[2], atC.Vals[3])
	}
	if !after.Ready || after.Vals[2] != 1 {
		t.Fatalf("bar after confirm should remain present, got ready=%v present=%v", after.Ready, after.Vals[2])
	}
	if after.Vals[3] != 1 && after.Vals[3] != 0 {
		t.Fatalf("age should be 1, or 0 if a new fact confirmed, got %v", after.Vals[3])
	}
}

func TestFeatureFill_ReplayLiveParity(t *testing.T) {
	settings := tape1ASettings()
	plan := tape1APlan(t, settings)
	klines := tape1ATVKlines(80)

	live := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	live.ApplyBacktestRSXConfig(settings)
	liveEv, err := BindFeatureEvaluator(live, plan)
	if err != nil {
		t.Fatal(err)
	}
	var liveObs []featureObs
	for _, k := range klines {
		live.UpdateKlineTick(k, true)
		liveObs = append(liveObs, observeFill(t, liveEv))
	}

	var replayObs []featureObs
	for i := range klines {
		rf := NewFrame(klines[:i+1], "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
		rf.ApplyBacktestRSXConfig(settings)
		rev, err := BindFeatureEvaluator(rf, plan)
		if err != nil {
			t.Fatal(err)
		}
		replayObs = append(replayObs, observeFill(t, rev))
	}

	if len(liveObs) != len(replayObs) {
		t.Fatalf("obs len live=%d replay=%d", len(liveObs), len(replayObs))
	}
	var firstLive, firstReplay int64
	for i := range liveObs {
		a, b := liveObs[i], replayObs[i]
		if a.At != b.At {
			t.Fatalf("At mismatch i=%d live=%d replay=%d", i, a.At, b.At)
		}
		if a.Ready != b.Ready {
			t.Fatalf("Ready mismatch At=%d live=%v replay=%v", a.At, a.Ready, b.Ready)
		}
		if a.Ready && firstLive == 0 {
			firstLive = a.At
		}
		if b.Ready && firstReplay == 0 {
			firstReplay = b.At
		}
		if !a.Ready {
			continue
		}
		for col := 0; col < 4; col++ {
			if math.Float64bits(a.Vals[col]) != math.Float64bits(b.Vals[col]) {
				t.Fatalf("bits At=%d col=%d live=%v/%#x replay=%v/%#x",
					a.At, col, a.Vals[col], math.Float64bits(a.Vals[col]), b.Vals[col], math.Float64bits(b.Vals[col]))
			}
		}
	}
	if firstLive == 0 || firstLive != firstReplay {
		t.Fatalf("first Ready At live=%d replay=%d", firstLive, firstReplay)
	}
}

func TestFeatureFill_AllocsPerRun(t *testing.T) {
	settings := tape1ASettings()
	plan := tape1APlan(t, settings)
	klines := tape1ATVKlines(80)
	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	ev, err := BindFeatureEvaluator(frame, plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range klines {
		frame.UpdateKlineTick(k, true)
	}
	ready, _, err := ev.FillOwned()
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Fatal("expected Ready after warmup fixture")
	}
	allocs := testing.AllocsPerRun(1000, func() {
		_, _ = ev.Fill(ev.dst)
	})
	if allocs != 0 {
		t.Fatalf("steady-state Fill allocated %.2f/op (want 0 from feature plumbing)", allocs)
	}
}

func TestAnalysisRecipeFromRSXSettings_UsesActualLookback(t *testing.T) {
	a, err := AnalysisRecipeFromRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivLookback: 30}, true, false, analysisLogicV2)
	if err != nil {
		t.Fatal(err)
	}
	b, err := AnalysisRecipeFromRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivLookback: 90}, true, false, analysisLogicV2)
	if err != nil {
		t.Fatal(err)
	}
	ida, _ := a.Identity()
	idb, _ := b.Identity()
	if ida.Digest == idb.Digest {
		t.Fatal("DivLookback must participate in AnalysisRecipe identity")
	}
}
