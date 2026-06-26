package strategy

import "testing"

func TestThresholds_SetAndRestore(t *testing.T) {
	origL, origS := LongScoreThreshold(), ShortScoreThreshold()
	t.Cleanup(func() { SetScoreThresholds(origL, origS) })
	SetScoreThresholds(101, 101)
	if LongScoreThreshold() != 101 || ShortScoreThreshold() != 101 {
		t.Fatalf("thresholds = %d/%d, want 101/101", LongScoreThreshold(), ShortScoreThreshold())
	}
}

func TestThresholds_InvalidIgnored(t *testing.T) {
	origL, origS := LongScoreThreshold(), ShortScoreThreshold()
	t.Cleanup(func() { SetScoreThresholds(origL, origS) })
	SetScoreThresholds(5, 500)
	if LongScoreThreshold() != origL || ShortScoreThreshold() != origS {
		t.Fatalf("invalid values should be ignored: got %d/%d", LongScoreThreshold(), ShortScoreThreshold())
	}
}
