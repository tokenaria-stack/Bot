package market

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/forecast"
)

func TestSetupDataset1_FromFrozenOwners(t *testing.T) {
	if os.Getenv("SETUP_DATASET_1") != "1" {
		t.Skip("set SETUP_DATASET_1=1 to join SETUP-DATASET-1 from tape2 + STOP-2 + labels + validation")
	}
	spec2, err := ResearchFeatureSpec2()
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
	th, trows, tf, err := forecast.ReadTape2(filepath.Join(root, "research", "tapes2", tapeName))
	if err != nil {
		t.Fatal(err)
	}
	rawLS, err := os.ReadFile(filepath.Join(root, "research", "setuplabels", "setup-labelset-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ls forecast.SetupLabelSet1
	if err := json.Unmarshal(rawLS, &ls); err != nil {
		t.Fatal(err)
	}
	rawSnap, err := os.ReadFile(filepath.Join(root, "research", "structuralstop1", "snapshot_s0.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snap struct {
		WickOwner   string                              `json:"wick_owner"`
		Assignments []forecast.StructuralStopAssignment `json:"assignments"`
	}
	if err := json.Unmarshal(rawSnap, &snap); err != nil {
		t.Fatal(err)
	}
	val, err := forecast.CompileSetupValidation1(ls)
	if err != nil {
		t.Fatal(err)
	}
	in := forecast.SetupDataset1Input{
		Spec2: spec2, TapeHeader: th, TapeFooter: tf, Tape: trows,
		Labels: ls, Assignments: snap.Assignments, Validation: val,
	}
	got, err := forecast.BuildSetupDataset1(in)
	if err != nil {
		t.Fatal(err)
	}
	got2, err := forecast.BuildSetupDataset1(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentDigest != got2.ContentDigest || got.Text != got2.Text {
		t.Fatal("SETUP-DATASET-1 digest mismatch on second pass")
	}
	if got.Width != 66 || got.MaxCandidateAt >= got.HoldoutStartAt {
		t.Fatalf("width=%d max_at=%d wall=%d", got.Width, got.MaxCandidateAt, got.HoldoutStartAt)
	}
	dir := filepath.Join(root, "research", "setupdataset")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := got
	meta.Rows = nil
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "setup-dataset-1.meta.json"), metaJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "setup-dataset-1.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "setup-dataset-1.txt"), []byte(got.Text), 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, got.Text)
	fmt.Fprint(os.Stderr, "SETUP-DATASET-1 GREEN no CatBoost DEV-only join\n")
}
