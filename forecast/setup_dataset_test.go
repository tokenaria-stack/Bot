package forecast

import (
	"math"
	"strings"
	"testing"

	"trading_bot/data"
)

func testSetupSpec2(t *testing.T) FeatureSpec2 {
	t.Helper()
	target, err := ResolveTargetSpec("research-15m-1m-c", TargetSpecDraft{
		HorizonBars: Spec2TargetHorizon, UpperATRMultiple: Spec2UpperATR, LowerATRMultiple: Spec2LowerATR,
		ATRPeriod: 14, DualHit: DualHitResolveFinerHistory, FinerTimeframe: "1m",
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := ResolveAnalysisRecipe("spec2", AnalysisRecipeDraft{
		RSXLength: Spec2RSXLength, RSXSignal: Spec2RSXSignal, RSXSource: Spec2RSXSource,
		DivLookback: Spec2TVLookback, EnableTV: true,
	}, "analysis:v2")
	if err != nil {
		t.Fatal(err)
	}
	primary := MarketKey{Venue: "BINANCE", Instrument: "BTCUSDT", Contract: "FUTURES_PERP", Timeframe: "15m"}
	s, err := ResolveFeatureSpec2(primary, target, analysis)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func testPinnedValidation(t *testing.T, folds []SetupValidationFold) SetupValidationReport {
	t.Helper()
	split := DefaultDataSplitPolicy()
	plan, err := FrozenSetupValidationPlan()
	if err != nil {
		t.Fatal(err)
	}
	sid, err := split.Identity()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := plan.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if len(folds) != SetupValidationFoldCount {
		t.Fatalf("need %d folds", SetupValidationFoldCount)
	}
	return SetupValidationReport{
		Plan: plan, Split: split,
		SplitDigest: sid.Digest.String(), PlanDigest: pid.Digest.String(),
		Folds: folds,
	}
}

func prev15(t *testing.T, at int64, n int) int64 {
	t.Helper()
	x := at
	for i := 0; i < n; i++ {
		var err error
		x, err = data.PreviousBarOpen(x, "15m")
		if err != nil {
			t.Fatal(err)
		}
	}
	return x
}

func next15(t *testing.T, at int64, n int) int64 {
	t.Helper()
	x := at
	for i := 0; i < n; i++ {
		var err error
		x, err = data.NextBarOpen(x, "15m")
		if err != nil {
			t.Fatal(err)
		}
	}
	return x
}

func readyVec(seed float64) FeatureVector2 {
	var v FeatureVector2
	for i := range v {
		v[i] = seed + float64(i)*0.01
	}
	return v
}

func ownedAsg(at int64, anchor int64, age int, risk float64) StructuralStopAssignment {
	return StructuralStopAssignment{
		Status: StructuralStopStatusValid, Side: GeomSideLong, At: at,
		Entry: 100, Stop: 99, R: 1, PivotAnchorAt: anchor, AnchorAgeBars: age, ROverATR15: risk,
	}
}

func TestSetupDataset1FeatureNames_WidthAndOrder(t *testing.T) {
	t.Parallel()
	names := SetupDataset1FeatureNames()
	if len(names) != 66 {
		t.Fatalf("width %d", len(names))
	}
	ids := FeatureSpec2IDs()
	for i, id := range ids {
		if names[i] != string(id) {
			t.Fatalf("spec2 order %d %s != %s", i, names[i], id)
		}
	}
	if names[64] != SetupFeatureRiskATR || names[65] != SetupFeatureS0AnchorAge {
		t.Fatalf("%q %q", names[64], names[65])
	}
}

func TestBuildSetupDataset1_JoinLaws(t *testing.T) {
	t.Parallel()
	spec := testSetupSpec2(t)
	base := int64(1609459200000) // 2021-01-01 00:00 UTC, 15m grid
	trainAt := base
	f0 := next15(t, base, 100)
	f1 := next15(t, base, 200)
	f2 := next15(t, base, 300)
	nr := next15(t, base, 400)
	inv := next15(t, base, 10)
	skip := next15(t, base, 11)
	hold := DefaultResearchHoldoutStartAt
	age := 7
	anchor := func(at int64) int64 { return prev15(t, at, age) }

	labels := SetupLabelSet1{
		Target: FrozenSetupTarget1(), Format: SetupLabelSetFormat, MatchOK: true,
		Rows: []SetupLabelRow{
			{At: trainAt, Class: SetupClassTP, Status: StructuralStopStatusValid},
			{At: inv, Class: SetupClassInvalid, Status: StructuralStopStatusInvalid},
			{At: skip, Class: SetupClassSkip, Status: StructuralStopStatusValid},
			{At: f0, Class: SetupClassTP, Status: StructuralStopStatusValid},
			{At: f1, Class: SetupClassStop, Status: StructuralStopStatusValid},
			{At: f2, Class: SetupClassTimeout, Status: StructuralStopStatusValid},
			{At: nr, Class: SetupClassTP, Status: StructuralStopStatusValid},
			{At: hold, Class: SetupClassStop, Status: StructuralStopStatusValid},
		},
	}
	tape := []FeatureRow2{
		{At: trainAt, Ready: IsReady, Values: readyVec(1)},
		{At: f0, Ready: IsReady, Values: readyVec(2)},
		{At: f1, Ready: IsReady, Values: readyVec(3)},
		{At: f2, Ready: IsReady, Values: readyVec(4)},
		{At: nr, Ready: NotReady, Reason: ReasonPrimaryWarmup},
	}
	asg := []StructuralStopAssignment{
		ownedAsg(trainAt, anchor(trainAt), age, 1.5),
		ownedAsg(f0, anchor(f0), age, 2.25),
		ownedAsg(f1, anchor(f1), age, 3.5),
		ownedAsg(f2, anchor(f2), age, 4.75),
		ownedAsg(nr, anchor(nr), age, 9),
		{Status: StructuralStopStatusValid, Side: GeomSideShort, At: f0, ROverATR15: 99, PivotAnchorAt: anchor(f0), AnchorAgeBars: age},
	}
	val := testPinnedValidation(t, []SetupValidationFold{
		{Index: 0, ValN: 1, ValFirstAt: f0, ValLastAt: f0},
		{Index: 1, ValN: 1, ValFirstAt: f1, ValLastAt: f1},
		{Index: 2, ValN: 1, ValFirstAt: f2, ValLastAt: f2},
		{Index: 3, ValN: 1, ValFirstAt: nr, ValLastAt: nr},
	})
	in := SetupDataset1Input{Spec2: spec, Tape: tape, Labels: labels, Assignments: asg, Validation: val}
	got, err := BuildSetupDataset1(in)
	if err != nil {
		t.Fatal(err)
	}
	got2, err := BuildSetupDataset1(in)
	if err != nil || got2.ContentDigest != got.ContentDigest || got2.Text != got.Text {
		t.Fatal("content digest must be deterministic")
	}
	if got.Width != 66 || len(got.Rows) != 4 {
		t.Fatalf("width=%d n=%d", got.Width, len(got.Rows))
	}
	if got.MaxCandidateAt >= got.HoldoutStartAt || got.HoldoutStartAt != DefaultResearchHoldoutStartAt {
		t.Fatalf("wall %+v", got)
	}
	if got.Accounting.SourceHoldout != 1 || got.Accounting.HoldoutUsable != 1 {
		t.Fatalf("holdout %+v", got.Accounting)
	}
	if got.Accounting.FeatureNotReady != 1 || got.Accounting.DEVInvalid != 1 || got.Accounting.DEVSkip != 1 {
		t.Fatalf("exclusions %+v", got.Accounting)
	}
	if got.Accounting.TrainOnlyRows != 1 {
		t.Fatalf("train-only %d", got.Accounting.TrainOnlyRows)
	}
	if got.Folds[3].Rows != 0 || got.Folds[3].FeatureNotReady != 1 || got.Folds[3].LabelValN != 1 {
		t.Fatalf("fold3 %+v", got.Folds[3])
	}
	var f0row SetupDatasetRow
	for _, r := range got.Rows {
		if r.At == f0 {
			f0row = r
		}
		if r.At >= DefaultResearchHoldoutStartAt {
			t.Fatal("holdout leaked")
		}
		if len(r.X) != 66 {
			t.Fatal("width")
		}
		for _, v := range r.X {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatal("nonfinite")
			}
		}
	}
	if f0row.Y != SetupClassTP || f0row.OOFFold != 0 {
		t.Fatalf("%+v", f0row)
	}
	if f0row.X[0] != 2 || f0row.X[63] != 2+0.63 {
		t.Fatal("FeatureSpec2 order not preserved")
	}
	if f0row.X[64] != 2.25 || f0row.X[65] != float64(age) {
		t.Fatalf("setup facts %v", f0row.X[64:])
	}
	if !strings.Contains(got.Text, "no CatBoost") || strings.Contains(got.Text, "2026 TP") {
		t.Fatal(got.Text)
	}
}

func TestBuildSetupDataset1_MissingTapeAtFails(t *testing.T) {
	t.Parallel()
	spec := testSetupSpec2(t)
	at := int64(1609459200000)
	anchor := prev15(t, at, 3)
	labels := SetupLabelSet1{
		Target: FrozenSetupTarget1(), Format: SetupLabelSetFormat, MatchOK: true,
		Rows: []SetupLabelRow{{At: at, Class: SetupClassTP, Status: StructuralStopStatusValid}},
	}
	val := testPinnedValidation(t, []SetupValidationFold{
		{Index: 0, ValN: 1, ValFirstAt: at, ValLastAt: at},
		{Index: 1, ValN: 0, ValFirstAt: at + 1, ValLastAt: at + 1},
		{Index: 2, ValN: 0, ValFirstAt: at + 2, ValLastAt: at + 2},
		{Index: 3, ValN: 0, ValFirstAt: at + 3, ValLastAt: at + 3},
	})
	_, err := BuildSetupDataset1(SetupDataset1Input{
		Spec2: spec, Labels: labels, Validation: val,
		Assignments: []StructuralStopAssignment{ownedAsg(at, anchor, 3, 1.2)},
	})
	if err == nil || !strings.Contains(err.Error(), "missing exact FeatureTape2") {
		t.Fatalf("got %v", err)
	}
}

func TestBuildSetupDataset1_DuplicateTapeAtFails(t *testing.T) {
	t.Parallel()
	spec := testSetupSpec2(t)
	at := int64(1609459200000)
	anchor := prev15(t, at, 1)
	labels := SetupLabelSet1{
		Target: FrozenSetupTarget1(), Format: SetupLabelSetFormat, MatchOK: true,
		Rows: []SetupLabelRow{{At: at, Class: SetupClassTimeout, Status: StructuralStopStatusValid}},
	}
	val := testPinnedValidation(t, []SetupValidationFold{
		{Index: 0, ValN: 1, ValFirstAt: at, ValLastAt: at},
		{Index: 1, ValN: 0, ValFirstAt: at + 1, ValLastAt: at + 1},
		{Index: 2, ValN: 0, ValFirstAt: at + 2, ValLastAt: at + 2},
		{Index: 3, ValN: 0, ValFirstAt: at + 3, ValLastAt: at + 3},
	})
	tape := []FeatureRow2{
		{At: at, Ready: IsReady, Values: readyVec(1)},
		{At: at, Ready: IsReady, Values: readyVec(2)},
	}
	_, err := BuildSetupDataset1(SetupDataset1Input{
		Spec2: spec, Tape: tape, Labels: labels, Validation: val,
		Assignments: []StructuralStopAssignment{ownedAsg(at, anchor, 1, 1.1)},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate FeatureTape2") {
		t.Fatalf("got %v", err)
	}
}

func TestBuildSetupDataset1_CopiesOwnedGeometryOnly(t *testing.T) {
	t.Parallel()
	spec := testSetupSpec2(t)
	at := int64(1609459200000)
	anchor := prev15(t, at, 4)
	labels := SetupLabelSet1{
		Target: FrozenSetupTarget1(), Format: SetupLabelSetFormat, MatchOK: true,
		Rows: []SetupLabelRow{{At: at, Class: SetupClassStop, Status: StructuralStopStatusValid}},
	}
	val := testPinnedValidation(t, []SetupValidationFold{
		{Index: 0, ValN: 1, ValFirstAt: at, ValLastAt: at},
		{Index: 1, ValN: 0, ValFirstAt: at + 1, ValLastAt: at + 1},
		{Index: 2, ValN: 0, ValFirstAt: at + 2, ValLastAt: at + 2},
		{Index: 3, ValN: 0, ValFirstAt: at + 3, ValLastAt: at + 3},
	})
	in := SetupDataset1Input{
		Spec2: spec, Labels: labels, Validation: val,
		Tape:        []FeatureRow2{{At: at, Ready: IsReady, Values: readyVec(8)}},
		Assignments: []StructuralStopAssignment{ownedAsg(at, anchor, 99, 1.4)}, // wrong owned age
	}
	if _, err := BuildSetupDataset1(in); err == nil || !strings.Contains(err.Error(), "AnchorAgeBars") {
		t.Fatalf("poisoned age must fail, got %v", err)
	}
	in.Assignments = []StructuralStopAssignment{ownedAsg(at, anchor, 4, math.NaN())}
	if _, err := BuildSetupDataset1(in); err == nil || !strings.Contains(err.Error(), "setup_risk_atr") {
		t.Fatalf("NaN risk must fail, got %v", err)
	}
}

func TestBuildSetupDataset1_RefusesHoldoutWallMismatch(t *testing.T) {
	t.Parallel()
	spec := testSetupSpec2(t)
	val := testPinnedValidation(t, []SetupValidationFold{
		{Index: 0, ValN: 0, ValFirstAt: 1, ValLastAt: 1},
		{Index: 1, ValN: 0, ValFirstAt: 2, ValLastAt: 2},
		{Index: 2, ValN: 0, ValFirstAt: 3, ValLastAt: 3},
		{Index: 3, ValN: 0, ValFirstAt: 4, ValLastAt: 4},
	})
	val.Split.HoldoutStartAt = DefaultResearchHoldoutStartAt + 15*60*1000
	_, err := BuildSetupDataset1(SetupDataset1Input{
		Spec2: spec, Labels: SetupLabelSet1{Target: FrozenSetupTarget1(), MatchOK: true}, Validation: val,
	})
	if err == nil || (!strings.Contains(err.Error(), "DataSplitPolicy identity") && !strings.Contains(err.Error(), "holdout wall")) {
		t.Fatalf("got %v", err)
	}
}

func TestFrozenSetupTarget1_IdentityStable(t *testing.T) {
	t.Parallel()
	a, err := FrozenSetupTarget1().Identity()
	if err != nil {
		t.Fatal(err)
	}
	b, err := FrozenSetupTarget1().Identity()
	if err != nil || a.Digest != b.Digest {
		t.Fatal("target identity")
	}
}
