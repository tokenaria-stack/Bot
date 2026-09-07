package market

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"trading_bot/forecast"
)

func TestValidationBindingPreflight1(t *testing.T) {
	rows, spec, tf := loadCanonicalTrainableRows(t)
	ats := make([]int64, len(rows))
	for i := range rows {
		ats[i] = rows[i].At
	}
	rows = nil
	if len(ats) == 0 {
		t.Fatal("FAIL: empty At[]")
	}
	for i := 1; i < len(ats); i++ {
		if ats[i] <= ats[i-1] {
			t.Fatalf("FAIL: At not strictly increasing at %d (%d after %d)", i, ats[i], ats[i-1])
		}
	}

	h := spec.HorizonBars
	horizon := make([]int64, len(ats))
	for i, at := range ats {
		end, err := forecast.HorizonEnd(at, tf, h)
		if err != nil {
			t.Fatal(err)
		}
		horizon[i] = end
	}

	report := func(format string, args ...any) { t.Logf(format, args...) }
	report("VALIDATION-BINDING-PREFLIGHT-1 MODE A")
	report("DATASET FirstAt=%d LastAt=%d TrainableRows=%d Timeframe=%s TargetH=%d", ats[0], ats[len(ats)-1], len(ats), tf, h)
	report("NEUTRAL UTC year opens (observations, not recommendations)")
	report("boundary_ms\tAt<B\tAt>=B\tdev_legal_H\tseam\tfirst_At_ge_B")

	first := time.UnixMilli(ats[0]).UTC()
	last := time.UnixMilli(ats[len(ats)-1]).UTC()
	for y := first.Year() + 1; y <= last.Year(); y++ {
		b := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
		if b > ats[len(ats)-1] {
			continue
		}
		lt := sort.Search(len(ats), func(i int) bool { return ats[i] >= b })
		dev, seam := 0, 0
		for i := 0; i < lt; i++ {
			if horizon[i] < b {
				dev++
			} else {
				seam++
			}
		}
		firstGE := int64(0)
		if lt < len(ats) {
			firstGE = ats[lt]
		}
		report("%d\t%d\t%d\t%d\t%d\t%d", b, lt, len(ats)-lt, dev, seam, firstGE)
	}

	report("READY FOR HUMAN BINDING")
	fmt.Fprintf(os.Stderr, "VALIDATION-BINDING-PREFLIGHT-1 MODE A N=%d H=%d\n", len(ats), h)
}

func TestCompileValidationPlan_CanonicalBTCGeometry(t *testing.T) {
	const (
		holdoutStartAt int64 = 1767225600000 // 2026-01-01 00:00:00 UTC
		devExclusive   int64 = 1767204000000 // 2025-12-31 18:00 UTC
		fold0Start     int64 = 1703959200000 // 2023-12-30 18:00 UTC
		fold0End       int64 = 1719770400000
		fold1End       int64 = 1735581600000
		fold2End       int64 = 1751392800000
		fold3End       int64 = 1767204000000
		train0Last     int64 = 1703936700000 // 2023-12-30 11:45 UTC
	)
	rows, spec, tf := loadCanonicalTrainableRows(t)
	ats := make([]int64, len(rows))
	for i := range rows {
		ats[i] = rows[i].At
	}
	plan, err := ResearchValidationPlan()
	if err != nil {
		t.Fatal(err)
	}
	if plan.TargetH != spec.HorizonBars {
		t.Fatalf("TargetH=%d spec.H=%d", plan.TargetH, spec.HorizonBars)
	}
	if plan.HoldoutStartAt != holdoutStartAt || plan.ValidationSpanBars != 17568 || plan.FoldCount != 4 ||
		plan.ExtraGapBars != 0 || plan.MinTrainRows != 35040 || plan.Timeframe != tf {
		t.Fatalf("binding %+v", plan)
	}
	got, err := forecast.CompileValidationPlan(ats, tf, plan)
	if err != nil {
		t.Fatal(err)
	}
	if got.HoldoutStartAt != holdoutStartAt || got.DevelopmentExclusiveEndAt != devExclusive {
		t.Fatalf("holdout/devEnd %d %d", got.HoldoutStartAt, got.DevelopmentExclusiveEndAt)
	}
	seam := got.HoldoutBeginIndex - got.DevelopmentEndIndex
	if seam != 24 {
		t.Fatalf("seam rows %d want 24", seam)
	}
	wantBounds := [][2]int64{{fold0Start, fold0End}, {fold0End, fold1End}, {fold1End, fold2End}, {fold2End, fold3End}}
	wantValN := []int{17564, 17561, 17568, 17566}
	wantTrainN := []int{151022, 168586, 186147, 203715}
	if len(got.Folds) != 4 {
		t.Fatalf("folds %d", len(got.Folds))
	}
	causal, err := plan.TotalCausalBars()
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range got.Folds {
		if f.ValBoundaryStartAt != wantBounds[i][0] || f.ValBoundaryEndAt != wantBounds[i][1] {
			t.Fatalf("fold %d bounds [%d,%d)", i, f.ValBoundaryStartAt, f.ValBoundaryEndAt)
		}
		if f.ValEnd-f.ValBegin != wantValN[i] || f.TrainEnd-f.TrainBegin != wantTrainN[i] {
			t.Fatalf("fold %d valN=%d trainN=%d", i, f.ValEnd-f.ValBegin, f.TrainEnd-f.TrainBegin)
		}
		he, err := forecast.HorizonEnd(f.TrainLastAt, tf, causal)
		if err != nil {
			t.Fatal(err)
		}
		if he >= f.ValBoundaryStartAt {
			t.Fatalf("fold %d HorizonEnd(TrainLast)=%d >= V=%d", i, he, f.ValBoundaryStartAt)
		}
		if f.ValEnd > got.HoldoutBeginIndex || f.TrainEnd > got.HoldoutBeginIndex {
			t.Fatalf("fold %d crosses holdout", i)
		}
	}
	if got.Folds[0].TrainLastAt != train0Last {
		t.Fatalf("fold0 TrainLastAt %d want %d", got.Folds[0].TrainLastAt, train0Last)
	}
	if got.HoldoutBeginIndex != 221329 {
		t.Fatalf("HoldoutBeginIndex %d", got.HoldoutBeginIndex)
	}
}

func loadCanonicalTrainableRows(t *testing.T) ([]forecast.ResearchRow, forecast.TargetSpec, string) {
	t.Helper()
	wantKey := ResearchMarketKey()
	plan, err := ResearchFeaturePlanMust(analysisLogicV2)
	if err != nil {
		t.Fatal(err)
	}
	planID, err := plan.Identity()
	if err != nil {
		t.Fatal(err)
	}
	spec, err := ResearchTargetSpec()
	if err != nil {
		t.Fatal(err)
	}
	targetID, err := spec.Identity()
	if err != nil {
		t.Fatal(err)
	}
	tapePath, labelPath, ok := canonicalResearchArtifactPaths(t, wantKey, planID.Digest, targetID.Digest)
	if !ok {
		t.Skip("canonical FeatureTape/LabelSet not present")
	}
	rows, _, err := forecast.BuildResearchDataset(tapePath, labelPath, forecast.ResearchDatasetExpect{
		Market:     wantKey,
		Plan:       planID.Digest,
		Target:     targetID.Digest,
		MinFirstAt: ResearchSourceStartMs(wantKey),
	})
	if err != nil {
		t.Fatal(err)
	}
	return rows, spec, wantKey.Timeframe
}
