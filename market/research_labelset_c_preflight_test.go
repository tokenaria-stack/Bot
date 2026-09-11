package market

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/exchange"
	"trading_bot/forecast"
)

func TestDumpLabelSetFromTape2_ResearchC(t *testing.T) {
	if os.Getenv("LABELSET_C_PREFLIGHT") != "1" {
		t.Skip("set LABELSET_C_PREFLIGHT=1 to dump/read canonical LabelSet-C from FeatureTape2")
	}

	spec2, err := ResearchFeatureSpec2()
	if err != nil {
		t.Fatalf("FAIL: FeatureSpec2: %v", err)
	}
	tid, err := spec2.Target.Identity()
	if err != nil {
		t.Fatal(err)
	}
	wantTarget := "3d4e57acfd061983c78feca7bbc4e5ad62f788a8e179ebbbdaf32205761c8922"
	if tid.Digest.String() != wantTarget {
		t.Fatalf("FAIL: Target C digest %s want %s", tid.Digest, wantTarget)
	}
	if spec2.Target.HorizonBars != 72 || spec2.Target.UpperATRMultiple != 2.0 || spec2.Target.LowerATRMultiple != 2.0 {
		t.Fatalf("FAIL: Target C H/U/L %+v", spec2.Target)
	}

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	tapeName, err := ResearchFeatureTape2FileName(spec2)
	if err != nil {
		t.Fatal(err)
	}
	tapePath := filepath.Join(root, "research", "tapes2", tapeName)
	if _, err := os.Stat(tapePath); err != nil {
		t.Fatalf("FAIL: canonical FeatureTape2 missing %s (run FEATURE_TAPE2_PREFLIGHT=1 first): %v", tapePath, err)
	}

	labelName, err := ResearchLabelSetCFileName(spec2)
	if err != nil {
		t.Fatal(err)
	}
	labelPath := filepath.Join(root, "research", "labels", labelName)
	th, trows, tf, err := forecast.ReadTape2(tapePath)
	if err != nil {
		t.Fatalf("FAIL: ReadTape2: %v", err)
	}

	finerKey := th.Primary
	finerKey.Timeframe = spec2.Target.FinerTimeframe
	if !th.Primary.SameFamily(finerKey) || finerKey.Timeframe == th.Primary.Timeframe {
		t.Fatalf("FAIL: finer family primary=%s finer=%s", th.Primary, finerKey)
	}

	expect := &forecast.LabelExpect{
		Market:      &th.Primary,
		Target:      &tid.Digest,
		TapePlan:    &th.PlanDigest,
		TapeSource:  &th.PrimarySource,
		TapeContent: &tf.ContentDigest,
	}

	if _, err := os.Stat(labelPath); err == nil {
		t.Log("MATCH existing LabelSet-C; skip dump")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	} else {
		if err := os.MkdirAll(filepath.Dir(labelPath), 0o755); err != nil {
			t.Fatal(err)
		}
		primary := loadResearchClosedBarsTF(t, th.Primary.Timeframe)
		finer := loadResearchClosedBarsTF(t, spec2.Target.FinerTimeframe)
		if primary[0].OpenTime < exchange.BinanceFuturesGenesisMs {
			t.Fatalf("FAIL: primary FirstAt=%d < genesis", primary[0].OpenTime)
		}
		if finer[0].OpenTime < exchange.BinanceFuturesGenesisMs {
			t.Fatalf("FAIL: finer FirstAt=%d < genesis", finer[0].OpenTime)
		}
		if err := DumpLabelSetFromTape2(labelPath, tapePath, spec2, primary, finer, finerKey, expect); err != nil {
			t.Fatalf("FAIL: DumpLabelSetFromTape2: %v", err)
		}
	}

	lh, lrows, lf, err := forecast.ReadLabelSet(labelPath, expect)
	if err != nil {
		t.Fatalf("FAIL: readback: %v", err)
	}
	if lh.FormatVersion != forecast.LabelSetFormatV2 {
		t.Fatalf("FAIL: format %q", lh.FormatVersion)
	}

	outcomes := map[forecast.TargetOutcome]int{}
	reasons := map[forecast.LabelReason]int{}
	for _, row := range lrows {
		outcomes[row.Outcome]++
		reasons[row.Reason]++
	}
	sumOut := 0
	for _, n := range outcomes {
		sumOut += n
	}
	sumReason := 0
	for _, n := range reasons {
		sumReason += n
	}

	lockstep := lf.RowCount == tf.RowCount && len(lrows) == len(trows) && sumOut == lf.RowCount && sumReason == lf.RowCount
	if lockstep {
		for i := range trows {
			if lrows[i].At != trows[i].At {
				lockstep = false
				break
			}
		}
	}
	if !lockstep {
		t.Fatalf("FAIL: At lockstep tape=%d labels=%d footer=%d", tf.RowCount, len(lrows), lf.RowCount)
	}
	if lh.Market != th.Primary || lh.FinerMarket != finerKey {
		t.Fatal("FAIL: market keys")
	}
	if lh.TargetDigest != tid.Digest {
		t.Fatal("FAIL: TargetDigest")
	}
	if lh.FeatureTapePlanDigest != th.PlanDigest || lh.FeatureTapeContentDigest != tf.ContentDigest {
		t.Fatal("FAIL: candidate-source binding")
	}
	if lh.FeatureTapeSourceRangeDigest != th.PrimarySource {
		t.Fatal("FAIL: primary provenance must be Tape2 PrimarySource")
	}

	t.Logf("LABEL-SET-C-PREFLIGHT")
	t.Logf("tape path=%s rows=%d ready=%d not_ready=%d content=%s", tapePath, tf.RowCount, tf.ReadyCount, tf.NotReadyCount, tf.ContentDigest)
	t.Logf("spec2=%s plan=%s target=%s H=%d U=%g L=%g", th.SpecDigest, th.PlanDigest, tid.Digest, spec2.Target.HorizonBars, spec2.Target.UpperATRMultiple, spec2.Target.LowerATRMultiple)
	t.Logf("primary_src=%s finer_src=%s finer_windows=%d finer_tf=%s", lh.FeatureTapeSourceRangeDigest, lf.FinerSourceDigest, lf.FinerWindowCount, lh.FinerMarket.Timeframe)
	t.Logf("labels path=%s rows=%d content=%s", labelPath, lf.RowCount, lf.ContentDigest)
	t.Logf("OUTCOMES %v", outcomes)
	t.Logf("REASONS %v", reasons)
	fmt.Fprintf(os.Stderr, "LABEL-SET-C-PREFLIGHT GREEN rows=%d truncated=%d dual_hit_windows=%d\n",
		lf.RowCount, reasons[forecast.ReasonTruncatedHorizon], lf.FinerWindowCount)
}
