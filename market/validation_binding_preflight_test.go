package market

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"trading_bot/forecast"
)

// TestValidationBindingPreflight1 is MODE A: chronology + year-open table.
// It does not compile a ValidationPlan and does not search for a binding.
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
	report("pin: HoldoutStartAt ValidationSpanBars FoldCount ExtraGapBars MinTrainRows")
	fmt.Fprintf(os.Stderr, "VALIDATION-BINDING-PREFLIGHT-1 MODE A N=%d H=%d\n", len(ats), h)
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
	spec, err := resolveIntendedResearchTargetSpec()
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
