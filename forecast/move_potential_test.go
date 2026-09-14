package forecast

import (
	"math"
	"strings"
	"testing"

	"trading_bot/data"
	"trading_bot/indicators"
)

func TestFormatMovePotential1_BucketsContinuationCDF(t *testing.T) {
	t.Parallel()
	rep := StructuralStopReport{
		Assignments: []StructuralStopAssignment{
			{Status: StructuralStopStatusValid, Side: GeomSideLong, Hit1RBeforeStop: true, Hit2RBeforeStop: true, TimeTo1R: 8, TimeTo2R: 20, TimeToMFEBefore: 10, TimeToMFEFullH: 10, MFEBeforeStopOverR: 2, MFEFullHOverR: 2.2},
			{Status: StructuralStopStatusValid, Side: GeomSideLong, Hit1RBeforeStop: true, TimeTo1R: 12, TimeToMFEBefore: 20, TimeToMFEFullH: 40, MFEBeforeStopOverR: 1, MFEFullHOverR: 1.5},
			{Status: StructuralStopStatusValid, Side: GeomSideLong, StopHit: true, TimeToMFEBefore: 40, TimeToMFEFullH: 70, MFEBeforeStopOverR: 0.2, MFEFullHOverR: 3},
			{Status: StructuralStopStatusValid, Side: GeomSideLong, TimeToMFEBefore: 70, TimeToMFEFullH: 70, MFEBeforeStopOverR: 0.4, MFEFullHOverR: 0.4},
			{Status: StructuralStopStatusValid, Side: GeomSideLong, IncompleteH: true, Hit2RBeforeStop: true},
			{Status: StructuralStopStatusValid, Side: GeomSideShort, Hit1RBeforeStop: true, TimeTo1R: 5, TimeToMFEBefore: 5, TimeToMFEFullH: 5, MFEBeforeStopOverR: 1, MFEFullHOverR: 1},
		},
	}
	txt := FormatMovePotential1(rep)
	if strings.Contains(txt, "TARGET SELECTED") && !strings.Contains(txt, "NO TARGET") {
		t.Fatal(txt)
	}
	if !strings.Contains(txt, "P(2R|1R)=0.500") {
		t.Fatal(txt)
	}
	if !strings.Contains(txt, "LONG") || !strings.Contains(txt, "SHORT") {
		t.Fatal("sides must stay separate")
	}
	if !strings.Contains(txt, "NOT_EVALUABLE=1") {
		t.Fatal(txt)
	}
	if cdfAtMost([]float64{10, 20, 40, 70}, 12) != 0.25 || cdfAtMost([]float64{10, 20, 40, 70}, 72) != 1 {
		t.Fatal("CDF knots")
	}
	a := FormatMovePotential1(rep)
	b := FormatMovePotential1(rep)
	if a != b {
		t.Fatal("determinism")
	}
}

func TestMFEBeforeStop_ExcludesStopBarHighWithoutFiner(t *testing.T) {
	t.Parallel()
	got := runMFEStopBarFixture(t, nil)
	if got.MFEBeforePrice >= 140 {
		t.Fatalf("15m stop-bar High must not be MFE_before without 1m: %+v", got)
	}
}

func TestMFEBeforeStop_Uses1mUntilStopNotWhole15mHigh(t *testing.T) {
	t.Parallel()
	step, _ := data.IntervalDurationMs("15m")
	base := int64(1_609_459_200_000)
	cand := 10
	parent := base + int64(cand+2)*step
	step1m, _ := data.IntervalDurationMs("1m")
	finer := make([]CanonicalClosedBar, 15)
	for i := 0; i < 15; i++ {
		high, low := 101.0, 99.8
		if i >= 10 {
			high, low = 100.0, 95.9
		}
		ot := parent + int64(i)*step1m
		finer[i] = CanonicalClosedBar{OpenTime: ot, Open: 100, High: high, Low: low, Close: 100, Volume: 1}
	}
	got := runMFEStopBarFixture(t, finer)
	if got.MFEBeforePrice >= 140 {
		t.Fatalf("must not take 15m High 150: %+v", got)
	}
	if got.MFEBeforePrice < 100.5 {
		t.Fatalf("must credit 1m High 101 before stop: %+v", got)
	}
}

func runMFEStopBarFixture(t *testing.T, finer []CanonicalClosedBar) StructuralStopAssignment {
	t.Helper()
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
	primary[cand+2].Low = 95.9
	primary[cand+2].High = 150
	primary[cand+20].High = 120
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
		Primary: primary, HTF1h: h1, Tape: tape, FinerMarket: fm, Finer: finer,
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
		t.Fatalf("full-H still includes post-stop rally: before=%g full=%g", got.MFEBeforeStopOverR, got.MFEFullHOverR)
	}
	if math.IsNaN(got.TimeToMFEBefore) {
		t.Fatal("time_to_MFE_before required")
	}
	return got
}
