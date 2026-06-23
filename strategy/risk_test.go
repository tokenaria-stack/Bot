package strategy

import (
	"context"
	"testing"
)

func TestAnalyst_PassThrough(t *testing.T) {
	a := NewAnalyst(false)
	report := &Report{
		Close: 100,
		Volatility: VolatilityState{
			ATR:    0.001,
			Regime: RegimeClimax,
		},
		JurikValue:    10,
		JurikIsRising: false,
	}
	if err := a.AnalyzeSignals(report, "BUY"); err != nil {
		t.Fatalf("AnalyzeSignals() = %v, want nil pass-through", err)
	}
	if err := a.AnalyzeSignals(report, "SELL"); err != nil {
		t.Fatalf("AnalyzeSignals() = %v, want nil pass-through", err)
	}
}

func TestTelemetryBrainStatus_ClearOnSignal(t *testing.T) {
	SetScoringMatrix(allEnabledScoringMatrix())
	t.Cleanup(ResetScoringMatrix)

	a := NewAnalyst(false)
	report := Report{
		Close:      100,
		RSXMarker:  "L",
		Falcon:     FalconSignals{VolCrossMarker: "lime"},
		Volatility: scalpVolatilityOK(),
	}
	result := ProcessScore(context.Background(), report, DefaultScalpFeeRate, nil)
	decision := ScalpDecisionFromScoreResult(result, report)
	if decision.Action != BuyAction {
		t.Fatalf("expected BUY signal for telemetry test, got %q", decision.Action)
	}
	status := TelemetryBrainStatus(decision, report, a)
	if status != "Clear" {
		t.Fatalf("status = %q, want Clear", status)
	}
}

func TestChiefAnalyst_PassThrough(t *testing.T) {
	chief := NewChiefAnalyst()
	in := ScalpDecision{Action: BuyAction, Score: 100, Reason: "test"}
	out := chief.Approve(in, Report{})
	if out != in {
		t.Fatalf("Approve() modified decision: %+v vs %+v", out, in)
	}
}
