package strategy

import (
	"testing"
)

func TestRSXTradingMarkerAtBar_CurrentBarOnly(t *testing.T) {
	t.Parallel()

	points := []RSXPoint{
		{Marker: "LL"},
		{Marker: "L"},
	}
	if got := RSXTradingMarkerAtBar(points, 1); got != "L" {
		t.Fatalf("current bar = %q, want L", got)
	}
	if got := RSXTradingMarkerAtBar(points, 0); got != "LL" {
		t.Fatalf("bar 0 = %q, want LL", got)
	}
}

func TestRSXTradingMarker_StreamMatchesBatchOnCurrentBar(t *testing.T) {
	t.Parallel()

	klines := syntheticRSXKlines(120)
	m := NewMarker(klines, nil, "1m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	falcon := NewFalconEngine()
	rsxValues := make([]float64, len(klines))
	for i, k := range klines {
		rsxValues[i] = falcon.Evaluate(k.High, k.Low, k.Close, k.Volume).JurikRSX
	}
	points := BuildRSXChart(klines, rsxValues, RSXLookbackDefault)
	batchCurrent := RSXTradingMarkerAtBar(points, len(points)-1)
	streamCurrent := m.RecentRSXMarker()
	if batchCurrent == "" && streamCurrent == "" {
		t.Skip("synthetic series produced no trading marker on current bar")
	}
	if streamCurrent != batchCurrent {
		t.Fatalf("streaming = %q, batch current bar = %q", streamCurrent, batchCurrent)
	}
}

func TestLatestRSXChartMarker_CurrentBar(t *testing.T) {
	t.Parallel()

	klines := syntheticRSXKlines(120)
	got := LatestRSXChartMarker(klines, RSXLookbackDefault)
	falcon := NewFalconEngine()
	rsxValues := make([]float64, len(klines))
	for i, k := range klines {
		rsxValues[i] = falcon.Evaluate(k.High, k.Low, k.Close, k.Volume).JurikRSX
	}
	points := BuildRSXChart(klines, rsxValues, RSXLookbackDefault)
	want := RSXTradingMarkerAtBar(points, len(points)-1)
	if got != want {
		t.Fatalf("chart marker = %q, want %q", got, want)
	}
}
