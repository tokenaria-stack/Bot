package strategy

import (
	"testing"
)

func TestChartMarkerAt_FractalConfirmationLag(t *testing.T) {
	t.Parallel()

	ResetRSXSettings()
	t.Cleanup(ResetRSXSettings)
	ApplyRSXSettings(RSXSettings{DivMethod: "fractal", PivotRadius: 2})

	rsx := []float64{50, 52, 54, 58, 62, 64, 63, 70, 63, 61, 58, 54, 52, 50, 48}
	klines := makeSyntheticKlines(len(rsx))
	m := NewMarker(klines, nil, "1m", "", ChaosConfig{})
	m.mu.Lock()
	m.JurikLines = append([]float64(nil), rsx...)
	m.mu.Unlock()

	if got := m.ChartMarkerAt(7); got != "" {
		t.Fatalf("pivot bar should not display marker, got %q", got)
	}
	if got := m.ChartMarkerAt(14); got != "P" {
		t.Fatalf("macro P confirmation bar = pivot+7, got %q want P", got)
	}
}

func TestExportAllAnnotations_KnownRSXMarker(t *testing.T) {
	t.Parallel()

	klines := makeSyntheticKlines(80)
	m := NewMarker(klines, nil, "1m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	color, position, shape := rsxAnnotationStyle("L")
	m.mu.Lock()
	m.Annotations = append(m.Annotations, ChartAnnotation{
		Time:     klines[21].OpenTime / 1000,
		Pane:     "rsx",
		Label:    "L",
		Color:    color,
		Position: position,
		Shape:    shape,
	})
	m.mu.Unlock()

	export := m.ExportAllAnnotations(0, len(klines)-1)
	found := false
	for _, ann := range export {
		if ann.Pane == "rsx" && ann.Label == "L" {
			found = true
			if ann.Time != klines[21].OpenTime/1000 {
				t.Fatalf("confirmation time = %d, want %d", ann.Time, klines[21].OpenTime/1000)
			}
			if ann.Color == "" || ann.Position == "" || ann.Shape == "" {
				t.Fatalf("incomplete style: %+v", ann)
			}
		}
	}
	if !found {
		t.Fatal("expected RSX annotation for injected L marker")
	}
}

func TestUpdateKlineTick_ClosedBarPreservesRSXAnnotations(t *testing.T) {
	t.Parallel()

	klines := makeSyntheticKlines(10)
	closed := NewMarker(nil, nil, "1m", "", ChaosConfig{})
	for i, k := range klines {
		closed.UpdateKlineTick(k, true)
		closed.mu.Lock()
		color, position, shape := rsxAnnotationStyle("L")
		closed.appendRSXAnnotationLocked(ChartAnnotation{
			Time:     klines[i].OpenTime / 1000,
			Pane:     "rsx",
			Label:    "L",
			Color:    color,
			Position: position,
			Shape:    shape,
		})
		closed.saveLayer2StreamingState()
		closed.mu.Unlock()
	}
	if got := len(closed.ExportAllAnnotations(0, len(klines)-1)); got == 0 {
		t.Fatal("closed-bar path should retain RSX annotations")
	}

	open := NewMarker(nil, nil, "1m", "", ChaosConfig{})
	for _, k := range klines {
		open.UpdateKlineTick(k, false)
	}
	if got := len(open.ExportAllAnnotations(0, len(klines)-1)); got != 0 {
		t.Fatalf("open-bar path without closed save should not retain RSX annotations, got %d", got)
	}
}
