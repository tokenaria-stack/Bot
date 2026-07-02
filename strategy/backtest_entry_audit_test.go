package strategy

import "testing"

func TestBuildTradeEntryAudit_WinningSideFactors(t *testing.T) {
	t.Parallel()

	decision := ScoreDecision{
		LongScore:  80,
		ShortScore: 20,
		Factors: map[string]ScoreFactor{
			"RSX":         {Name: "RSX LL", Direction: BuyAction, Score: 45},
			"WozduhCross": {Name: "Wozduh cross", Direction: BuyAction, Score: 35},
			"noise":       {Name: "HTF RSX 4h", Direction: SellAction, Score: 20},
		},
	}

	reason, snapshot, score := buildTradeEntryAudit(decision, "BUY")
	if score != 80 {
		t.Fatalf("score = %v, want 80", score)
	}
	if reason != "RSX LL + Wozduh cross" {
		t.Fatalf("reason = %q", reason)
	}
	if len(snapshot) != 2 {
		t.Fatalf("snapshot = %v, want 2 factors", snapshot)
	}
}

func TestSnapshotHasRSXFactor(t *testing.T) {
	t.Parallel()

	if snapshotHasRSXFactor([]string{"Wozduh cross (+35)"}) {
		t.Fatal("expected false without RSX")
	}
	if !snapshotHasRSXFactor([]string{"RSX L (+35)"}) {
		t.Fatal("expected true with RSX")
	}
}
