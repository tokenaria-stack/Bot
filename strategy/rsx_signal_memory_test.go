package strategy

import (
	"context"
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

func TestEvaluateScalpSignal_RSXOnlyThreshold10(t *testing.T) {
	SetScoreThresholds(10, 10)
	t.Cleanup(func() {
		SetScoreThresholds(70, 70)
	})

	m := GetScoringMatrix()
	m.UseRSX = true
	m.UseWozduhCross = false
	m.UseRedCross = false
	m.UseGeometry = false
	m.UseDivergence = false
	m.UseFib = false
	m.UseExpRegime = false
	m.UseJurikTrend = false
	m.UseWozduhSpike = false
	m.UseGeometryBounce = false
	m.UseGeometryTriangle = false
	m.UseAD = false
	m.UseAOCross = false
	SetScoringMatrix(m)
	t.Cleanup(func() { ResetScoringMatrix() })

	report := Report{
		Close:      100,
		RSXMarker:  "L",
		Volatility: scalpVolatilityOK(),
	}
	decision := scalpDecisionFromReport(context.Background(), report)
	if decision.Action != BuyAction {
		t.Fatalf("Action = %q, want BUY (score=%d)", decision.Action, decision.LongScore)
	}

	report.RSXMarker = "LL"
	decision = scalpDecisionFromReport(context.Background(), report)
	if decision.Action != BuyAction || decision.Score < 45 {
		t.Fatalf("LL: Action=%q score=%d", decision.Action, decision.Score)
	}
}

func TestAnalyst_PassThroughStrongRSX(t *testing.T) {
	a := NewAnalyst(false)
	report := &Report{
		Close: 100,
		Volatility: VolatilityState{
			ATR:    1.0,
			Regime: RegimeExpansion,
		},
		JurikValue:    30,
		JurikIsRising: false,
		RSXMarker:     "LL",
	}
	if err := a.AnalyzeSignals(report, "BUY"); err != nil {
		t.Fatalf("AnalyzeSignals() = %v, want nil", err)
	}

	report.RSXMarker = "L"
	if err := a.AnalyzeSignals(report, "BUY"); err != nil {
		t.Fatalf("AnalyzeSignals() = %v, want nil pass-through", err)
	}
}

// Ensure syntheticRSXKlines is available (defined in rsx_incremental_test.go same package).
