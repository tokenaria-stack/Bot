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

func TestPriceSwingLaw_ResearchArchive(t *testing.T) {
	if os.Getenv("PRICE_SWING_LAW") != "1" {
		t.Skip("set PRICE_SWING_LAW=1 to compare P1/P2 on STOP-1 samples")
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
	disputed := map[int]bool{}
	for _, o := range pivotDisagreementOrdinals {
		disputed[o] = true
	}
	primary := loadResearchClosedBarsTF(t, "15m")
	pCanon, err := klinesToCanonical(primary)
	if err != nil {
		t.Fatal(err)
	}
	rsx := make([]float64, len(primary))
	jurik := indicators.NewJurikRSX(forecast.Spec2RSXLength)
	for i, k := range primary {
		rsx[i] = jurik.Update((k.High + k.Low + k.Close) / 3)
	}
	facts2 := RSTFractalFactsFromClosedSeries(primary, rsx, ResearchStructuralStopFractalSettings())
	var rows []forecast.PriceSwingLawRow
	for i, ai := range snap.LongSample {
		ord := i + 1
		a := snap.Assignments[ai]
		audit := forecast.AuditPivotDisagreement(a, ord, pCanon, facts2, nil, nil)
		later := forecast.PivotSighting{}
		if len(audit.R2) > 0 {
			later = audit.R2[0]
		}
		row, err := forecast.EvaluatePriceSwingLaw(a, ord, disputed[ord], pCanon, later)
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	text := forecast.FormatPriceSwingLaw(rows)
	dir := filepath.Join(root, "research", "structuralstop1")
	if err := os.WriteFile(filepath.Join(dir, "price_swing_law.txt"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	js, err := json.MarshalIndent(struct {
		Notes []string                    `json:"notes"`
		Rows  []forecast.PriceSwingLawRow `json:"rows"`
	}{
		Notes: []string{"NO K SELECTED", "P1 k=1 P2 k=2 causal OHLC", "no ATR buffer"},
		Rows:  rows,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "price_swing_law.json"), js, 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, text)
}
