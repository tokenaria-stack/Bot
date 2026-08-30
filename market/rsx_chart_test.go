package market

import (
	"testing"

	"trading_bot/indicators"
)

func TestIsRSXPivotHigh(t *testing.T) {
	rsx := []float64{50, 55, 58, 65, 63, 61, 59}
	if !indicators.IsRSXPivotHigh(rsx, 3, 2) {
		t.Fatal("index 3 should be 5-bar pivot high")
	}
	if indicators.IsRSXPivotHigh(rsx, 4, 2) {
		t.Fatal("index 4 should not be pivot high")
	}
}

func TestScanRSXFractalHits_SingleP(t *testing.T) {
	ResetRSXSettings()
	t.Cleanup(ResetRSXSettings)
	ApplyRSXSettings(RSXSettings{DivMethod: "fractal", PivotRadius: 2})

	rsx := []float64{50, 52, 54, 58, 62, 64, 63, 70, 63, 61, 58, 54, 52, 50, 48}
	prices := make([]float64, len(rsx))
	for i := range prices {
		prices[i] = 100 + rsx[i]
	}
	cfg := RSXScanConfigFromSettings(GetRSXSettings())
	engine := indicators.NewSmartDivergenceEngine(cfg)
	bus := newBatchDataBus(rsx, prices, nil)
	hits := engine.ScanRSX(bus)
	var pAtPivot int
	for _, h := range hits {
		if h.Label == "P" && h.PivotBar == 7 {
			pAtPivot++
		}
	}
	if pAtPivot != 1 {
		t.Fatalf("expected exactly one P at pivot 7, got %d hits: %+v", pAtPivot, hits)
	}
	if len(hits) != 1 {
		t.Fatalf("expected exactly one marker, got %d: %+v", len(hits), hits)
	}
}

func TestScanRSXFractalMarkers_NoPWithoutMacro(t *testing.T) {
	ResetRSXSettings()
	t.Cleanup(ResetRSXSettings)
	ApplyRSXSettings(RSXSettings{DivMethod: "fractal", PivotRadius: 2})

	rsx := []float64{55, 58, 62, 65, 63, 61, 59, 57}
	prices := make([]float64, len(rsx))
	for i := range prices {
		prices[i] = 100 + rsx[i]
	}
	cfg := RSXScanConfigFromSettings(GetRSXSettings())
	engine := indicators.NewSmartDivergenceEngine(cfg)
	bus := newBatchDataBus(rsx, prices, nil)
	if len(engine.ScanRSX(bus)) != 0 {
		t.Fatalf("expected no P without macro pivot, got %+v", engine.ScanRSX(bus))
	}
}

func TestBearishRSXMarker(t *testing.T) {
	m := indicators.BearishRSXMarkerLabel(indicators.DivergenceResult{Class: indicators.ClassA})
	if m != "SS" {
		t.Fatalf("ClassA bearish = SS, got %s", m)
	}
}
