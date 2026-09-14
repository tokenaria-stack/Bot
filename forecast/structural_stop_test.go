package forecast

import (
	"math"
	"strings"
	"testing"

	"trading_bot/data"
	"trading_bot/indicators"
)

func TestStructuralStop_IgnitionAndNoFuturePivot(t *testing.T) {
	t.Parallel()
	step, _ := data.IntervalDurationMs("15m")
	step1h, _ := data.IntervalDurationMs("1h")
	base := int64(1_609_459_200_000)
	const n = 90
	primary := make([]CanonicalClosedBar, n)
	for i := 0; i < n; i++ {
		px := 100.0
		primary[i] = CanonicalClosedBar{
			OpenTime: base + int64(i)*step,
			Open:     px, High: px + 2, Low: px - 3, Close: px, Volume: 1,
		}
	}
	h1n := (n + 3) / 4
	h1 := make([]CanonicalClosedBar, h1n)
	for i := 0; i < h1n; i++ {
		h1[i] = CanonicalClosedBar{
			OpenTime: base + int64(i)*step1h,
			Open:     100, High: 102, Low: 97, Close: 100, Volume: 1,
		}
	}
	cand := 20
	var tape []FeatureRow2
	for i := 18; i <= 22; i++ {
		var v FeatureVector2
		if i == cand {
			v[spec2Col(FeatureRSX50CrossUpPresent)] = 1
			v[spec2Col(FeatureRSX50CrossUpAge)] = 0
		}
		if i == cand+1 {
			v[spec2Col(FeatureRSX50CrossUpPresent)] = 1
			v[spec2Col(FeatureRSX50CrossUpAge)] = 1
		}
		tape = append(tape, FeatureRow2{At: primary[i].OpenTime, Ready: IsReady, Values: v})
	}
	future := indicators.IndicatorFactEvent{
		Source: indicators.FactSourceRSXFractalPivot, Direction: indicators.FactDirPivotLow,
		AnchorAt: primary[cand-2].OpenTime, ConfirmedAt: primary[cand+1].OpenTime,
	}
	old := indicators.IndicatorFactEvent{
		Source: indicators.FactSourceRSXFractalPivot, Direction: indicators.FactDirPivotLow,
		AnchorAt: primary[10].OpenTime, ConfirmedAt: primary[12].OpenTime,
		AnchorPrice: 50, // hlc3 trap — must not become the stop
	}
	wrongSide := indicators.IndicatorFactEvent{
		Source: indicators.FactSourceRSXFractalPivot, Direction: indicators.FactDirPivotHigh,
		AnchorAt: primary[11].OpenTime, ConfirmedAt: primary[13].OpenTime,
	}
	fm := testTapeHeader().Market
	fm.Timeframe = "1m"
	rep, err := RunStructuralStop1(StructuralStopInput{
		PrimaryTF: "15m", HTF1hTF: "1h", FinerMarket: fm,
		Primary: primary, HTF1h: h1, Tape: tape,
		Fractals: []indicators.IndicatorFactEvent{future, old, wrongSide},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got *StructuralStopAssignment
	for i := range rep.Assignments {
		if rep.Assignments[i].At == primary[cand].OpenTime && rep.Assignments[i].Side == GeomSideLong {
			got = &rep.Assignments[i]
		}
	}
	if got == nil {
		t.Fatal("missing LONG ignition")
	}
	if got.Status != StructuralStopStatusValid {
		t.Fatalf("status %s", got.Status)
	}
	if got.PivotAnchorAt != primary[10].OpenTime || got.PivotConfirmedAt != primary[12].OpenTime {
		t.Fatalf("used wrong/unconfirmed pivot %+v", got)
	}
	wantStop := primary[10].Low
	if got.Stop != wantStop {
		t.Fatalf("stop=%g want bar Low %g (not AnchorPrice)", got.Stop, wantStop)
	}
	if got.R != math.Abs(got.Entry-wantStop) {
		t.Fatalf("R not from structural stop")
	}
	if got.Plus2R != got.Entry+2*got.R || got.Plus1R != got.Entry+got.R || got.Plus3R != got.Entry+3*got.R {
		t.Fatal("R ladder")
	}
	if !strings.Contains(rep.Text, "NO TARGET SELECTED") {
		t.Fatal("must not freeze a target")
	}
}

func TestStructuralStop_NoStructureAndInvalid(t *testing.T) {
	t.Parallel()
	step, _ := data.IntervalDurationMs("15m")
	step1h, _ := data.IntervalDurationMs("1h")
	base := int64(1_609_459_200_000)
	const n = 90
	primary := make([]CanonicalClosedBar, n)
	for i := 0; i < n; i++ {
		primary[i] = CanonicalClosedBar{
			OpenTime: base + int64(i)*step,
			Open:     100, High: 101, Low: 99, Close: 100, Volume: 1,
		}
	}
	h1n := (n + 3) / 4
	h1 := make([]CanonicalClosedBar, h1n)
	for i := 0; i < h1n; i++ {
		h1[i] = CanonicalClosedBar{
			OpenTime: base + int64(i)*step1h,
			Open:     100, High: 101, Low: 99, Close: 100, Volume: 1,
		}
	}
	var tape []FeatureRow2
	add := func(i int, up bool) {
		var v FeatureVector2
		if up {
			v[spec2Col(FeatureRSX50CrossUpPresent)] = 1
			v[spec2Col(FeatureRSX50CrossUpAge)] = 0
		} else {
			v[spec2Col(FeatureRSX50CrossDownPresent)] = 1
			v[spec2Col(FeatureRSX50CrossDownAge)] = 0
		}
		tape = append(tape, FeatureRow2{At: primary[i].OpenTime, Ready: IsReady, Values: v})
	}
	add(20, true)
	add(40, false)
	primary[5].Low = 100.5 // LONG stop above entry
	fm := testTapeHeader().Market
	fm.Timeframe = "1m"
	rep, err := RunStructuralStop1(StructuralStopInput{
		Primary: primary, HTF1h: h1, Tape: tape, FinerMarket: fm,
		Fractals: []indicators.IndicatorFactEvent{{
			Source: indicators.FactSourceRSXFractalPivot, Direction: indicators.FactDirPivotLow,
			AnchorAt: primary[5].OpenTime, ConfirmedAt: primary[8].OpenTime,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Assignments) != 2 {
		t.Fatalf("n=%d", len(rep.Assignments))
	}
	if rep.Assignments[0].Status != StructuralStopStatusInvalid {
		t.Fatalf("want INVALID got %s", rep.Assignments[0].Status)
	}
	if rep.Assignments[0].Plus2R != 0 {
		t.Fatal("INVALID must not grow R targets")
	}
	if rep.Assignments[1].Status != StructuralStopStatusNone {
		t.Fatalf("SHORT without high pivot want NO_STRUCTURE got %s", rep.Assignments[1].Status)
	}
	if rep.Assignments[1].Stop != 0 {
		t.Fatal("NO_STRUCTURE must not invent a stop")
	}
}

func TestStructuralStop_MFEBeforeVsFull(t *testing.T) {
	t.Parallel()
	step, _ := data.IntervalDurationMs("15m")
	step1h, _ := data.IntervalDurationMs("1h")
	base := int64(1_609_459_200_000)
	const n = 90
	primary := make([]CanonicalClosedBar, n)
	for i := 0; i < n; i++ {
		primary[i] = CanonicalClosedBar{
			OpenTime: base + int64(i)*step,
			Open:     100, High: 100.2, Low: 99.8, Close: 100, Volume: 1,
		}
	}
	cand := 10
	primary[6].Low = 96
	primary[cand+2].Low = 95.9 // stop
	primary[cand+2].High = 100.1
	primary[cand+20].High = 120 // move after stop
	h1n := (n + 3) / 4
	h1 := make([]CanonicalClosedBar, h1n)
	for i := 0; i < h1n; i++ {
		h1[i] = CanonicalClosedBar{
			OpenTime: base + int64(i)*step1h,
			Open:     100, High: 102, Low: 96, Close: 100, Volume: 1,
		}
	}
	var v FeatureVector2
	v[spec2Col(FeatureRSX50CrossUpPresent)] = 1
	v[spec2Col(FeatureRSX50CrossUpAge)] = 0
	tape := []FeatureRow2{{At: primary[cand].OpenTime, Ready: IsReady, Values: v}}
	fm := testTapeHeader().Market
	fm.Timeframe = "1m"
	rep, err := RunStructuralStop1(StructuralStopInput{
		Primary: primary, HTF1h: h1, Tape: tape, FinerMarket: fm,
		Fractals: []indicators.IndicatorFactEvent{{
			Source: indicators.FactSourceRSXFractalPivot, Direction: indicators.FactDirPivotLow,
			AnchorAt: primary[6].OpenTime, ConfirmedAt: primary[8].OpenTime,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rep.Assignments[0]
	if got.Status != StructuralStopStatusValid || !got.StopHit {
		t.Fatalf("%+v", got)
	}
	if got.MFEFullHOverR <= got.MFEBeforeStopOverR+0.5 {
		t.Fatalf("full-H MFE should include post-stop rally: before=%g full=%g", got.MFEBeforeStopOverR, got.MFEFullHOverR)
	}
}

func TestStructuralStop_ShortMirrorsLong(t *testing.T) {
	t.Parallel()
	step, _ := data.IntervalDurationMs("15m")
	step1h, _ := data.IntervalDurationMs("1h")
	base := int64(1_609_459_200_000)
	const n = 90
	primary := make([]CanonicalClosedBar, n)
	for i := 0; i < n; i++ {
		primary[i] = CanonicalClosedBar{
			OpenTime: base + int64(i)*step,
			Open:     100, High: 100.5, Low: 99.5, Close: 100, Volume: 1,
		}
	}
	primary[8].High = 104
	h1n := (n + 3) / 4
	h1 := make([]CanonicalClosedBar, h1n)
	for i := 0; i < h1n; i++ {
		h1[i] = CanonicalClosedBar{
			OpenTime: base + int64(i)*step1h,
			Open:     100, High: 104, Low: 96, Close: 100, Volume: 1,
		}
	}
	var v FeatureVector2
	v[spec2Col(FeatureRSX50CrossDownPresent)] = 1
	v[spec2Col(FeatureRSX50CrossDownAge)] = 0
	tape := []FeatureRow2{{At: primary[20].OpenTime, Ready: IsReady, Values: v}}
	fm := testTapeHeader().Market
	fm.Timeframe = "1m"
	rep, err := RunStructuralStop1(StructuralStopInput{
		Primary: primary, HTF1h: h1, Tape: tape, FinerMarket: fm,
		Fractals: []indicators.IndicatorFactEvent{{
			Source: indicators.FactSourceRSXFractalPivot, Direction: indicators.FactDirPivotHigh,
			AnchorAt: primary[8].OpenTime, ConfirmedAt: primary[10].OpenTime,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rep.Assignments[0]
	if got.Status != StructuralStopStatusValid {
		t.Fatalf("status %s", got.Status)
	}
	if got.Stop != 104 {
		t.Fatalf("short stop must be AnchorAt High, got %g", got.Stop)
	}
	if got.R != 4 || got.Plus2R != 92 || got.Plus1R != 96 || got.Plus3R != 88 {
		t.Fatalf("short R ladder %+v", got)
	}
	if got.Plus2ATR == got.Plus2R {
		t.Fatal("+2 ATR must be independent of +2R when ATR ≠ R")
	}
}

func TestStructuralStop_PriceK2S0NoLookahead(t *testing.T) {
	t.Parallel()
	step, _ := data.IntervalDurationMs("15m")
	step1h, _ := data.IntervalDurationMs("1h")
	base := int64(1_609_459_200_000)
	const n = 90
	primary := make([]CanonicalClosedBar, n)
	for i := 0; i < n; i++ {
		primary[i] = CanonicalClosedBar{
			OpenTime: base + int64(i)*step,
			Open:     100, High: 102, Low: 99, Close: 100, Volume: 1,
		}
	}
	primary[30].Low = 90
	h1n := (n + 3) / 4
	h1 := make([]CanonicalClosedBar, h1n)
	for i := 0; i < h1n; i++ {
		h1[i] = CanonicalClosedBar{
			OpenTime: base + int64(i)*step1h,
			Open:     100, High: 102, Low: 90, Close: 100, Volume: 1,
		}
	}
	var v FeatureVector2
	v[spec2Col(FeatureRSX50CrossUpPresent)] = 1
	v[spec2Col(FeatureRSX50CrossUpAge)] = 0
	tape := []FeatureRow2{{At: primary[31].OpenTime, Ready: IsReady, Values: v}}
	fm := testTapeHeader().Market
	fm.Timeframe = "1m"
	in := StructuralStopInput{
		Primary: primary, HTF1h: h1, Tape: tape, FinerMarket: fm,
		StopOwner: StopOwnerPriceK2,
	}
	rep, err := RunStructuralStop1(in)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Assignments[0].Status != StructuralStopStatusNone {
		t.Fatalf("k=2 must wait two right bars, got %s", rep.Assignments[0].Status)
	}
	tape[0].At = primary[32].OpenTime
	rep, err = RunStructuralStop1(in)
	if err != nil {
		t.Fatal(err)
	}
	got := rep.Assignments[0]
	if got.Status != StructuralStopStatusValid || got.Wick != 90 || got.PivotAnchorAt != primary[30].OpenTime {
		t.Fatalf("S0 k=2 after confirm: %+v", got)
	}
	if got.BufferATR != StructuralStopBufferATR15 || !(got.Stop < got.Wick) {
		t.Fatalf("LONG buffer must sit beyond wick: %+v", got)
	}
	if math.Abs(got.R-math.Abs(got.Entry-got.Stop)) > 1e-9 {
		t.Fatalf("R must use buffered stop")
	}
	if !strings.Contains(rep.Text, "0.15") {
		t.Fatal(rep.Text)
	}
}
