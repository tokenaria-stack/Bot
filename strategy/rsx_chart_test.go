package strategy

import (
	"testing"

	"trading_bot/exchange"
	"trading_bot/indicators"
)

func synthKline(high, low, close float64) exchange.Kline {
	return exchange.Kline{High: high, Low: low, Close: close, Volume: 1}
}

func TestRSXColor(t *testing.T) {
	if RSXColor(60, 55) != RSXColorGreen {
		t.Fatalf("expected green for rising above 50")
	}
	if RSXColor(40, 45) != RSXColorRed {
		t.Fatalf("expected red for falling below 50")
	}
	if RSXColor(45, 44) != RSXColorNeutral {
		t.Fatalf("expected neutral beige")
	}
	if RSXColor(52, 52) != RSXColorNeutral {
		t.Fatalf("flat slope should be neutral")
	}
}

func TestIsRSXPivotHigh(t *testing.T) {
	rsx := []float64{50, 55, 58, 65, 63, 61, 59}
	if !isRSXPivotHigh(rsx, 3) {
		t.Fatal("index 3 should be 5-bar pivot high")
	}
	if isRSXPivotHigh(rsx, 4) {
		t.Fatal("index 4 should not be pivot high")
	}
}

func TestScanRSXFractalMarkers_SingleP(t *testing.T) {
	ResetRSXSettings()
	t.Cleanup(ResetRSXSettings)
	ApplyRSXSettings(RSXSettings{DivMethod: "fractal", PivotRadius: 2})

	rsx := []float64{50, 52, 54, 58, 62, 64, 63, 70, 63, 61, 58, 54, 52, 50, 48}
	prices := make([]float64, len(rsx))
	for i := range prices {
		prices[i] = 100 + rsx[i]
	}
	markers := scanRSXFractalMarkers(prices, rsx, RSXLookbackDefault)
	if markers[7] != "P" {
		t.Fatalf("expected P at macro pivot index 7, got %q", markers[7])
	}
	if len(markers) != 1 {
		t.Fatalf("expected exactly one marker, got %d: %v", len(markers), markers)
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
	markers := scanRSXFractalMarkers(prices, rsx, RSXLookbackDefault)
	if len(markers) != 0 {
		t.Fatalf("expected no P without macro pivot, got %v", markers)
	}
}

func TestBearishRSXMarker(t *testing.T) {
	m := bearishRSXMarker(indicators.DivergenceResult{Class: indicators.ClassA})
	if m != "SS" {
		t.Fatalf("ClassA bearish = SS, got %s", m)
	}
}

func TestBuildRSXChart_ColorsNotSticky(t *testing.T) {
	rsx := []float64{48, 49, 48.5, 49, 48.8}
	klines := make([]exchange.Kline, len(rsx))
	for i, v := range rsx {
		klines[i] = synthKline(100+v, 99+v, 100+v)
	}
	points := BuildRSXChart(klines, rsx, RSXLookbackDefault)
	for i, p := range points {
		want := RSXColor(rsx[i], rsx[max(0, i-1)])
		if p.Color != want {
			t.Fatalf("bar %d color %s want %s", i, p.Color, want)
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
