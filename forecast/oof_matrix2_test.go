package forecast

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeOOFWorld2(t *testing.T, dir string, n, holdoutIdx int, folds, span, minTrain int) (tapePath, labelPath string, spec2 FeatureSpec2, plan ValidationPlan, ats []int64) {
	t.Helper()
	spec2 = testSpec2(t)
	start := int64(1_699_999_200_000)
	ats = seq15m(t, start, n)
	bars := make([]CanonicalClosedBar, n)
	readyAt := map[int64]bool{}
	for i, at := range ats {
		bars[i] = CanonicalClosedBar{OpenTime: at, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1}
	}
	readyAt[ats[0]] = false
	tapePath, th, tf := writeLabelTape2(t, dir, spec2, bars, readyAt, 1.5)
	outcomes := []TargetOutcome{OutcomeUpFirst, OutcomeDownFirst, OutcomeTimeout}
	lrows := make([]LabelRow, n)
	for i := 0; i < n; i++ {
		out := outcomes[i%3]
		hit := ats[i] + 1
		if out == OutcomeTimeout {
			hit = 0
		}
		reason := ReasonNone
		if i == n-1 {
			out = OutcomeAmbiguous
			reason = ReasonTruncatedHorizon
			hit = 0
		}
		lrows[i] = LabelRow{At: ats[i], Outcome: out, Reason: reason, HitAt: hit}
	}
	labelPath = writeResearchLabels2(t, dir, spec2, th, tf, lrows)
	plan = mustPlan(t, ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: ats[holdoutIdx], ValidationSpanBars: span,
		FoldCount: folds, ExtraGapBars: 0, MinTrainRows: minTrain,
	}, spec2.Target.HorizonBars)
	return tapePath, labelPath, spec2, plan, ats
}

func TestGenerateOOFMatrixFromTape2_RefusesV1Tape(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, expect, plan := writeTinyOOFWorld(t, dir, 80, nil)
	spec2 := testSpec2(t)
	_, _, _, err := GenerateOOFMatrixFromTape2(tape, labels, filepath.Join(dir, "m.oofmatrix"), spec2, ResearchDataset2Expect{}, plan)
	if err == nil {
		t.Fatal("V2 door must refuse feature-tape-v1")
	}
	_ = expect
}

func TestGenerateOOFMatrixFromTape2_DevelopmentPopulation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, spec2, plan, _ := writeOOFWorld2(t, dir, 250, 200, 1, 30, 5)
	out := filepath.Join(dir, "m.oofmatrix")
	hdr, ft, matched, err := GenerateOOFMatrixFromTape2(tape, labels, out, spec2, ResearchDataset2Expect{}, plan)
	if err != nil || matched {
		t.Fatalf("generate: matched=%v err=%v", matched, err)
	}
	rows, acc, err := BuildResearchDatasetFromTape2(tape, labels, spec2, ResearchDataset2Expect{})
	if err != nil {
		t.Fatal(err)
	}
	if acc.FeatureNotReady != 1 || acc.ExcludedTotal != 1 {
		t.Fatalf("partition %+v", acc)
	}
	ats := make([]int64, len(rows))
	for i := range rows {
		ats[i] = rows[i].At
	}
	compiled, err := CompileValidationPlan(ats, spec2.Primary.Timeframe, plan)
	if err != nil {
		t.Fatal(err)
	}
	if hdr.DevRowCount != compiled.DevelopmentEndIndex || ft.RowCount != compiled.DevelopmentEndIndex {
		t.Fatalf("row count hdr=%d ft=%d compiled=%d trainable=%d", hdr.DevRowCount, ft.RowCount, compiled.DevelopmentEndIndex, acc.TrainableTotal)
	}
	if hdr.DevRowCount == compiled.HoldoutBeginIndex || hdr.DevRowCount == acc.TrainableTotal {
		t.Fatal("matrix must be development prefix, not holdout-begin or full trainable")
	}
	gotH, gotRows, _, err := ReadOOFMatrix(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotRows) != compiled.DevelopmentEndIndex {
		t.Fatalf("matrix rows %d", len(gotRows))
	}
	if gotRows[0].At != rows[0].At {
		t.Fatal("early train-only row missing")
	}
	if compiled.Folds[0].ValBegin == 0 {
		t.Fatal("fixture needs train-only prefix before fold0 val")
	}
	if gotRows[len(gotRows)-1].At != rows[compiled.DevelopmentEndIndex-1].At {
		t.Fatal("last matrix row is not last legal development observation")
	}
	union := 0
	for _, f := range gotH.Folds {
		union += f.ValEnd - f.ValBegin
	}
	if len(gotRows) == union {
		t.Fatal("matrix must not be validation-slice union")
	}
	for _, r := range gotRows {
		if r.At >= compiled.DevelopmentExclusiveEndAt {
			t.Fatalf("seam leaked %d", r.At)
		}
		if r.At >= compiled.HoldoutStartAt {
			t.Fatalf("holdout leaked %d", r.At)
		}
		if r.Outcome == OutcomeAmbiguous {
			t.Fatal("AMBIGUOUS in matrix")
		}
		switch r.Outcome {
		case OutcomeUpFirst, OutcomeDownFirst, OutcomeTimeout:
		default:
			t.Fatalf("illegal outcome %q", r.Outcome)
		}
		if len(r.Features) != Spec2FeatureWidth {
			t.Fatalf("width %d", len(r.Features))
		}
		ids := FeatureSpec2IDs()
		if len(gotH.FeatureIDs) != len(ids) {
			t.Fatal("FeatureIDs count")
		}
		wantX := dummyVec2(1.5)
		for i := range ids {
			if gotH.FeatureIDs[i] != ids[i] {
				t.Fatalf("FeatureIDs[%d]=%s want %s", i, gotH.FeatureIDs[i], ids[i])
			}
			if math.Float64bits(r.Features[i]) != math.Float64bits(wantX[i]) {
				t.Fatalf("X[%d] bits mismatch for %s", i, ids[i])
			}
			if math.IsNaN(r.Features[i]) || math.IsInf(r.Features[i], 0) {
				t.Fatal("nonfinite")
			}
		}
	}
	if gotH.FormatVersion != OOFMatrixFormatV1 || gotH.AtUnit != OOFAtUnitUnixMs {
		t.Fatalf("format %+v", gotH)
	}
	if gotH.Folds[0] != compiled.Folds[0] {
		t.Fatal("fold geometry must be CompileValidationPlan")
	}
	if gotH.Folds[0].ValEnd-gotH.Folds[0].ValBegin != compiled.Folds[0].ValEnd-compiled.Folds[0].ValBegin {
		t.Fatal("ValN must equal compiled geometry, not a repaired span")
	}
	if gotH.Folds[0].TrainEnd > gotH.DevRowCount || gotH.Folds[0].ValEnd > gotH.DevRowCount {
		t.Fatal("fold not matrix-local")
	}
	if gotH.TapeContent == (Digest{}) || gotH.LabelContent == (Digest{}) {
		t.Fatal("source provenance missing")
	}
}

func TestGenerateOOFMatrixFromTape2_FoldsAndDeterminism(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, spec2, plan, _ := writeOOFWorld2(t, dir, 400, 360, 4, 20, 5)
	out := filepath.Join(dir, "m.oofmatrix")
	h1, f1, _, err := GenerateOOFMatrixFromTape2(tape, labels, out, spec2, ResearchDataset2Expect{}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(h1.Folds) != 4 {
		t.Fatalf("folds %d", len(h1.Folds))
	}
	rows, _, err := BuildResearchDatasetFromTape2(tape, labels, spec2, ResearchDataset2Expect{})
	if err != nil {
		t.Fatal(err)
	}
	ats := make([]int64, len(rows))
	for i := range rows {
		ats[i] = rows[i].At
	}
	compiled, err := CompileValidationPlan(ats, spec2.Primary.Timeframe, plan)
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range h1.Folds {
		cf := compiled.Folds[i]
		if f != cf {
			t.Fatalf("fold %d != compiled", i)
		}
		if f.TrainBegin != 0 || f.TrainEnd < 0 || f.ValBegin < 0 || f.ValEnd > h1.DevRowCount || f.TrainEnd > h1.DevRowCount {
			t.Fatalf("fold %d matrix-local fail %+v", i, f)
		}
		if i > 0 && f.TrainEnd <= h1.Folds[i-1].TrainEnd {
			t.Fatalf("expanding train broken at fold %d", i)
		}
		if f.ValEnd-f.ValBegin == 17568 {
			t.Fatal("forced ValN=17568")
		}
	}
	sparseDir := t.TempDir()
	spec2s := testSpec2(t)
	start := int64(1_699_999_200_000)
	n := 250
	atsS := seq15m(t, start, n)
	bars := make([]CanonicalClosedBar, n)
	readyAt := map[int64]bool{}
	for i, at := range atsS {
		bars[i] = CanonicalClosedBar{OpenTime: at, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1}
		if i%2 == 1 {
			readyAt[at] = false
		}
	}
	tapeS, thS, tfS := writeLabelTape2(t, sparseDir, spec2s, bars, readyAt, 1.5)
	lrows := make([]LabelRow, n)
	for i := 0; i < n; i++ {
		out := OutcomeTimeout
		lrows[i] = LabelRow{At: atsS[i], Outcome: out, Reason: ReasonNone}
	}
	labS := writeResearchLabels2(t, sparseDir, spec2s, thS, tfS, lrows)
	planS := mustPlan(t, ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: atsS[200], ValidationSpanBars: 30,
		FoldCount: 1, ExtraGapBars: 0, MinTrainRows: 5,
	}, spec2s.Target.HorizonBars)
	hS, _, _, err := GenerateOOFMatrixFromTape2(tapeS, labS, filepath.Join(sparseDir, "s.oofmatrix"), spec2s, ResearchDataset2Expect{}, planS)
	if err != nil {
		t.Fatal(err)
	}
	if hS.Folds[0].ValEnd-hS.Folds[0].ValBegin >= 30 {
		t.Fatalf("sparse ValN=%d must not be repaired to ValidationSpanBars=30", hS.Folds[0].ValEnd-hS.Folds[0].ValBegin)
	}
	dir2 := t.TempDir()
	tape2, labels2, spec2b, plan2, _ := writeOOFWorld2(t, dir2, 400, 360, 4, 20, 5)
	out2 := filepath.Join(dir2, "n.oofmatrix")
	h2, f2, _, err := GenerateOOFMatrixFromTape2(tape2, labels2, out2, spec2b, ResearchDataset2Expect{}, plan2)
	if err != nil {
		t.Fatal(err)
	}
	if f1.ContentDigest != f2.ContentDigest || h1.ValPlan.Digest != h2.ValPlan.Digest {
		t.Fatal("independent identical worlds must match ContentDigest")
	}
	_, got1, _, err := ReadOOFMatrix(out)
	if err != nil {
		t.Fatal(err)
	}
	_, got2, _, err := ReadOOFMatrix(out2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got1) != len(got2) {
		t.Fatal("row count")
	}
	for i := range got1 {
		if got1[i].At != got2[i].At || got1[i].Outcome != got2[i].Outcome {
			t.Fatalf("row %d semantics", i)
		}
		if len(got1[i].Features) != Spec2FeatureWidth {
			t.Fatal("width")
		}
		for j := range got1[i].Features {
			if math.Float64bits(got1[i].Features[j]) != math.Float64bits(got2[i].Features[j]) {
				t.Fatalf("Float64bits row %d col %d", i, j)
			}
		}
	}
	st1, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	_, _, matched, err := GenerateOOFMatrixFromTape2(tape, labels, out, spec2, ResearchDataset2Expect{}, plan)
	if err != nil || !matched {
		t.Fatalf("MATCH: matched=%v err=%v", matched, err)
	}
	st2, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if st1.ModTime() != st2.ModTime() || st1.Size() != st2.Size() {
		t.Fatal("MATCH rewrote")
	}
	other := mustPlan(t, ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: plan.HoldoutStartAt, ValidationSpanBars: 21,
		FoldCount: 4, ExtraGapBars: 0, MinTrainRows: 5,
	}, spec2.Target.HorizonBars)
	_, _, _, err = GenerateOOFMatrixFromTape2(tape, labels, out, spec2, ResearchDataset2Expect{}, other)
	if err == nil || !strings.Contains(err.Error(), "refuse overwrite") {
		t.Fatalf("want collision refuse, got %v", err)
	}
	bad := filepath.Join(dir, "bad.oofmatrix")
	if err := os.WriteFile(bad, []byte("not-jsonl\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = GenerateOOFMatrixFromTape2(tape, labels, bad, spec2, ResearchDataset2Expect{}, plan)
	if err == nil || !strings.Contains(err.Error(), "refuse existing") {
		t.Fatalf("want corrupt refuse, got %v", err)
	}
}

func TestGenerateOOFMatrixFromTape2_InputMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, spec2, plan, _ := writeOOFWorld2(t, dir, 250, 200, 1, 30, 5)
	out := filepath.Join(dir, "m.oofmatrix")
	bad := Digest{0xff}
	_, _, _, err := GenerateOOFMatrixFromTape2(tape, labels, out, spec2, ResearchDataset2Expect{TapeContent: &bad}, plan)
	if err == nil {
		t.Fatal("want refuse wrong Tape2 ContentDigest")
	}
	_, _, _, err = GenerateOOFMatrixFromTape2(tape, labels, out, spec2, ResearchDataset2Expect{LabelContent: &bad}, plan)
	if err == nil {
		t.Fatal("want refuse wrong LabelSet ContentDigest")
	}
	h24 := mustPlan(t, ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: plan.HoldoutStartAt, ValidationSpanBars: 30,
		FoldCount: 1, ExtraGapBars: 0, MinTrainRows: 5,
	}, 24)
	_, _, _, err = GenerateOOFMatrixFromTape2(tape, labels, out, spec2, ResearchDataset2Expect{}, h24)
	if err == nil || !strings.Contains(err.Error(), "TargetH") {
		t.Fatalf("want TargetH refuse, got %v", err)
	}
	other := testSpec2(t)
	other.Primary.Instrument = "ETHUSDT"
	_, _, _, err = GenerateOOFMatrixFromTape2(tape, labels, out, other, ResearchDataset2Expect{}, plan)
	if err == nil {
		t.Fatal("want refuse FeatureSpec2 mismatch")
	}
}

func TestOOFSourceSnap2_TOCTOUEqual(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, _, _, _ := writeOOFWorld2(t, dir, 250, 200, 1, 30, 5)
	a, err := readOOFSources2(tape, labels)
	if err != nil {
		t.Fatal(err)
	}
	b, err := readOOFSources2(tape, labels)
	if err != nil {
		t.Fatal(err)
	}
	if !a.equal(b) {
		t.Fatal("identical Tape2 reads must match")
	}
	b.TapeContent[0] ^= 1
	if a.equal(b) {
		t.Fatal("changed Tape2 ContentDigest must refuse")
	}
	b = a
	b.labFt.ContentDigest[0] ^= 1
	if a.equal(b) {
		t.Fatal("changed LabelSet ContentDigest must refuse")
	}
}

func TestGenerateOOFMatrixFromTape2_BitwiseDatasetCopy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, spec2, plan, _ := writeOOFWorld2(t, dir, 250, 200, 1, 30, 5)
	out := filepath.Join(dir, "m.oofmatrix")
	hdr, _, _, err := GenerateOOFMatrixFromTape2(tape, labels, out, spec2, ResearchDataset2Expect{}, plan)
	if err != nil {
		t.Fatal(err)
	}
	rows, _, err := BuildResearchDatasetFromTape2(tape, labels, spec2, ResearchDataset2Expect{})
	if err != nil {
		t.Fatal(err)
	}
	_, got, _, err := ReadOOFMatrix(out)
	if err != nil {
		t.Fatal(err)
	}
	for i := range got {
		src := rows[i].Features
		if len(got[i].Features) != Spec2FeatureWidth {
			t.Fatal("width")
		}
		for j := 0; j < Spec2FeatureWidth; j++ {
			if math.Float64bits(got[i].Features[j]) != math.Float64bits(src[j]) {
				t.Fatalf("not bitwise Dataset-C copy row=%d col=%d id=%s", i, j, hdr.FeatureIDs[j])
			}
		}
	}
}
