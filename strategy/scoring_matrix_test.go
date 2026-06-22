package strategy

import (
	"context"
	"testing"
)

func TestScoringMatrix_DisableRSX(t *testing.T) {
	ResetScoringMatrix()
	t.Cleanup(ResetScoringMatrix)

	m := GetScoringMatrix()
	m.UseRSX = false
	SetScoringMatrix(m)

	report := longSignalReport()
	if got := scoreLong(report); got != 65 {
		t.Fatalf("scoreLong without RSX = %d, want 65 (wozduh+expansion+AO)", got)
	}

	decision := EvaluateScalpSignal(context.Background(), report, DefaultScalpFeeRate, nil)
	if decision.Action != WaitAction {
		t.Fatalf("Action = %q, want WAIT (score=%d)", decision.Action, decision.Score)
	}
}

func TestScoringMatrix_DisableAll(t *testing.T) {
	ResetScoringMatrix()
	t.Cleanup(ResetScoringMatrix)

	SetScoringMatrix(ScoringMatrix{})

	decision := EvaluateScalpSignal(context.Background(), longSignalReport(), DefaultScalpFeeRate, nil)
	if decision.Action != WaitAction || decision.Score != 0 {
		t.Fatalf("Action = %q score = %d, want WAIT/0", decision.Action, decision.Score)
	}
}

func TestScoringMatrix_DefaultAllEnabled(t *testing.T) {
	ResetScoringMatrix()
	t.Cleanup(ResetScoringMatrix)

	m := GetScoringMatrix()
	if !m.UseRSX || !m.UseWozduhCross || !m.UseGeometry || !m.UseDivergence {
		t.Fatalf("default matrix not fully enabled: %+v", m)
	}
}
