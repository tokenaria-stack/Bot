package market

import (
	"testing"
)

func TestChartMarkerAt_PhaseFEmpty(t *testing.T) {
	t.Parallel()

	ResetRSXSettings()
	t.Cleanup(ResetRSXSettings)
	ApplyRSXSettings(RSXSettings{DivMethod: "fractal", PivotRadius: 2})

	rsx := []float64{50, 52, 54, 58, 62, 64, 63, 70, 63, 61, 58, 54, 52, 50, 48}
	klines := makeSyntheticKlines(len(rsx))
	m := NewFrame(klines, "1m", ChaosConfig{})
	m.mu.Lock()
	m.JurikLines = append([]float64(nil), rsx...)
	m.mu.Unlock()

	if got := m.ChartMarkerAt(7); got != "" {
		t.Fatalf("Phase F: ChartMarkerAt must be empty, got %q", got)
	}
	if got := m.ChartMarkerAt(14); got != "" {
		t.Fatalf("Phase F: ChartMarkerAt must be empty, got %q", got)
	}
}

func TestExportAllAnnotations_NoRSXTradingLabels(t *testing.T) {
	t.Parallel()

	klines := makeSyntheticKlines(80)
	m := NewFrame(klines, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	m.mu.Lock()
	m.Annotations = append(m.Annotations, ChartAnnotation{
		Time:     klines[21].OpenTime / 1000,
		Pane:     "rsx",
		Label:    "L",
		Color:    "#26a69a",
		Position: "belowBar",
		Shape:    "arrowUp",
	})
	m.mu.Unlock()

	export := m.ExportAllAnnotations(0, len(klines)-1)
	for _, ann := range export {
		if ann.Pane == "rsx" && (ann.Label == "L" || ann.Label == "LL" || ann.Label == "S" || ann.Label == "SS") {
			t.Fatalf("Phase F: RSX trading label leaked: %+v", ann)
		}
	}
}
