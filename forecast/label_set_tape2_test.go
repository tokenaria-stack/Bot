package forecast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testSpec2(t *testing.T) FeatureSpec2 {
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

func dummyVec2(seed float64) []float64 {
	v := make([]float64, Spec2FeatureWidth)
	for i := range v {
		v[i] = seed + float64(i)*0.001
	}
	return v
}

func tape2HeaderForPrimary(t *testing.T, spec2 FeatureSpec2, primary []CanonicalClosedBar) Tape2Header {
	t.Helper()
	dummy := []CanonicalClosedBar{{OpenTime: 1, Open: 1, High: 2, Low: 0.5, Close: 1.2, Volume: 1}}
	sid, err := spec2.Identity()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := spec2.Plan.Identity()
	if err != nil {
		t.Fatal(err)
	}
	fid, err := spec2.Features.Identity()
	if err != nil {
		t.Fatal(err)
	}
	aid, err := spec2.Analysis.Identity()
	if err != nil {
		t.Fatal(err)
	}
	tid, err := spec2.Target.Identity()
	if err != nil {
		t.Fatal(err)
	}
	return Tape2Header{
		FormatVersion: FeatureTapeFormatV2, SpecDigest: sid.Digest, PlanDigest: pid.Digest,
		FeaturesDigest: fid.Digest, AnalysisDigest: aid.Digest, TargetDigest: tid.Digest,
		Primary: spec2.Primary, HTF1h: spec2.HTF1h, HTF4h: spec2.HTF4h, FeatureIDs: FeatureSpec2IDs(),
		VectorLen: Spec2FeatureWidth, Q: spec2.Q, Demand: spec2.Demand,
		PrimarySource: SourceRangeDigest(spec2.Primary, primary),
		HTF1hSource:   SourceRangeDigest(spec2.HTF1h, dummy),
		HTF4hSource:   SourceRangeDigest(spec2.HTF4h, dummy),
	}
}

func writeLabelTape2(t *testing.T, dir string, spec2 FeatureSpec2, bars []CanonicalClosedBar, readyAt map[int64]bool, seed float64) (string, Tape2Header, Tape2Footer) {
	t.Helper()
	path := filepath.Join(dir, "t.featuretape2")
	h := tape2HeaderForPrimary(t, spec2, bars)
	w, err := CreateTape2Writer(path, h)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bars {
		ready := true
		if readyAt != nil {
			if v, ok := readyAt[b.OpenTime]; ok {
				ready = v
			}
		}
		if ready {
			if err := w.WriteRow(b.OpenTime, IsReady, "", dummyVec2(seed)); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := w.WriteRow(b.OpenTime, NotReady, ReasonPrimaryWarmup, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	foot, err := w.Finish()
	if err != nil {
		t.Fatal(err)
	}
	return path, h, foot
}

func genTape2Labels(t *testing.T, spec2 FeatureSpec2, tapeBars, primary, finer []CanonicalClosedBar, readyAt map[int64]bool, seed float64) (LabelHeader, []LabelRow, LabelFooter, Tape2Header, Tape2Footer) {
	t.Helper()
	dir := t.TempDir()
	tapePath, th, tf := writeLabelTape2(t, dir, spec2, tapeBars, readyAt, seed)
	out := filepath.Join(dir, "t.labelset")
	fm := finerMarketOf(spec2.Primary, spec2.Target.FinerTimeframe)
	if err := GenerateLabelSetFromTape2(out, tapePath, spec2, primary, fm, finer, nil); err != nil {
		t.Fatal(err)
	}
	h, rows, f, err := ReadLabelSet(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	return h, rows, f, th, tf
}

func TestLabelSetFromTape2_RefusesV1Tape(t *testing.T) {
	spec2 := testSpec2(t)
	dir := t.TempDir()
	primary := stampBars(t, boundBars(20))
	tapePath, _ := writeLabelTape(t, dir, primary[10:11], nil)
	out := filepath.Join(dir, "t.labelset")
	err := GenerateLabelSetFromTape2(out, tapePath, spec2, primary, MarketKey{}, nil, nil)
	if err == nil {
		t.Fatal("expected native v2 refuse of feature-tape-v1")
	}
}

func TestLabelSetFromTape2_CorruptTapeRefused(t *testing.T) {
	spec2 := testSpec2(t)
	dir := t.TempDir()
	primary := stampBars(t, boundBars(20))
	tapePath, _, _ := writeLabelTape2(t, dir, spec2, primary[10:12], nil, 1)
	raw, err := os.ReadFile(tapePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tapePath+".bad", raw[:len(raw)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "t.labelset")
	if err := GenerateLabelSetFromTape2(out, tapePath+".bad", spec2, primary, MarketKey{}, nil, nil); err == nil {
		t.Fatal("expected corrupt tape2 refuse")
	}
}

func TestLabelSetFromTape2_WrongSpec2Refused(t *testing.T) {
	spec2 := testSpec2(t)
	dir := t.TempDir()
	primary := stampBars(t, boundBars(20))
	tapePath, _, _ := writeLabelTape2(t, dir, spec2, primary[10:11], nil, 1)
	otherPrimary := spec2.Primary
	otherPrimary.Instrument = "ETHUSDT"
	target := spec2.Target
	other, err := ResolveFeatureSpec2(otherPrimary, target, spec2.Analysis)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "t.labelset")
	err = GenerateLabelSetFromTape2(out, tapePath, other, primary, MarketKey{}, nil, nil)
	if err == nil || !(strings.Contains(err.Error(), "SpecDigest") || strings.Contains(err.Error(), "MarketKey")) {
		t.Fatalf("want spec2 refuse, got %v", err)
	}
}

func TestLabelSetFromTape2_WrongTargetCRefused(t *testing.T) {
	spec2 := testSpec2(t)
	tid, err := spec2.Target.Identity()
	if err != nil {
		t.Fatal(err)
	}
	wrong := tid.Digest
	wrong[0] ^= 0xff
	dir := t.TempDir()
	primary := stampBars(t, boundBars(20))
	tapePath, _, _ := writeLabelTape2(t, dir, spec2, primary[10:11], nil, 1)
	out := filepath.Join(dir, "t.labelset")
	err = GenerateLabelSetFromTape2(out, tapePath, spec2, primary, MarketKey{}, nil, &LabelExpect{Target: &wrong})
	if err == nil || !strings.Contains(err.Error(), "TargetDigest") {
		t.Fatalf("want Target C refuse, got %v", err)
	}
}

func TestLabelSetFromTape2_AllRowsBecomeCandidates(t *testing.T) {
	spec2 := testSpec2(t)
	primary := stampBars(t, boundBars(30))
	tape := primary[10:14]
	readyAt := map[int64]bool{tape[0].OpenTime: false, tape[2].OpenTime: false}
	h, rows, f, th, tf := genTape2Labels(t, spec2, tape, primary, nil, readyAt, 3)
	if len(rows) != len(tape) || f.RowCount != len(tape) {
		t.Fatalf("row lockstep tape=%d labels=%d footer=%d", len(tape), len(rows), f.RowCount)
	}
	for i := range tape {
		if rows[i].At != tape[i].OpenTime {
			t.Fatalf("At lockstep i=%d tape=%d label=%d", i, tape[i].OpenTime, rows[i].At)
		}
	}
	if h.FeatureTapeContentDigest != tf.ContentDigest || h.FeatureTapePlanDigest != th.PlanDigest {
		t.Fatal("candidate-source binding")
	}
	if h.FeatureTapeSourceRangeDigest != th.PrimarySource {
		t.Fatal("primary consumed digest must be Tape2 PrimarySource")
	}
	if h.FeatureTapeSourceRangeDigest == th.HTF1hSource || h.FeatureTapeSourceRangeDigest == th.HTF4hSource {
		t.Fatal("must not bind HTF source as primary label provenance")
	}
}

func TestLabelSetFromTape2_ValuesIgnored(t *testing.T) {
	spec2 := testSpec2(t)
	primary := stampBars(t, boundBars(30))
	tape := primary[10:12]
	_, rowsA, _, _, _ := genTape2Labels(t, spec2, tape, primary, nil, nil, 1)
	_, rowsB, _, _, _ := genTape2Labels(t, spec2, tape, primary, nil, nil, 99)
	if len(rowsA) != len(rowsB) {
		t.Fatal("len")
	}
	for i := range rowsA {
		if rowsA[i].At != rowsB[i].At || rowsA[i].Outcome != rowsB[i].Outcome || rowsA[i].HitAt != rowsB[i].HitAt || rowsA[i].Reason != rowsB[i].Reason {
			t.Fatalf("values must not change label math: %+v vs %+v", rowsA[i], rowsB[i])
		}
	}
}

func TestLabelSetFromTape2_MalformedOrderRefused(t *testing.T) {
	u := labelCandidateUniverse{
		Ats:           []int64{2, 2},
		Market:        testSpec2(t).Primary,
		PlanDigest:    Digest{1},
		PrimarySource: Digest{2},
		TapeContent:   Digest{3},
	}
	spec2 := testSpec2(t)
	_, err := buildLabelsFromCandidates(u, spec2.Target, stampBars(t, boundBars(10)), MarketKey{}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "strictly increasing") {
		t.Fatalf("want order refuse, got %v", err)
	}
}

func TestLabelSetFromTape2_TargetCExact(t *testing.T) {
	spec2 := testSpec2(t)
	tid, err := spec2.Target.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if spec2.Target.HorizonBars != 72 || spec2.Target.UpperATRMultiple != 2.0 || spec2.Target.LowerATRMultiple != 2.0 {
		t.Fatalf("Target C H/U/L %+v", spec2.Target)
	}
	if tid.Digest.String() != "3d4e57acfd061983c78feca7bbc4e5ad62f788a8e179ebbbdaf32205761c8922" {
		t.Fatalf("TargetDigest %s", tid.Digest)
	}
	primary := stampBars(t, boundBars(30))
	h, _, _, _, _ := genTape2Labels(t, spec2, primary[10:11], primary, nil, nil, 1)
	if h.TargetDigest != tid.Digest {
		t.Fatal("header TargetDigest")
	}
	if h.FormatVersion != LabelSetFormatV2 {
		t.Fatalf("format %s", h.FormatVersion)
	}
}

func TestLabelSetFromTape2_UpDownTimeout(t *testing.T) {
	spec2 := testSpec2(t)
	spec := spec2.Target
	primary := stampBars(t, boundBars(100))
	c := 20
	atr := atrThrough(t, spec, primary, c)
	upper := primary[c].Close + spec.UpperATRMultiple*atr
	lower := primary[c].Close - spec.LowerATRMultiple*atr

	up := append([]CanonicalClosedBar(nil), primary...)
	up[c+3].High = upper
	up[c+3].Low = primary[c].Close
	_, upRows, _, _, _ := genTape2Labels(t, spec2, up[c:c+1], up, nil, nil, 1)
	if len(upRows) != 1 || upRows[0].Outcome != OutcomeUpFirst || upRows[0].HitAt != up[c+3].OpenTime {
		t.Fatalf("UP_FIRST %+v", upRows)
	}

	down := append([]CanonicalClosedBar(nil), primary...)
	down[c+3].High = primary[c].Close
	down[c+3].Low = lower
	_, downRows, _, _, _ := genTape2Labels(t, spec2, down[c:c+1], down, nil, nil, 1)
	if len(downRows) != 1 || downRows[0].Outcome != OutcomeDownFirst || downRows[0].HitAt != down[c+3].OpenTime {
		t.Fatalf("DOWN_FIRST %+v", downRows)
	}

	_, toRows, _, _, _ := genTape2Labels(t, spec2, primary[c:c+1], primary, nil, nil, 1)
	if len(toRows) != 1 || toRows[0].Outcome != OutcomeTimeout || toRows[0].HitAt != 0 || toRows[0].Reason != ReasonNone {
		t.Fatalf("TIMEOUT %+v", toRows)
	}
}

func TestLabelSetFromTape2_DualHitFinerUnchanged(t *testing.T) {
	spec2 := testSpec2(t)
	spec := spec2.Target
	primary := stampBars(t, boundBars(40))
	c := 20
	atr := atrThrough(t, spec, primary, c)
	upper := primary[c].Close + spec.UpperATRMultiple*atr
	lower := primary[c].Close - spec.LowerATRMultiple*atr
	parent := c + 1
	primary[parent].High = upper
	primary[parent].Low = lower
	finer := minuteBars(t, primary[parent].OpenTime, 15, primary[c].Close, upper, primary[c].Close)
	h, rows, f, _, _ := genTape2Labels(t, spec2, primary[c:c+1], primary, finer, nil, 1)
	if len(rows) != 1 || rows[0].Outcome != OutcomeUpFirst || rows[0].Reason != ReasonNone {
		t.Fatalf("dual-hit resolve %+v", rows)
	}
	if rows[0].HitAt != primary[parent].OpenTime {
		t.Fatalf("HitAt must remain primary dual-hit bar, got %d", rows[0].HitAt)
	}
	if f.FinerWindowCount != 1 || f.FinerSourceDigest == (Digest{}) {
		t.Fatalf("finer provenance %+v", f)
	}
	if h.FinerMarket.Timeframe != "1m" {
		t.Fatal("finer MarketKey")
	}
}

func TestLabelSetFromTape2_TruncatedHorizonH72(t *testing.T) {
	spec2 := testSpec2(t)
	primary := stampBars(t, boundBars(30))
	c := 20
	_, rows, _, _, _ := genTape2Labels(t, spec2, primary[c:c+1], primary, nil, nil, 1)
	if len(rows) != 1 || rows[0].Outcome != OutcomeAmbiguous || rows[0].Reason != ReasonTruncatedHorizon {
		t.Fatalf("TRUNCATED_HORIZON %+v", rows)
	}
}

func TestLabelSetFromTape2_RoundtripFormatV2(t *testing.T) {
	spec2 := testSpec2(t)
	primary := stampBars(t, boundBars(30))
	dir := t.TempDir()
	tapePath, _, _ := writeLabelTape2(t, dir, spec2, primary[10:12], nil, 1)
	out := filepath.Join(dir, "t.labelset")
	fm := finerMarketOf(spec2.Primary, spec2.Target.FinerTimeframe)
	if err := GenerateLabelSetFromTape2(out, tapePath, spec2, primary, fm, nil, nil); err != nil {
		t.Fatal(err)
	}
	h1, rows1, f1, err := ReadLabelSet(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	h2, rows2, f2, err := ReadLabelSet(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if h1.FormatVersion != LabelSetFormatV2 || h1 != h2 || f1.ContentDigest != f2.ContentDigest || len(rows1) != len(rows2) {
		t.Fatal("label-set-v2 readback")
	}
}
