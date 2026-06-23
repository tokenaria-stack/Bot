package strategy

import (
	"context"
	"testing"
)

func TestSetScoreThresholds(t *testing.T) {
	origL, origS := LongScoreThreshold(), ShortScoreThreshold()
	t.Cleanup(func() { SetScoreThresholds(origL, origS) })

	SetScoringMatrix(allEnabledScoringMatrix())
	t.Cleanup(ResetScoringMatrix)

	SetScoreThresholds(101, 101)
	if LongScoreThreshold() != 101 || ShortScoreThreshold() != 101 {
		t.Fatalf("thresholds = %d/%d, want 101/101", LongScoreThreshold(), ShortScoreThreshold())
	}

	decision := EvaluateScalpSignal(context.Background(), longSignalReport(), DefaultScalpFeeRate, nil)
	if decision.Action != WaitAction {
		t.Fatalf("Action = %q with threshold 101, want WAIT (score=%d)", decision.Action, decision.Score)
	}

	SetScoreThresholds(70, 70)
	decision = EvaluateScalpSignal(context.Background(), longSignalReport(), DefaultScalpFeeRate, nil)
	if decision.Action != BuyAction {
		t.Fatalf("Action = %q with threshold 70, want BUY", decision.Action)
	}
}

func TestSetScoreThresholds_Clamp(t *testing.T) {
	origL, origS := LongScoreThreshold(), ShortScoreThreshold()
	t.Cleanup(func() { SetScoreThresholds(origL, origS) })

	SetScoreThresholds(5, 300)
	if LongScoreThreshold() != origL || ShortScoreThreshold() != origS {
		t.Fatalf("invalid values should be ignored: got %d/%d", LongScoreThreshold(), ShortScoreThreshold())
	}
}
