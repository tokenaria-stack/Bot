package forecast

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func tinyValPlan(t *testing.T, holdout int64) ValidationPlan {
	t.Helper()
	return mustPlan(t, ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: holdout, ValidationSpanBars: 10,
		FoldCount: 1, ExtraGapBars: 0, MinTrainRows: 5,
	}, 2)
}

func writeTinyOOFWorld(t *testing.T, dir string, n int, mutTape func(i int, v []float64)) (tapePath, labelPath string, expect ResearchDatasetExpect, plan ValidationPlan) {
	t.Helper()
	start := int64(1_699_999_200_000)
	ats := seq15m(t, start, n)
	hdr := testTapeHeader()
	trows := make([]TapeRow, n)
	lrows := make([]LabelRow, n)
	outcomes := []TargetOutcome{OutcomeUpFirst, OutcomeDownFirst, OutcomeTimeout}
	for i := 0; i < n; i++ {
		v := []float64{1, 2, 3, 4}
		if mutTape != nil {
			mutTape(i, v)
		}
		trows[i] = TapeRow{At: ats[i], Ready: IsReady, Values: v}
		out := outcomes[i%3]
		hit := ats[i] + 1
		if out == OutcomeTimeout {
			hit = 0
		}
		lrows[i] = LabelRow{At: ats[i], Outcome: out, Reason: ReasonNone, HitAt: hit}
	}
	tapePath, tf := writeResearchTape(t, dir, hdr, trows)
	target := Digest{0x42}
	labelPath = writeResearchLabels(t, dir, hdr, tf, target, lrows)
	expect = testResearchExpect(hdr, target)
	plan = tinyValPlan(t, ats[70])
	return tapePath, labelPath, expect, plan
}

func TestOOFClassOrder_Contract(t *testing.T) {
	t.Parallel()
	want := [...]TargetOutcome{OutcomeUpFirst, OutcomeDownFirst, OutcomeTimeout}
	if OOFClassOrder != want {
		t.Fatalf("OOFClassOrder=%v want %v", OOFClassOrder, want)
	}
}

func TestOOFMatrixFormatV1_IsGeneric(t *testing.T) {
	t.Parallel()
	rt := reflect.TypeOf(OOFRow{})
	if _, ok := rt.FieldByName("FoldID"); ok {
		t.Fatal("per-row FoldID is forbidden")
	}
	if _, ok := rt.FieldByName("Logits"); ok {
		t.Fatal("logits must not live on OOFRow")
	}
	ht := reflect.TypeOf(OOFHeader{})
	for _, banned := range []string{"Signal9", "Horizon24", "Logistic", "CatBoost", "FeatureSpecIdentity"} {
		if _, ok := ht.FieldByName(banned); ok {
			t.Fatalf("header field %s would lie or fork identity", banned)
		}
	}
}

func TestExportPythonGoldenOOFMatrix(t *testing.T) {
	dir := t.TempDir()
	tape, labels, expect, plan := writeTinyOOFWorld(t, dir, 80, nil)
	root := filepath.Join("..", "research", "modelfit", "testdata")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "tiny.oofmatrix")
	digestPath := filepath.Join(root, "tiny.contentdigest")
	tmp := filepath.Join(dir, "m.oofmatrix")
	_, ft, _, err := GenerateOOFMatrix(tape, labels, tmp, expect, plan)
	if err != nil {
		t.Fatal(err)
	}
	want := ft.ContentDigest.String()
	if _, err := os.Stat(out); err != nil {
		data, err := os.ReadFile(tmp)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(digestPath, []byte(want+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != want {
		t.Fatalf("python golden digest drift: file=%s generate=%s", strings.TrimSpace(string(got)), want)
	}
}

func TestGenerateOOFMatrix_PhysicalSplitExcludesSeamHoldout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, expect, plan := writeTinyOOFWorld(t, dir, 80, nil)
	out := filepath.Join(dir, "m.oofmatrix")
	hdr, ft, matched, err := GenerateOOFMatrix(tape, labels, out, expect, plan)
	if err != nil || matched {
		t.Fatalf("generate: matched=%v err=%v", matched, err)
	}
	rows, acc, err := BuildResearchDataset(tape, labels, expect)
	if err != nil {
		t.Fatal(err)
	}
	ats := make([]int64, len(rows))
	for i := range rows {
		ats[i] = rows[i].At
	}
	compiled, err := CompileValidationPlan(ats, expect.Market.Timeframe, plan)
	if err != nil {
		t.Fatal(err)
	}
	if hdr.DevRowCount != compiled.DevelopmentEndIndex || ft.RowCount != compiled.DevelopmentEndIndex {
		t.Fatalf("dev rows hdr=%d ft=%d compiled=%d acc=%d", hdr.DevRowCount, ft.RowCount, compiled.DevelopmentEndIndex, acc.TrainableTotal)
	}
	gotH, gotRows, _, err := ReadOOFMatrix(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotRows) != compiled.DevelopmentEndIndex {
		t.Fatalf("matrix rows %d want %d", len(gotRows), compiled.DevelopmentEndIndex)
	}
	if gotRows[len(gotRows)-1].At != rows[compiled.DevelopmentEndIndex-1].At {
		t.Fatalf("last matrix At mismatch")
	}
	for _, r := range gotRows {
		if r.At >= compiled.DevelopmentExclusiveEndAt {
			t.Fatalf("seam/holdout At leaked %d", r.At)
		}
		if r.At >= compiled.HoldoutStartAt {
			t.Fatalf("holdout At leaked %d", r.At)
		}
	}
	seam := compiled.HoldoutBeginIndex - compiled.DevelopmentEndIndex
	hold := len(rows) - compiled.HoldoutBeginIndex
	if seam <= 0 || hold <= 0 {
		t.Fatalf("fixture must include seam and holdout seam=%d hold=%d", seam, hold)
	}
	if gotH.Folds[0].ValEnd > gotH.DevRowCount || gotH.Folds[0].TrainEnd > gotH.DevRowCount {
		t.Fatalf("fold range not matrix-local")
	}
	if gotH.AtUnit != OOFAtUnitUnixMs || gotH.FormatVersion != OOFMatrixFormatV1 {
		t.Fatalf("wire contract %+v", gotH)
	}
}

func TestGenerateOOFMatrix_ExistingSlot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, expect, plan := writeTinyOOFWorld(t, dir, 80, nil)
	out := filepath.Join(dir, "m.oofmatrix")
	_, _, matched, err := GenerateOOFMatrix(tape, labels, out, expect, plan)
	if err != nil || matched {
		t.Fatalf("first write: matched=%v err=%v", matched, err)
	}
	st1, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	_, _, matched, err = GenerateOOFMatrix(tape, labels, out, expect, plan)
	if err != nil || !matched {
		t.Fatalf("match: matched=%v err=%v", matched, err)
	}
	st2, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if st1.ModTime() != st2.ModTime() || st1.Size() != st2.Size() {
		t.Fatalf("MATCH rewrote the slot")
	}
	other := mustPlan(t, ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: plan.HoldoutStartAt, ValidationSpanBars: 11,
		FoldCount: 1, ExtraGapBars: 0, MinTrainRows: 5,
	}, 2)
	_, _, _, err = GenerateOOFMatrix(tape, labels, out, expect, other)
	if err == nil || !strings.Contains(err.Error(), "refuse overwrite") {
		t.Fatalf("want refuse different artifact, got %v", err)
	}
	bad := filepath.Join(dir, "bad.oofmatrix")
	if err := os.WriteFile(bad, []byte("not-jsonl\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = GenerateOOFMatrix(tape, labels, bad, expect, plan)
	if err == nil || !strings.Contains(err.Error(), "refuse existing") {
		t.Fatalf("want refuse corrupted, got %v", err)
	}
}

func TestGenerateOOFMatrix_IdentityAndProvenance(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, expect, plan := writeTinyOOFWorld(t, dir, 80, nil)
	out := filepath.Join(dir, "a.oofmatrix")
	h1, f1, _, err := GenerateOOFMatrix(tape, labels, out, expect, plan)
	if err != nil {
		t.Fatal(err)
	}
	dir2 := t.TempDir()
	tape2, labels2, expect2, plan2 := writeTinyOOFWorld(t, dir2, 80, nil)
	out2 := filepath.Join(dir2, "b.oofmatrix")
	h2, f2, _, err := GenerateOOFMatrix(tape2, labels2, out2, expect2, plan2)
	if err != nil {
		t.Fatal(err)
	}
	if f1.ContentDigest != f2.ContentDigest || h1.ValPlan.Digest != h2.ValPlan.Digest {
		t.Fatalf("identical worlds must match OOF and ValidationPlan digests")
	}
	dir3 := t.TempDir()
	tape3, labels3, expect3, plan3 := writeTinyOOFWorld(t, dir3, 80, func(i int, v []float64) {
		if i == 0 {
			v[0] = 9
		}
	})
	out3 := filepath.Join(dir3, "c.oofmatrix")
	h3, f3, _, err := GenerateOOFMatrix(tape3, labels3, out3, expect3, plan3)
	if err != nil {
		t.Fatal(err)
	}
	if h3.ValPlan.Digest != h1.ValPlan.Digest {
		t.Fatalf("ValidationPlanDigest must be rules-only")
	}
	if f3.ContentDigest == f1.ContentDigest {
		t.Fatalf("different tape content must change OOF ContentDigest")
	}
	other := mustPlan(t, ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: plan.HoldoutStartAt, ValidationSpanBars: 11,
		FoldCount: 1, ExtraGapBars: 0, MinTrainRows: 5,
	}, 2)
	out4 := filepath.Join(t.TempDir(), "d.oofmatrix")
	h4, f4, _, err := GenerateOOFMatrix(tape, labels, out4, expect, other)
	if err != nil {
		t.Fatal(err)
	}
	if h4.ValPlan.Digest == h1.ValPlan.Digest {
		t.Fatalf("changed ValidationPlan must change ValidationPlanDigest")
	}
	if f4.ContentDigest == f1.ContentDigest {
		t.Fatalf("changed geometry must change OOF ContentDigest")
	}
	badExpect := expect
	badExpect.Plan[0] ^= 0xff
	_, _, _, err = GenerateOOFMatrix(tape, labels, filepath.Join(t.TempDir(), "e.oofmatrix"), badExpect, plan)
	if err == nil {
		t.Fatal("want refuse tape provenance mismatch")
	}
	badT := expect
	badT.Target[0] ^= 0xff
	_, _, _, err = GenerateOOFMatrix(tape, labels, filepath.Join(t.TempDir(), "f.oofmatrix"), badT, plan)
	if err == nil {
		t.Fatal("want refuse label provenance mismatch")
	}
}

func TestOOFSourceSnap_TOCTOUEqual(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, _, _ := writeTinyOOFWorld(t, dir, 80, nil)
	a, err := readOOFSources(tape, labels)
	if err != nil {
		t.Fatal(err)
	}
	b, err := readOOFSources(tape, labels)
	if err != nil {
		t.Fatal(err)
	}
	if !a.equal(b) {
		t.Fatal("identical sequential reads must match")
	}
	b.TapeContent[0] ^= 1
	if a.equal(b) {
		t.Fatal("changed tape ContentDigest must refuse equality")
	}
	b = a
	b.labFt.ContentDigest[0] ^= 1
	if a.equal(b) {
		t.Fatal("changed label ContentDigest must refuse equality")
	}
}

func TestValidateOOFExportRow_Contract(t *testing.T) {
	t.Parallel()
	ok := ResearchRow{At: 1, Features: []float64{1, 2, 3, 4}, Outcome: OutcomeUpFirst}
	if err := validateOOFExportRow(4, ok); err != nil {
		t.Fatal(err)
	}
	if err := validateOOFExportRow(3, ok); err == nil {
		t.Fatal("want width refuse")
	}
	nan := ok
	nan.Features = []float64{1, math.NaN(), 3, 4}
	if err := validateOOFExportRow(4, nan); err == nil {
		t.Fatal("want NaN refuse")
	}
	pos := ok
	pos.Features = []float64{1, math.Inf(1), 3, 4}
	if err := validateOOFExportRow(4, pos); err == nil {
		t.Fatal("want +Inf refuse")
	}
	neg := ok
	neg.Features = []float64{1, math.Inf(-1), 3, 4}
	if err := validateOOFExportRow(4, neg); err == nil {
		t.Fatal("want -Inf refuse")
	}
	amb := ok
	amb.Outcome = OutcomeAmbiguous
	if err := validateOOFExportRow(4, amb); err == nil {
		t.Fatal("want AMBIGUOUS refuse")
	}
	if err := validateOOFExportRow(4, ResearchRow{At: 1, Features: []float64{1, 2, 3, 4}, Outcome: "NOPE"}); err == nil {
		t.Fatal("want illegal outcome refuse")
	}
}

func TestAssembleOOFMatrix_CopyAndOutcomeRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, expect, plan := writeTinyOOFWorld(t, dir, 80, nil)
	rows, _, err := BuildResearchDataset(tape, labels, expect)
	if err != nil {
		t.Fatal(err)
	}
	src, err := readOOFSources(tape, labels)
	if err != nil {
		t.Fatal(err)
	}
	hdr, exported, err := assembleOOFMatrix(rows, src, plan)
	if err != nil {
		t.Fatal(err)
	}
	orig := exported[0].Features[0]
	rows[0].Features[0] = 12345
	if exported[0].Features[0] != orig {
		t.Fatal("exported features aliased ResearchRow buffer")
	}
	out := filepath.Join(dir, "rt.oofmatrix")
	want := hashOOFMatrix(hdr, exported)
	if _, err := writeOOFMatrix(out, hdr, exported, want); err != nil {
		t.Fatal(err)
	}
	_, got, _, err := ReadOOFMatrix(out)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[TargetOutcome]bool{}
	for _, r := range got {
		seen[r.Outcome] = true
	}
	if !seen[OutcomeUpFirst] || !seen[OutcomeDownFirst] || !seen[OutcomeTimeout] {
		t.Fatalf("outcome round-trip missing classes %+v", seen)
	}
}

func TestValidateOOFFolds_MatrixLocal(t *testing.T) {
	t.Parallel()
	c := CompiledValidationPlan{
		DevelopmentEndIndex: 10,
		Folds: []CompiledFold{
			{TrainBegin: 0, TrainEnd: 6, ValBegin: 6, ValEnd: 8},
			{TrainBegin: 0, TrainEnd: 7, ValBegin: 8, ValEnd: 10},
		},
	}
	if err := validateOOFFolds(c, 10); err != nil {
		t.Fatal(err)
	}
	c.Folds[0].ValEnd = 11
	if err := validateOOFFolds(c, 10); err == nil {
		t.Fatal("want range past DevelopmentRowCount")
	}
	c.Folds[0].ValEnd = 9
	c.Folds[1].ValBegin = 7
	if err := validateOOFFolds(c, 10); err == nil {
		t.Fatal("want overlapping validation refuse")
	}
}
