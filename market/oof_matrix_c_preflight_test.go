package market

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/exchange"
	"trading_bot/forecast"
)

func TestOOFMatrixC_Preflight(t *testing.T) {
	if os.Getenv("OOF_MATRIX_C_PREFLIGHT") != "1" {
		t.Skip("set OOF_MATRIX_C_PREFLIGHT=1 to generate canonical OOF-MATRIX-C")
	}

	spec2, err := ResearchFeatureSpec2()
	if err != nil {
		t.Fatal(err)
	}
	sid, err := spec2.Identity()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := spec2.Plan.Identity()
	if err != nil {
		t.Fatal(err)
	}
	tid, err := spec2.Target.Identity()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := ResearchValidationPlanC()
	if err != nil {
		t.Fatal(err)
	}
	valID, err := plan.Identity()
	if err != nil {
		t.Fatal(err)
	}

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	tapeName, err := ResearchFeatureTape2FileName(spec2)
	if err != nil {
		t.Fatal(err)
	}
	labelName, err := ResearchLabelSetCFileName(spec2)
	if err != nil {
		t.Fatal(err)
	}
	tapePath := filepath.Join(root, "research", "tapes2", tapeName)
	labelPath := filepath.Join(root, "research", "labels", labelName)

	wantTape := "5ba899ebe5a1bf59fe7a0adb953a7c8c951883071f6d6312bfcd6b850aa9f372"
	wantLab := "491c7b8274a27291dbcb44bb00fe67cad1d6acb29525ba35fd2576ad70ef5b1e"
	wantPlanC := "b328000202d3512cf84cf823bb65c7f3ceeadb3a5136a36b76291f257a12536e"
	tapeDig, err := forecast.ParseDigestHex(wantTape)
	if err != nil {
		t.Fatal(err)
	}
	labDig, err := forecast.ParseDigestHex(wantLab)
	if err != nil {
		t.Fatal(err)
	}
	if valID.Digest.String() != wantPlanC {
		t.Fatalf("FAIL: ValidationPlan-C rules digest %s want %s", valID.Digest, wantPlanC)
	}

	rows, acc, err := forecast.BuildResearchDatasetFromTape2(tapePath, labelPath, spec2, forecast.ResearchDataset2Expect{
		MinFirstAt:   exchange.BinanceFuturesGenesisMs,
		TapeContent:  &tapeDig,
		LabelContent: &labDig,
	})
	if err != nil {
		t.Fatalf("FAIL: Dataset-C: %v", err)
	}
	ats := make([]int64, len(rows))
	for i := range rows {
		ats[i] = rows[i].At
	}
	compiled, err := forecast.CompileValidationPlan(ats, spec2.Primary.Timeframe, plan)
	if err != nil {
		t.Fatalf("FAIL: ValidationPlan-C: %v", err)
	}

	outDir := filepath.Join(root, "research", "oof")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(outDir, ResearchOOFMatrixFileName(spec2.Primary, tapeDig, labDig, valID.Digest))
	hdr, ft, matched, err := forecast.GenerateOOFMatrixFromTape2(tapePath, labelPath, outPath, spec2, forecast.ResearchDataset2Expect{
		MinFirstAt:   exchange.BinanceFuturesGenesisMs,
		TapeContent:  &tapeDig,
		LabelContent: &labDig,
	}, plan)
	if err != nil {
		t.Fatalf("FAIL: OOF-MATRIX-C: %v", err)
	}
	backH, got, backF, err := forecast.ReadOOFMatrix(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if backF.ContentDigest != ft.ContentDigest || !oofHeaderDevEqual(hdr, backH) {
		t.Fatal("FAIL: ContentDigest roundtrip")
	}
	if len(got) != compiled.DevelopmentEndIndex || hdr.DevRowCount != compiled.DevelopmentEndIndex {
		t.Fatalf("FAIL: matrix rows=%d compiled DevEnd=%d", len(got), compiled.DevelopmentEndIndex)
	}
	if hdr.FormatVersion != forecast.OOFMatrixFormatV1 {
		t.Fatalf("FAIL: format %s", hdr.FormatVersion)
	}
	if len(hdr.FeatureIDs) != forecast.Spec2FeatureWidth {
		t.Fatal("FAIL: FeatureIDs width")
	}
	ids := forecast.FeatureSpec2IDs()
	for i := range ids {
		if hdr.FeatureIDs[i] != ids[i] {
			t.Fatalf("FAIL: FeatureIDs[%d]", i)
		}
	}
	up, down, timeout := 0, 0, 0
	for i, r := range got {
		if r.At >= compiled.DevelopmentExclusiveEndAt || r.At >= compiled.HoldoutStartAt {
			t.Fatalf("FAIL: seam/holdout At %d", r.At)
		}
		if len(r.Features) != forecast.Spec2FeatureWidth {
			t.Fatal("width")
		}
		src := rows[i].Features
		for j := 0; j < forecast.Spec2FeatureWidth; j++ {
			if math.Float64bits(r.Features[j]) != math.Float64bits(src[j]) {
				t.Fatalf("FAIL: Float64bits row=%d col=%d", i, j)
			}
			if math.IsNaN(r.Features[j]) || math.IsInf(r.Features[j], 0) {
				t.Fatal("nonfinite")
			}
		}
		switch r.Outcome {
		case forecast.OutcomeUpFirst:
			up++
		case forecast.OutcomeDownFirst:
			down++
		case forecast.OutcomeTimeout:
			timeout++
		default:
			t.Fatalf("FAIL: outcome %q", r.Outcome)
		}
	}
	if got[0].At != rows[0].At {
		t.Fatal("FAIL: early train-only rows missing")
	}
	if got[len(got)-1].At != rows[compiled.DevelopmentEndIndex-1].At {
		t.Fatal("FAIL: last development row")
	}
	if len(hdr.Folds) != 4 {
		t.Fatal("FAIL: fold count")
	}
	for i, f := range hdr.Folds {
		if f != compiled.Folds[i] {
			t.Fatalf("FAIL: fold %d != compiled", i)
		}
		if f.TrainEnd > hdr.DevRowCount || f.ValEnd > hdr.DevRowCount {
			t.Fatalf("FAIL: fold %d not matrix-local", i)
		}
		if f.ValEnd-f.ValBegin == 17568 {
			t.Logf("note: fold %d ValN happens to equal span; still not a law", i)
		}
	}

	_, _, matched2, err := forecast.GenerateOOFMatrixFromTape2(tapePath, labelPath, outPath, spec2, forecast.ResearchDataset2Expect{
		MinFirstAt:   exchange.BinanceFuturesGenesisMs,
		TapeContent:  &tapeDig,
		LabelContent: &labDig,
	}, plan)
	if err != nil || !matched2 {
		t.Fatalf("FAIL: second generation MATCH matched=%v firstMatched=%v err=%v", matched2, matched, err)
	}

	t.Logf("OOF-MATRIX-C-PREFLIGHT")
	t.Logf("format=%s ContentDigest=%s", hdr.FormatVersion, ft.ContentDigest)
	t.Logf("spec2=%s plan=%s target=%s valplan=%s market=%s", sid.Digest, pid.Digest, tid.Digest, valID.Digest, spec2.Primary)
	t.Logf("tape=%s labels=%s", wantTape, wantLab)
	t.Logf("trainable=%d DevelopmentEndIndex=%d matrixRows=%d width=%d firstAt=%d lastAt=%d",
		acc.TrainableTotal, compiled.DevelopmentEndIndex, len(got), len(hdr.FeatureIDs), got[0].At, got[len(got)-1].At)
	t.Logf("development classes UP=%d DOWN=%d TIMEOUT=%d", up, down, timeout)
	t.Logf("FeatureIDs[0]=%s FeatureIDs[63]=%s", hdr.FeatureIDs[0], hdr.FeatureIDs[63])
	for i, f := range hdr.Folds {
		t.Logf("fold %d Train[%d:%d] n=%d Val[%d:%d] n=%d valAt=[%d,%d]",
			i, f.TrainBegin, f.TrainEnd, f.TrainEnd-f.TrainBegin,
			f.ValBegin, f.ValEnd, f.ValEnd-f.ValBegin, f.ValFirstAt, f.ValLastAt)
	}
	fmt.Fprintf(os.Stderr, "OOF-MATRIX-C PREFLIGHT GREEN digest=%s rows=%d matched=%v\n", ft.ContentDigest, len(got), matched2)
}

func oofHeaderDevEqual(a, b forecast.OOFHeader) bool {
	return a.FormatVersion == b.FormatVersion && a.DevRowCount == b.DevRowCount && a.TapeContent == b.TapeContent && a.LabelContent == b.LabelContent && a.ValPlan.Digest == b.ValPlan.Digest
}
