package forecast

import (
	"path/filepath"
	"testing"
)

func TestPinnedCatBoostSpec1_IdentityAndClassMap(t *testing.T) {
	t.Parallel()
	s := PinnedCatBoostSpec1()
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if s.ClassOrder != OOFClassOrder {
		t.Fatal(s.ClassOrder)
	}
	if s.InnerSpanBars != 17568 || s.MinInnerTrainRows != 35040 || s.MaxIterations != 1000 {
		t.Fatal("inner/cap pins")
	}
	if s.ThreadCount != 1 || s.UseBestModel || s.StandardScaler != "NONE" || s.ClassWeights != "NONE" {
		t.Fatal("trainer knobs")
	}
	id1, err := s.Identity()
	if err != nil {
		t.Fatal(err)
	}
	id2, err := PinnedCatBoostSpec1().Identity()
	if err != nil {
		t.Fatal(err)
	}
	if id1.Digest != id2.Digest {
		t.Fatal("CatBoostSpec1 digest must be deterministic")
	}
	up, err := EncodeOOFClass(OutcomeUpFirst)
	if err != nil || up != 0 {
		t.Fatal(up, err)
	}
	dn, err := EncodeOOFClass(OutcomeDownFirst)
	if err != nil || dn != 1 {
		t.Fatal(dn, err)
	}
	to, err := EncodeOOFClass(OutcomeTimeout)
	if err != nil || to != 2 {
		t.Fatal(to, err)
	}
	if _, err := EncodeOOFClass(OutcomeAmbiguous); err == nil {
		t.Fatal("AMBIGUOUS must refuse")
	}
	bad := s
	bad.ClassOrder[0], bad.ClassOrder[1] = bad.ClassOrder[1], bad.ClassOrder[0]
	if err := bad.Validate(); err == nil {
		t.Fatal("swapped class order must refuse")
	}
}

func testCatBoostSpecTiny() CatBoostSpec1 {
	s := PinnedCatBoostSpec1()
	s.MaxIterations = 8
	s.Depth = 2
	s.InnerSpanBars = 20
	s.MinInnerTrainRows = 5
	return s
}

func TestResolveCatBoostFitPlan_OuterTrainTail(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, spec2, plan, _ := writeOOFWorld2(t, dir, 400, 360, 4, 20, 5)
	out := filepath.Join(dir, "m.oofmatrix")
	hdr, ft, _, err := GenerateOOFMatrixFromTape2(tape, labels, out, spec2, ResearchDataset2Expect{}, plan)
	if err != nil {
		t.Fatal(err)
	}
	_, rows, _, err := ReadOOFMatrix(out)
	if err != nil {
		t.Fatal(err)
	}
	cs := testCatBoostSpecTiny()
	fp, err := ResolveCatBoostFitPlan(hdr, rows, ft.ContentDigest, cs)
	if err != nil {
		t.Fatal(err)
	}
	if fp.Digest() == (Digest{}) {
		t.Fatal("empty fitplan digest")
	}
	fp2, err := ResolveCatBoostFitPlan(hdr, rows, ft.ContentDigest, cs)
	if err != nil {
		t.Fatal(err)
	}
	if fp.Digest() != fp2.Digest() {
		t.Fatal("FitPlan digest must be deterministic")
	}
	if len(fp.FeatureIDs) != Spec2FeatureWidth || fp.FeatureIDs[0] != FeatureSpec2IDs()[0] {
		t.Fatal("FeatureIDs")
	}
	mut := hdr
	mut.Folds = append([]CompiledFold(nil), hdr.Folds...)
	for i := range mut.Folds {
		mut.Folds[i].ValBoundaryStartAt += 15 * 60 * 1000
	}
	fp3, err := ResolveCatBoostFitPlan(mut, rows, ft.ContentDigest, cs)
	if err != nil {
		t.Fatal(err)
	}
	for i := range fp.Folds {
		if fp.Folds[i].InnerTrainBegin != fp3.Folds[i].InnerTrainBegin ||
			fp.Folds[i].InnerTrainEnd != fp3.Folds[i].InnerTrainEnd ||
			fp.Folds[i].InnerValBegin != fp3.Folds[i].InnerValBegin ||
			fp.Folds[i].InnerValEnd != fp3.Folds[i].InnerValEnd ||
			fp.Folds[i].TailExclusiveEnd != fp3.Folds[i].TailExclusiveEnd {
			t.Fatalf("inner split must ignore outer ValStart fold=%d", i)
		}
	}
	for i, f := range fp.Folds {
		of := hdr.Folds[i]
		if f.OuterTrainBegin != of.TrainBegin || f.OuterTrainEnd != of.TrainEnd || f.OuterValBegin != of.ValBegin || f.OuterValEnd != of.ValEnd {
			t.Fatalf("outer ranges rewritten fold %d", i)
		}
		if f.InnerValEnd > f.OuterTrainEnd || f.InnerTrainEnd > f.OuterTrainEnd {
			t.Fatal("inner not in outer train")
		}
		if f.InnerTrainEnd-f.InnerTrainBegin < cs.MinInnerTrainRows {
			t.Fatal("min inner train")
		}
		if f.TrainLastAt != rows[f.OuterTrainEnd-1].At {
			t.Fatal("TrainLastAt")
		}
	}
}
