package market

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/exchange"
	"trading_bot/forecast"
)

func TestDatasetC_ValidationPlanC_Preflight(t *testing.T) {
	if os.Getenv("DATASET_C_PREFLIGHT") != "1" {
		t.Skip("set DATASET_C_PREFLIGHT=1 to join canonical Tape2+LabelSet-C and compile ValidationPlan-C")
	}

	spec2, err := ResearchFeatureSpec2()
	if err != nil {
		t.Fatal(err)
	}
	tid, err := spec2.Target.Identity()
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
	tapeDig, err := forecast.ParseDigestHex(wantTape)
	if err != nil {
		t.Fatal(err)
	}
	labDig, err := forecast.ParseDigestHex(wantLab)
	if err != nil {
		t.Fatal(err)
	}

	rows, acc, err := forecast.BuildResearchDatasetFromTape2(tapePath, labelPath, spec2, forecast.ResearchDataset2Expect{
		MinFirstAt:   exchange.BinanceFuturesGenesisMs,
		TapeContent:  &tapeDig,
		LabelContent: &labDig,
	})
	if err != nil {
		t.Fatalf("FAIL: Dataset-C: %v", err)
	}
	if acc.TotalCandidates != acc.FeatureNotReady+acc.ExcludedTotal+acc.TrainableTotal {
		t.Fatalf("FAIL: partition %+v", acc)
	}
	if acc.TrainableTotal != acc.TrainableUP+acc.TrainableDOWN+acc.TrainableTIMEOUT || len(rows) != acc.TrainableTotal {
		t.Fatalf("FAIL: trainable %+v rows=%d", acc, len(rows))
	}

	ats := make([]int64, len(rows))
	for i := range rows {
		ats[i] = rows[i].At
		if len(rows[i].Features) != forecast.Spec2FeatureWidth {
			t.Fatal("width")
		}
	}

	plan, err := ResearchValidationPlanC()
	if err != nil {
		t.Fatal(err)
	}
	if plan.TargetH != 72 || plan.ValidationSpanBars != 17568 || plan.MinTrainRows != 35040 {
		t.Fatalf("FAIL: policy %+v", plan)
	}
	compiled, err := forecast.CompileValidationPlan(ats, spec2.Primary.Timeframe, plan)
	if err != nil {
		t.Fatalf("FAIL: ValidationPlan-C: %v", err)
	}
	causal, err := plan.TotalCausalBars()
	if err != nil {
		t.Fatal(err)
	}
	id, err := plan.Identity()
	if err != nil {
		t.Fatal(err)
	}
	seam := compiled.HoldoutBeginIndex - compiled.DevelopmentEndIndex
	holdoutN := len(ats) - compiled.HoldoutBeginIndex

	t.Logf("DATASET-C-PREFLIGHT")
	t.Logf("spec2=%s plan=%s target=%s market=%s", sid.Digest, pid.Digest, tid.Digest, spec2.Primary)
	t.Logf("tape=%s labels=%s", wantTape, wantLab)
	t.Logf("Candidates=%d FeatureNotReady=%d Excluded=%d Trainable=%d", acc.TotalCandidates, acc.FeatureNotReady, acc.ExcludedTotal, acc.TrainableTotal)
	t.Logf("ExcludedByReason %v", acc.ExcludedByReason)
	t.Logf("Trainable UP=%d DOWN=%d TIMEOUT=%d first=%d last=%d", acc.TrainableUP, acc.TrainableDOWN, acc.TrainableTIMEOUT, rows[0].At, rows[len(rows)-1].At)
	t.Logf("VALIDATION-PLAN-C digest=%s TargetH=%d TotalCausalBars=%d HoldoutStartAt=%d", id.Digest, plan.TargetH, causal, plan.HoldoutStartAt)
	t.Logf("DevelopmentEndIndex=%d HoldoutBeginIndex=%d seam=%d holdoutN=%d", compiled.DevelopmentEndIndex, compiled.HoldoutBeginIndex, seam, holdoutN)
	for i, f := range compiled.Folds {
		t.Logf("fold %d Train[%d:%d] n=%d Val[%d:%d] n=%d valClock=[%d,%d) valObs=[%d,%d]",
			i, f.TrainBegin, f.TrainEnd, f.TrainEnd-f.TrainBegin,
			f.ValBegin, f.ValEnd, f.ValEnd-f.ValBegin,
			f.ValBoundaryStartAt, f.ValBoundaryEndAt, f.ValFirstAt, f.ValLastAt)
	}
	fmt.Fprintf(os.Stderr, "DATASET-C + VALIDATION-PLAN-C PREFLIGHT GREEN trainable=%d folds=%d\n", acc.TrainableTotal, len(compiled.Folds))
}
