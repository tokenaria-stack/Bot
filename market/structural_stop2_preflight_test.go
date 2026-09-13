package market

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/forecast"
)

func TestStructuralStop2_PriceK2S0_ResearchArchive(t *testing.T) {
	if os.Getenv("STRUCTURAL_STOP_2") != "1" {
		t.Skip("set STRUCTURAL_STOP_2=1 to freeze S0 price-k2 wick assignments")
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
	th, trows, _, err := forecast.ReadTape2(filepath.Join(root, "research", "tapes2", tapeName))
	if err != nil {
		t.Fatal(err)
	}
	primary := loadResearchClosedBarsTF(t, "15m")
	h1 := loadResearchClosedBarsTF(t, "1h")
	finer := loadResearchClosedBarsTF(t, "1m")
	pCanon, err := klinesToCanonical(primary)
	if err != nil {
		t.Fatal(err)
	}
	hCanon, err := klinesToCanonical(h1)
	if err != nil {
		t.Fatal(err)
	}
	fCanon, err := klinesToCanonical(finer)
	if err != nil {
		t.Fatal(err)
	}
	fm := th.Primary
	fm.Timeframe = "1m"
	in := forecast.StructuralStopInput{
		PrimaryTF: "15m", FinerTF: "1m", HTF1hTF: "1h",
		FinerMarket: fm, Primary: pCanon, HTF1h: hCanon, Finer: fCanon,
		Tape: trows, StopOwner: forecast.StopOwnerPriceK2,
	}
	a, err := forecast.RunStructuralStop1(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := forecast.RunStructuralStop1(in)
	if err != nil {
		t.Fatal(err)
	}
	if a.Text != b.Text {
		t.Fatal("determinism mismatch")
	}
	dir := filepath.Join(root, "research", "structuralstop1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	snap := struct {
		WickOwner   string                              `json:"wick_owner"`
		LongSample  []int                               `json:"long_sample"`
		Assignments []forecast.StructuralStopAssignment `json:"assignments"`
		Report      string                              `json:"report"`
		Notes       []string                            `json:"notes"`
	}{WickOwner: a.WickOwner, LongSample: a.LongSample, Assignments: a.Assignments, Report: a.Text, Notes: a.Notes}
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot_s0.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, a.Text)
	fmt.Fprintf(os.Stderr, "STRUCTURAL-STOP-2 S0 GREEN assignments=%d NO TARGET NO 0.25ATR\n", len(a.Assignments))
}
