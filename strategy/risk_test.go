package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestAnalyst_PassThrough(t *testing.T) {
	a := NewAnalyst(false)
	m := testMarkerWithFlags(t, func(marker *Marker) {})
	if err := a.AnalyzeSignals(m, "BUY"); err != nil {
		t.Fatalf("AnalyzeSignals() = %v, want nil", err)
	}
}

func TestChief_ApprovePassThrough(t *testing.T) {
	chief := NewChiefAnalyst()
	in := ScoreDecision{
		RawAction:   BuyAction,
		FinalAction: BuyAction,
		LongScore:   100,
		Reason:      "test",
		Factors:     map[string]ScoreFactor{"RSX": {Name: "RSX", Score: 35}},
	}
	chief.Approve(&in)
	if in.LongScore != 100 || len(in.Factors) != 1 {
		t.Fatalf("Approve modified telemetry: %+v", in)
	}
}

func TestTelemetryBrainStatus(t *testing.T) {
	a := NewAnalyst(false)
	if got := TelemetryBrainStatus(ScoreDecision{RawAction: BuyAction, FinalAction: BuyAction, LongScore: 80}, a); got != "Clear" {
		t.Fatalf("got %q, want Clear", got)
	}
	if got := TelemetryBrainStatus(ScoreDecision{IsVetoed: true, RawAction: BuyAction, LongScore: 80}, a); got != "Vetoed" {
		t.Fatalf("got %q, want Vetoed", got)
	}
	if got := TelemetryBrainStatus(ScoreDecision{LongScore: 30, ShortScore: 10}, a); got != "Below Threshold" {
		t.Fatalf("got %q, want Below Threshold", got)
	}
}

func TestApplyExecutionVetoes_Warmup(t *testing.T) {
	m := testMarkerWithFlags(t, func(marker *Marker) {
		marker.klines = make([]exchange.Kline, minScoreBars-1)
	})
	decision := ScoreDecision{RawAction: BuyAction, FinalAction: BuyAction, LongScore: 80}
	got := ApplyExecutionVetoes(decision, m, NewAnalyst(false), NewChiefAnalyst())
	if !got.IsVetoed {
		t.Fatal("expected warmup veto")
	}
	if got.VetoReason != "System Warmup: Not enough history" {
		t.Fatalf("VetoReason = %q", got.VetoReason)
	}
	if got.FinalAction != WaitAction {
		t.Fatalf("FinalAction = %q, want WAIT", got.FinalAction)
	}
}
