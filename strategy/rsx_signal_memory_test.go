package strategy

import (
	"testing"
)

func TestRecentRSXTradingMarker_FindsLaggedPivot(t *testing.T) {
	t.Parallel()

	klines := syntheticRSXKlines(120)
	falcon := NewFalconEngine()
	rsxValues := make([]float64, len(klines))
	for i, k := range klines {
		rsxValues[i] = falcon.Evaluate(k.High, k.Low, k.Close, k.Volume).JurikRSX
	}
	points := BuildRSXChart(klines, rsxValues, RSXLookbackDefault)

	state := newRSXMarkerState(RSXLookbackDefault)
	for i, k := range klines {
		state.appendBar(k.High, k.Low, k.Close, rsxValues[i])
	}

	batchRecent := RecentRSXTradingMarker(points, RSXSignalMemoryBars)
	streamRecent := state.recentTradingMarker(RSXSignalMemoryBars)
	if batchRecent == "" {
		t.Skip("synthetic series produced no recent trading markers")
	}
	if streamRecent != batchRecent {
		t.Fatalf("streaming = %q, batch = %q", streamRecent, batchRecent)
	}
	if state.latest == streamRecent && streamRecent != "" {
		// latest is only set when the marker sits on the current bar (rare with pivot lag)
		lastIdx := len(points) - 1
		if points[lastIdx].Marker != streamRecent {
			t.Fatalf("latest=%q should not equal lagged recent=%q on last bar", state.latest, streamRecent)
		}
	}
}

func TestRecentRSXTradingMarker_PrefersStronger(t *testing.T) {
	t.Parallel()

	points := []RSXPoint{
		{Marker: "L"},
		{Marker: "LL"},
	}
	if got := RecentRSXTradingMarker(points, 3); got != "LL" {
		t.Fatalf("got %q, want LL", got)
	}
}

func TestRecentRSXTradingMarkerFromSeries(t *testing.T) {
	t.Parallel()

	klines := syntheticRSXKlines(120)
	falcon := NewFalconEngine()
	rsxValues := make([]float64, len(klines))
	for i, k := range klines {
		rsxValues[i] = falcon.Evaluate(k.High, k.Low, k.Close, k.Volume).JurikRSX
	}
	got := RecentRSXTradingMarkerFromSeries(klines, rsxValues, RSXLookbackDefault, RSXSignalMemoryBars)
	points := BuildRSXChart(klines, rsxValues, RSXLookbackDefault)
	want := RecentRSXTradingMarker(points, RSXSignalMemoryBars)
	if got != want {
		t.Fatalf("from series = %q, batch scan = %q", got, want)
	}
}

// Ensure syntheticRSXKlines is available (defined in rsx_incremental_test.go same package).
