package market

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"trading_bot/exchange"
	"trading_bot/forecast"
)

func TestGenerateOOFMatrix_CanonicalArtifacts(t *testing.T) {
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
	valPlan, err := ResearchValidationPlan()
	if err != nil {
		t.Fatal(err)
	}
	valID, err := valPlan.Identity()
	if err != nil {
		t.Fatal(err)
	}
	_, _, tapeFt, err := forecast.ReadTape(tapePath, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, labFt, err := forecast.ReadLabelSet(labelPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(root, "research", "oof")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(outDir, ResearchOOFMatrixFileName(wantKey, tapeFt.ContentDigest, labFt.ContentDigest, valID.Digest))
	hdr, ft, _, err := forecast.GenerateOOFMatrix(tapePath, labelPath, outPath, forecast.ResearchDatasetExpect{
		Market:     wantKey,
		Plan:       planID.Digest,
		Target:     targetID.Digest,
		MinFirstAt: exchange.BinanceFuturesGenesisMs,
	}, valPlan)
	if err != nil {
		t.Fatal(err)
	}
	if hdr.Market != wantKey || hdr.AtUnit != forecast.OOFAtUnitUnixMs || hdr.FormatVersion != forecast.OOFMatrixFormatV1 {
		t.Fatalf("header identity %+v", hdr)
	}
	if hdr.DevRowCount != 221305 || ft.RowCount != 221305 {
		t.Fatalf("snapshot DevelopmentRowCount hdr=%d ft=%d", hdr.DevRowCount, ft.RowCount)
	}
	rows, _, err := forecast.BuildResearchDataset(tapePath, labelPath, forecast.ResearchDatasetExpect{
		Market:     wantKey,
		Plan:       planID.Digest,
		Target:     targetID.Digest,
		MinFirstAt: exchange.BinanceFuturesGenesisMs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 245173 {
		t.Fatalf("snapshot ResearchDataset rows=%d", len(rows))
	}
	ats := make([]int64, len(rows))
	for i := range rows {
		ats[i] = rows[i].At
	}
	compiled, err := forecast.CompileValidationPlan(ats, wantKey.Timeframe, valPlan)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.DevelopmentEndIndex != 221305 {
		t.Fatalf("compiled DevelopmentEndIndex=%d", compiled.DevelopmentEndIndex)
	}
	seam := compiled.HoldoutBeginIndex - compiled.DevelopmentEndIndex
	hold := len(rows) - compiled.HoldoutBeginIndex
	if seam != 24 || hold != 23844 {
		t.Fatalf("snapshot seam=%d holdout=%d", seam, hold)
	}
	_, got, _, err := forecast.ReadOOFMatrix(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 221305 {
		t.Fatalf("matrix rows %d", len(got))
	}
	if got[len(got)-1].At != rows[compiled.DevelopmentEndIndex-1].At {
		t.Fatal("last matrix row is not last legal development ResearchRow")
	}
	wall := time.Date(2025, 12, 31, 18, 0, 0, 0, time.UTC).UnixMilli()
	union := 0
	wantVal := []int{17564, 17561, 17568, 17566}
	if len(hdr.Folds) != 4 {
		t.Fatalf("folds %d", len(hdr.Folds))
	}
	for i, f := range hdr.Folds {
		cf := compiled.Folds[i]
		if f != cf {
			t.Fatalf("fold %d ranges != CompileValidationPlan", i)
		}
		if f.TrainEnd > hdr.DevRowCount || f.ValEnd > hdr.DevRowCount {
			t.Fatalf("fold %d not matrix-local", i)
		}
		n := f.ValEnd - f.ValBegin
		if n != wantVal[i] {
			t.Fatalf("fold %d val rows %d want %d", i, n, wantVal[i])
		}
		union += n
	}
	if union != 70259 {
		t.Fatalf("validation union %d", union)
	}
	for i := 1; i < len(hdr.Folds); i++ {
		if hdr.Folds[i-1].ValEnd > hdr.Folds[i].ValBegin && hdr.Folds[i].ValEnd > hdr.Folds[i-1].ValBegin {
			t.Fatal("validation overlap")
		}
	}
	for _, r := range got {
		if r.At >= wall {
			t.Fatalf("row At %d >= 2025-12-31 18:00 UTC", r.At)
		}
		if r.At >= compiled.HoldoutStartAt {
			t.Fatalf("holdout row present %d", r.At)
		}
		if len(r.Features) != len(hdr.FeatureIDs) {
			t.Fatal("feature width")
		}
	}
	if hdr.ValPlan.Digest != valID.Digest {
		t.Fatal("ValidationPlanDigest mismatch")
	}
	if hdr.PlanDigest != planID.Digest || hdr.TapeContent != tapeFt.ContentDigest || hdr.LabelContent != labFt.ContentDigest {
		t.Fatal("tape/label provenance mismatch")
	}
}
