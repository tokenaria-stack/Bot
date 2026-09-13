package market

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/forecast"
	"trading_bot/indicators"
)

// ResearchStructuralStopFractalSettings pins fractal scan to analysis:v2 RSX
// (length 14, hlc3, lookback 90) and default pivot radius 2 — not live JSON.
func ResearchStructuralStopFractalSettings() RSXSettings {
	return NormalizeRSXSettings(RSXSettings{
		Length:       forecast.Spec2RSXLength,
		SignalLength: forecast.Spec2RSXSignal,
		Source:       forecast.Spec2RSXSource,
		DivLookback:  forecast.Spec2TVLookback,
		PivotRadius:  DefaultRSXPivotRadius,
	})
}

func TestStructuralStop1_ResearchArchive(t *testing.T) {
	if os.Getenv("STRUCTURAL_STOP_1") != "1" {
		t.Skip("set STRUCTURAL_STOP_1=1 to run the archive structural-stop study")
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
	tapePath := filepath.Join(root, "research", "tapes2", tapeName)
	th, trows, _, err := forecast.ReadTape2(tapePath)
	if err != nil {
		t.Fatalf("ReadTape2: %v", err)
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
	rsx := make([]float64, len(primary))
	jurik := indicators.NewJurikRSX(forecast.Spec2RSXLength)
	for i, k := range primary {
		rsx[i] = jurik.Update((k.High + k.Low + k.Close) / 3)
	}
	facts := RSTFractalFactsFromClosedSeries(primary, rsx, ResearchStructuralStopFractalSettings())
	fm := th.Primary
	fm.Timeframe = "1m"
	in := forecast.StructuralStopInput{
		PrimaryTF: "15m", FinerTF: "1m", HTF1hTF: "1h",
		FinerMarket: fm, Primary: pCanon, HTF1h: hCanon, Finer: fCanon,
		Tape: trows, Fractals: facts,
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
		LongSample  []int                               `json:"long_sample"`
		Assignments []forecast.StructuralStopAssignment `json:"assignments"`
		Report      string                              `json:"report"`
		Notes       []string                            `json:"notes"`
	}{LongSample: a.LongSample, Assignments: a.Assignments, Report: a.Text, Notes: a.Notes}
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, a.Text)
	fmt.Fprintf(os.Stderr, "STRUCTURAL-STOP-1 GREEN assignments=%d sample=%d NO TARGET SELECTED\n", len(a.Assignments), len(a.LongSample))
}
