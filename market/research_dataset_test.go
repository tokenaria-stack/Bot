package market

import (
	"os"
	"path/filepath"
	"testing"

	"trading_bot/exchange"
	"trading_bot/forecast"
)

func TestBuildResearchDataset_CanonicalArtifacts(t *testing.T) {
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

	rows, acc, err := forecast.BuildResearchDataset(tapePath, labelPath, forecast.ResearchDatasetExpect{
		Market:     wantKey,
		Plan:       planID.Digest,
		Target:     targetID.Digest,
		MinFirstAt: exchange.BinanceFuturesGenesisMs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if acc.TotalCandidates != 245251 || acc.FeatureNotReady != 0 || acc.TrainableTotal != 245173 || len(rows) != 245173 {
		t.Fatalf("snapshot accounting %+v rows=%d", acc, len(rows))
	}
	if acc.TrainableUP != 94588 || acc.TrainableDOWN != 145119 || acc.TrainableTIMEOUT != 5466 {
		t.Fatalf("trainable snapshot %+v", acc)
	}
	if acc.ExcludedByReason[forecast.ReasonATRZero] != 5 ||
		acc.ExcludedByReason[forecast.ReasonTruncatedHorizon] != 5 ||
		acc.ExcludedByReason[forecast.ReasonFinerMissing] != 8 ||
		acc.ExcludedByReason[forecast.ReasonFinerDualHit] != 60 {
		t.Fatalf("excluded snapshot %+v", acc.ExcludedByReason)
	}
}

func canonicalResearchArtifactPaths(t *testing.T, key forecast.MarketKey, plan, target forecast.Digest) (tapePath, labelPath string, ok bool) {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	tapePath = filepath.Join(root, "research", "tapes", researchTapeFileName(key, plan))
	labelPath = filepath.Join(root, "research", "labels", researchLabelSetFileName(key, plan, target))
	if _, err := os.Stat(tapePath); err != nil {
		return "", "", false
	}
	if _, err := os.Stat(labelPath); err != nil {
		return "", "", false
	}
	return tapePath, labelPath, true
}
