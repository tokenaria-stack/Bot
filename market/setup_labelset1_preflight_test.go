package market

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/forecast"
)

func TestSetupLabelSet1_FromFrozenS0Snapshot(t *testing.T) {
	if os.Getenv("SETUP_LABELSET_1") != "1" {
		t.Skip("set SETUP_LABELSET_1=1 to freeze SETUP-LABELSET-1 from snapshot_s0.json")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "research", "structuralstop1", "snapshot_s0.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snap struct {
		WickOwner   string                              `json:"wick_owner"`
		Assignments []forecast.StructuralStopAssignment `json:"assignments"`
		Notes       []string                            `json:"notes"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	rep := forecast.StructuralStopReport{WickOwner: snap.WickOwner, Assignments: snap.Assignments, Notes: snap.Notes}
	ls, err := forecast.LabelSetupTarget1(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !ls.MatchOK {
		t.Fatal(ls.MatchDetail)
	}
	dir := filepath.Join(root, "research", "setuplabels")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := json.MarshalIndent(ls, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "setup-labelset-1.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "setup-labelset-1.txt"), []byte(ls.Text), 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, ls.Text)
	fmt.Fprint(os.Stderr, "SETUP-LABELSET-1 GREEN MOVE-POTENTIAL +2R MATCH NO CATBOOST\n")
}
