package market

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/forecast"
)

var structuralSignificanceControls = []int{3, 8, 12, 19, 26, 29}

func TestStructuralSignificanceAudit_ResearchArchive(t *testing.T) {
	if os.Getenv("STRUCTURAL_SIGNIFICANCE") != "1" {
		t.Skip("set STRUCTURAL_SIGNIFICANCE=1 to annotate k=2 prominence")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "research", "structuralstop1", "snapshot.json"))
	if err != nil {
		t.Fatalf("need STRUCTURAL-STOP-1 snapshot: %v", err)
	}
	var snap struct {
		LongSample  []int                               `json:"long_sample"`
		Assignments []forecast.StructuralStopAssignment `json:"assignments"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	want := map[int]bool{}
	for _, o := range pivotDisagreementOrdinals {
		want[o] = true
	}
	for _, o := range structuralSignificanceControls {
		want[o] = true
	}
	disputed := map[int]bool{}
	for _, o := range pivotDisagreementOrdinals {
		disputed[o] = true
	}
	primary := loadResearchClosedBarsTF(t, "15m")
	pCanon, err := klinesToCanonical(primary)
	if err != nil {
		t.Fatal(err)
	}
	var all, report []forecast.StructuralSignificanceRow
	for i, ai := range snap.LongSample {
		ord := i + 1
		a := snap.Assignments[ai]
		sw, atr, err := forecast.LastCausalK2WithProminence(pCanon, a.At, true, forecast.StructuralSignificanceLastN)
		if err != nil {
			t.Fatal(err)
		}
		row := forecast.StructuralSignificanceRow{
			Ordinal: ord, Disputed: disputed[ord], Control: want[ord] && !disputed[ord],
			At: a.At, Entry: a.Entry, ATR15: atr, Swings: sw,
		}
		all = append(all, row)
		if want[ord] {
			report = append(report, row)
		}
	}
	text := forecast.FormatStructuralSignificanceAudit(report)
	dir := filepath.Join(root, "research", "structuralstop1")
	if err := os.WriteFile(filepath.Join(dir, "structural_significance.txt"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	js, err := json.MarshalIndent(struct {
		Notes []string                             `json:"notes"`
		Rows  []forecast.StructuralSignificanceRow `json:"rows"`
	}{
		Notes: []string{"NO τ SELECTED", "S0=latest k=2", "prominence causal to ENTRY"},
		Rows:  all,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "structural_significance.json"), js, 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, text)
}
