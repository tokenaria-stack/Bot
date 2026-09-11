package market

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"trading_bot/forecast"
)

func TestCatBoostBrain1_Preflight(t *testing.T) {
	if os.Getenv("CATBOOST_BRAIN_1") != "1" {
		t.Skip("set CATBOOST_BRAIN_1=1 to run canonical four-fold CatBoost research")
	}
	spec := forecast.PinnedCatBoostSpec1()
	sid, err := spec.Identity()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	wantMatrix := "6793d8fe01a8533fc20ec372801b4c864396adde0c493aaeb9f3eed107c8e4e7"
	md, err := forecast.ParseDigestHex(wantMatrix)
	if err != nil {
		t.Fatal(err)
	}
	key := ResearchMarketKey()
	matrixPath := filepath.Join(root, "research", "oof",
		"BINANCE_BTCUSDT_FUTURES_PERP_15m_tape-5ba899ebe5a1bf59_labels-491c7b8274a27291_valplan-b328000202d3512c.oofmatrix")
	hdr, rows, ft, err := forecast.ReadOOFMatrix(matrixPath)
	if err != nil {
		t.Fatalf("OOF-MATRIX-C: %v", err)
	}
	if ft.ContentDigest != md {
		t.Fatalf("MATRIX_IDENTITY_MISMATCH %s", ft.ContentDigest)
	}
	plan, err := forecast.ResolveCatBoostFitPlan(hdr, rows, ft.ContentDigest, spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("CatBoostSpec1=%s FitPlan=%s", sid.Digest, plan.Digest())
	for i, fp := range plan.Folds {
		of := hdr.Folds[i]
		t.Logf("plan fold %d outerTrain[%d:%d] n=%d outerVal[%d:%d] n=%d innerTrainN=%d innerValN=%d innerValStart=%d tailExcl=%d TrainLastAt=%d",
			i, of.TrainBegin, of.TrainEnd, of.TrainEnd-of.TrainBegin, of.ValBegin, of.ValEnd, of.ValEnd-of.ValBegin,
			fp.InnerTrainEnd-fp.InnerTrainBegin, fp.InnerValEnd-fp.InnerValBegin, fp.InnerValStartAt, fp.TailExclusiveEnd, fp.TrainLastAt)
	}
	outDir := filepath.Join(root, "research", "catboost")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	envRaw, err := exec.Command("python3", "-c",
		"import platform,sys,numpy,catboost; print(sys.version.replace('\\n',' ')); print(catboost.__version__); print(numpy.__version__); print(platform.platform()); print(platform.machine())").CombinedOutput()
	if err != nil {
		t.Fatalf("env witness: %v %s", err, envRaw)
	}
	t.Logf("trainer env:\n%sGo %s", envRaw, runtime.Version())
	logitsPath := filepath.Join(outDir, ResearchCatBoostLogitsFileName(key, md, sid.Digest))
	logitsB := filepath.Join(outDir, "determinismB.catboostlogits")
	gotA, err := forecast.RunCatBoostBrain1(matrixPath, wantMatrix, "python3", root, filepath.Join(outDir, "runA.tmp"), logitsPath, spec)
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := forecast.RunCatBoostBrain1(matrixPath, wantMatrix, "python3", root, filepath.Join(outDir, "runB.tmp"), logitsB, spec)
	if err != nil {
		t.Fatal(err)
	}
	if gotA.LogitsDigest != gotB.LogitsDigest {
		t.Fatal("DETERMINISM_NOT_PROVEN logits digest")
	}
	for i := range gotA.SelectedN {
		if gotA.SelectedN[i] != gotB.SelectedN[i] || gotA.PortableDigest[i] != gotB.PortableDigest[i] {
			t.Fatalf("DETERMINISM_NOT_PROVEN N/portable A=%v B=%v", gotA, gotB)
		}
	}
	_, rowsA, lf, err := forecast.ReadCatBoostOOFLogits(logitsPath)
	if err != nil {
		t.Fatal(err)
	}
	_, rowsB, _, err := forecast.ReadCatBoostOOFLogits(logitsB)
	if err != nil {
		t.Fatal(err)
	}
	if lf.ContentDigest != gotA.LogitsDigest {
		t.Fatal("logits digest")
	}
	if len(rowsA) != len(rowsB) {
		t.Fatal("row count")
	}
	for i := range rowsA {
		for c := 0; c < 3; c++ {
			if math.Float64bits(rowsA[i].Logits[c]) != math.Float64bits(rowsB[i].Logits[c]) {
				t.Fatal("DETERMINISM_NOT_PROVEN Float64bits")
			}
		}
	}
	got := gotA
	lrows := rowsA
	union := 0
	for i, f := range hdr.Folds {
		union += f.ValEnd - f.ValBegin
		fp := plan.Folds[i]
		t.Logf("fold %d outerTrain[%d:%d] n=%d outerVal[%d:%d] n=%d innerTrainN=%d innerValN=%d innerValStart=%d tailExcl=%d N=%d innerLoss=%g portable=%s",
			i, f.TrainBegin, f.TrainEnd, f.TrainEnd-f.TrainBegin, f.ValBegin, f.ValEnd, f.ValEnd-f.ValBegin,
			fp.InnerTrainEnd-fp.InnerTrainBegin, fp.InnerValEnd-fp.InnerValBegin, fp.InnerValStartAt, fp.TailExclusiveEnd,
			got.SelectedN[i], got.InnerLoss[i], got.PortableDigest[i])
	}
	if len(lrows) != union {
		t.Fatalf("OOF rows %d union %d", len(lrows), union)
	}
	up, down, to := 0, 0, 0
	for _, r := range lrows {
		switch r.Outcome {
		case forecast.OutcomeUpFirst:
			up++
		case forecast.OutcomeDownFirst:
			down++
		case forecast.OutcomeTimeout:
			to++
		}
		if r.At >= hdr.HoldoutStart {
			t.Fatal("holdout")
		}
	}
	t.Logf("CATBOOST-BRAIN-1 spec=%s fitplan=%s matrix=%s logits=%s rows=%d first=%d last=%d UP=%d DOWN=%d TIMEOUT=%d go=%s DETERMINISM_PROVEN",
		sid.Digest, plan.Digest(), ft.ContentDigest, lf.ContentDigest, len(lrows), lrows[0].At, lrows[len(lrows)-1].At, up, down, to, runtime.Version())
	fmt.Fprintf(os.Stderr, "CATBOOST-BRAIN-1 PREFLIGHT digest=%s N=%v\n", lf.ContentDigest, got.SelectedN)
}
