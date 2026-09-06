package indicators_test

import (
	"testing"

	"trading_bot/indicators"
)

type stubDataBus struct {
	jurik, red, green, prices, closes []float64
}

func (s *stubDataBus) JurikSeries() []float64       { return s.jurik }
func (s *stubDataBus) WozduhRedSeries() []float64   { return s.red }
func (s *stubDataBus) WozduhGreenSeries() []float64 { return s.green }
func (s *stubDataBus) RSXPriceSeries() []float64    { return s.prices }
func (s *stubDataBus) CloseSeries() []float64       { return s.closes }

func TestScanRSXFractalHits_SingleP(t *testing.T) {
	t.Parallel()
	rsx := []float64{50, 52, 54, 58, 62, 64, 63, 70, 63, 61, 58, 54, 52, 50, 48}
	prices := make([]float64, len(rsx))
	for i := range prices {
		prices[i] = 100 + rsx[i]
	}
	cfg := indicators.RSXScanConfig{
		Mode:        indicators.RSXScanFractal,
		Lookback:    indicators.DefaultRSXLookback,
		PivotRadius: 2,
	}
	bus := &stubDataBus{jurik: rsx, prices: prices}
	hits := indicators.ScanRSXMarkers(bus, cfg)
	var pAtPivot int
	for _, h := range hits {
		if h.IsPivot && h.PivotBar == 7 && h.PeakType == indicators.PeakHigh {
			pAtPivot++
		}
		if h.Label == "P" || h.Label == "L" || h.Label == "LL" || h.Label == "S" || h.Label == "SS" {
			t.Fatalf("fractal hit leaked label %q", h.Label)
		}
	}
	if pAtPivot != 1 {
		t.Fatalf("expected exactly one pivot high at 7, got %d hits: %+v", pAtPivot, hits)
	}
}
