package indicators_test

import (
	"testing"

	"trading_bot/indicators"
)

func TestFractalFacts_SinglePivotHigh(t *testing.T) {
	t.Parallel()
	rsx := []float64{50, 52, 54, 58, 62, 64, 63, 70, 63, 61, 58, 54, 52, 50, 48}
	prices := make([]float64, len(rsx))
	opens := make([]int64, len(rsx))
	for i := range prices {
		prices[i] = 100 + rsx[i]
		opens[i] = 1_700_000_000_000 + int64(i)*60_000
	}
	cfg := indicators.RSXScanConfig{
		Mode:        indicators.RSXScanFractal,
		Lookback:    indicators.DefaultRSXLookback,
		PivotRadius: 2,
	}
	facts := indicators.FractalFacts(prices, rsx, opens, cfg)
	var pAtPivot int
	for _, ev := range facts {
		if ev.Source == indicators.FactSourceRSXFractalPivot && ev.Direction == indicators.FactDirPivotHigh && ev.AnchorAt == opens[7] {
			pAtPivot++
		}
		if ev.Direction == "P" || ev.Pattern == "P" || ev.Direction == "L" || ev.Direction == "S" {
			t.Fatalf("fractal fact leaked legacy vocab %+v", ev)
		}
	}
	if pAtPivot != 1 {
		t.Fatalf("expected exactly one pivot high at 7, got %d facts: %+v", pAtPivot, facts)
	}
	if len(facts) != 1 {
		t.Fatalf("expected exactly one fact, got %d: %+v", len(facts), facts)
	}
}
