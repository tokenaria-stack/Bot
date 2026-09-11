package forecast

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func haveCatBoost(t *testing.T) (python, repo string, ok bool) {
	t.Helper()
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	python = "python3"
	cmd := exec.Command(python, "-c", "import catboost")
	if err := cmd.Run(); err != nil {
		return python, repo, false
	}
	return python, repo, true
}

func TestRunCatBoostBrain1_WrongMatrixDigest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, spec2, plan, _ := writeOOFWorld2(t, dir, 400, 360, 4, 20, 5)
	out := filepath.Join(dir, "m.oofmatrix")
	_, _, _, err := GenerateOOFMatrixFromTape2(tape, labels, out, spec2, ResearchDataset2Expect{}, plan)
	if err != nil {
		t.Fatal(err)
	}
	bad := Digest{0xff}
	_, err = RunCatBoostBrain1(out, bad.String(), "python3", dir, dir, filepath.Join(dir, "l.logits"), testCatBoostSpecTiny())
	if err == nil {
		t.Fatal("want MATRIX_IDENTITY_MISMATCH")
	}
}

func writeOOFWorld2CatBoost(t *testing.T, dir string, n, holdoutIdx int, folds, span, minTrain int) (tapePath, labelPath string, spec2 FeatureSpec2, plan ValidationPlan) {
	t.Helper()
	spec2 = testSpec2(t)
	start := int64(1_699_999_200_000)
	ats := seq15m(t, start, n)
	bars := make([]CanonicalClosedBar, n)
	readyAt := map[int64]bool{}
	for i, at := range ats {
		bars[i] = CanonicalClosedBar{OpenTime: at, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1}
	}
	readyAt[ats[0]] = false
	path := filepath.Join(dir, "t.featuretape2")
	h := tape2HeaderForPrimary(t, spec2, bars)
	w, err := CreateTape2Writer(path, h)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range bars {
		if readyAt[b.OpenTime] == false && i == 0 {
			if err := w.WriteRow(b.OpenTime, NotReady, ReasonPrimaryWarmup, nil); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := w.WriteRow(b.OpenTime, IsReady, "", dummyVec2(1.5+float64(i)*0.01)); err != nil {
			t.Fatal(err)
		}
	}
	tf, err := w.Finish()
	if err != nil {
		t.Fatal(err)
	}
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
	labelPath = writeResearchLabels2(t, dir, spec2, h, tf, lrows)
	plan = mustPlan(t, ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: ats[holdoutIdx], ValidationSpanBars: span,
		FoldCount: folds, ExtraGapBars: 0, MinTrainRows: minTrain,
	}, spec2.Target.HorizonBars)
	return path, labelPath, spec2, plan
}

func TestRunCatBoostBrain1_TinyFourFold(t *testing.T) {
	python, repo, ok := haveCatBoost(t)
	if !ok {
		t.Skip("catboost not installed")
	}
	dir := t.TempDir()
	tape, labels, spec2, plan := writeOOFWorld2CatBoost(t, dir, 400, 360, 4, 20, 5)
	matrix := filepath.Join(dir, "m.oofmatrix")
	hdr, ft, _, err := GenerateOOFMatrixFromTape2(tape, labels, matrix, spec2, ResearchDataset2Expect{}, plan)
	if err != nil {
		t.Fatal(err)
	}
	logits := filepath.Join(dir, "out.logits")
	work := filepath.Join(dir, "work")
	got, err := RunCatBoostBrain1(matrix, ft.ContentDigest.String(), python, repo, work, logits, testCatBoostSpecTiny())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SelectedN) != 4 || got.LogitsRows <= 0 {
		t.Fatalf("%+v", got)
	}
	for _, n := range got.SelectedN {
		if n <= 0 || n >= testCatBoostSpecTiny().MaxIterations {
			t.Fatalf("N=%v", got.SelectedN)
		}
	}
	lh, lrows, lf, err := ReadCatBoostOOFLogits(logits)
	if err != nil {
		t.Fatal(err)
	}
	if lf.ContentDigest != got.LogitsDigest || lh.MatrixDigest != ft.ContentDigest {
		t.Fatal("logits identity")
	}
	union := 0
	for _, f := range hdr.Folds {
		union += f.ValEnd - f.ValBegin
	}
	if len(lrows) != union {
		t.Fatalf("logits rows %d union %d", len(lrows), union)
	}
	seen := map[int64]int{}
	for _, r := range lrows {
		seen[r.At]++
		if r.At >= hdr.HoldoutStart {
			t.Fatal("holdout")
		}
	}
	for a, c := range seen {
		if c != 1 {
			t.Fatalf("At %d count %d", a, c)
		}
	}
	got2, err := RunCatBoostBrain1(matrix, ft.ContentDigest.String(), python, repo, filepath.Join(dir, "work2"), logits, testCatBoostSpecTiny())
	if err != nil {
		t.Fatal(err)
	}
	if !got2.Matched {
		t.Fatal("second run must MATCH")
	}
	if got2.LogitsDigest != got.LogitsDigest {
		t.Fatal("MATCH digest")
	}
	other := testCatBoostSpecTiny()
	other.MaxIterations = 7
	_, err = RunCatBoostBrain1(matrix, ft.ContentDigest.String(), python, repo, filepath.Join(dir, "work3"), logits, other)
	if err == nil {
		t.Fatal("want refuse conflicting spec")
	}
	logitsB := filepath.Join(dir, "outB.logits")
	gotB, err := RunCatBoostBrain1(matrix, ft.ContentDigest.String(), python, repo, filepath.Join(dir, "workB"), logitsB, testCatBoostSpecTiny())
	if err != nil {
		t.Fatal(err)
	}
	if gotB.Matched || gotB.LogitsDigest != got.LogitsDigest {
		t.Fatalf("independent run identity %+v vs %+v", gotB, got)
	}
	for i := range got.SelectedN {
		if got.SelectedN[i] != gotB.SelectedN[i] || got.PortableDigest[i] != gotB.PortableDigest[i] {
			t.Fatalf("DETERMINISM_NOT_PROVEN N/portable %v %v", got, gotB)
		}
	}
	_, arows, _, err := ReadCatBoostOOFLogits(logits)
	if err != nil {
		t.Fatal(err)
	}
	_, brows, _, err := ReadCatBoostOOFLogits(logitsB)
	if err != nil {
		t.Fatal(err)
	}
	if len(arows) != len(brows) {
		t.Fatal("row count")
	}
	for i := range arows {
		if arows[i].At != brows[i].At {
			t.Fatal("At")
		}
		for c := 0; c < 3; c++ {
			if math.Float64bits(arows[i].Logits[c]) != math.Float64bits(brows[i].Logits[c]) {
				t.Fatal("DETERMINISM_NOT_PROVEN logits bits")
			}
		}
	}
}

func TestRunCatBoostBrain1_IncompleteOfficialRefuses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tape, labels, spec2, plan := writeOOFWorld2CatBoost(t, dir, 400, 360, 4, 20, 5)
	matrix := filepath.Join(dir, "m.oofmatrix")
	_, ft, _, err := GenerateOOFMatrixFromTape2(tape, labels, matrix, spec2, ResearchDataset2Expect{}, plan)
	if err != nil {
		t.Fatal(err)
	}
	logits := filepath.Join(dir, "partial.logits")
	if err := os.WriteFile(logits, []byte("{\"kind\":\"header\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = RunCatBoostBrain1(matrix, ft.ContentDigest.String(), "python3", dir, dir, logits, testCatBoostSpecTiny())
	if err == nil {
		t.Fatal("want refuse incomplete official logits")
	}
}

func TestPythonTrainerHasNoEvalSet(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "research", "catboost1", "__main__.py"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if containsWord(s, "early_stopping") || containsWord(s, "od_wait") {
		t.Fatal("early stopping in trainer")
	}
}

func containsWord(s, w string) bool {
	return len(s) > 0 && (stringIndex(s, w) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
