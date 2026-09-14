package forecast

import (
	"strings"
	"testing"

	"trading_bot/data"
)

func TestDefaultDataSplitPolicy_2026Wall(t *testing.T) {
	t.Parallel()
	p := DefaultDataSplitPolicy()
	if p.HoldoutStartAt != 1767225600000 || p.Timeframe != "15m" {
		t.Fatalf("%+v", p)
	}
	open, err := data.CurrentBarOpen(p.HoldoutStartAt, "15m")
	if err != nil || open != p.HoldoutStartAt {
		t.Fatal("wall must sit on 15m grid")
	}
	if p.Role(p.HoldoutStartAt-1) != DataRoleDev || p.Role(p.HoldoutStartAt) != DataRoleHoldout {
		t.Fatal("role cut")
	}
	if err := p.RefuseHoldoutAt(p.HoldoutStartAt - 1); err != nil {
		t.Fatal(err)
	}
	if err := p.RefuseHoldoutAt(p.HoldoutStartAt); err == nil {
		t.Fatal("holdout At must refuse")
	}
	a, err := p.Identity()
	if err != nil {
		t.Fatal(err)
	}
	q := p
	q.HoldoutStartAt = p.HoldoutStartAt + 15*60*1000
	b, err := q.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if a.Digest == b.Digest {
		t.Fatal("new wall must mint a new DataSplitPolicy identity")
	}
}

func TestFrozenSetupValidationPlan_EventScale(t *testing.T) {
	t.Parallel()
	p, err := FrozenSetupValidationPlan()
	if err != nil {
		t.Fatal(err)
	}
	if p.Logic != SetupValidationLogicWalkForwardV1 || p.TargetH != 72 {
		t.Fatalf("%+v", p)
	}
	if p.ValidationSpanBars != 35040 || p.FoldCount != 4 || p.MinTrainRows != 1000 {
		t.Fatalf("year-scale event plan %+v", p)
	}
	if p.HoldoutStartAt != DefaultResearchHoldoutStartAt {
		t.Fatalf("wall %d", p.HoldoutStartAt)
	}
	id, err := p.Identity()
	if err != nil {
		t.Fatal(err)
	}
	alt := p
	alt.HoldoutStartAt = DefaultResearchHoldoutStartAt + 15*60*1000
	id2, err := alt.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if id.Digest == id2.Digest {
		t.Fatal("ValidationPlan identity must include the wall")
	}
}

func TestResolveSetupValidationPlan_RefusesDenseMinTrain(t *testing.T) {
	t.Parallel()
	_, err := ResolveSetupValidationPlan(ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: SetupValidationHoldoutStartAt(),
		ValidationSpanBars: 10, FoldCount: 1, MinTrainRows: 35040,
	}, FrozenSetupTarget1(), SetupValidationLogicWalkForwardV1)
	if err == nil || !strings.Contains(err.Error(), "event-scale") {
		t.Fatalf("got %v", err)
	}
}

func TestCompileValidationPlan_SetupLogicEmbargoBeforeWall(t *testing.T) {
	t.Parallel()
	holdout := SetupValidationHoldoutStartAt()
	prev, err := data.PreviousBarOpen(holdout, "15m")
	if err != nil {
		t.Fatal(err)
	}
	start := prev
	for i := 0; i < 250; i++ {
		start, err = data.PreviousBarOpen(start, "15m")
		if err != nil {
			t.Fatal(err)
		}
	}
	ats := seq15m(t, start, 250)
	p, err := ResolveSetupValidationPlan(ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: holdout,
		ValidationSpanBars: 40, FoldCount: 1, ExtraGapBars: 0, MinTrainRows: 10,
	}, FrozenSetupTarget1(), SetupValidationLogicWalkForwardV1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CompileValidationPlan(ats, "15m", p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Folds[0].ValLastAt >= holdout {
		t.Fatalf("OOF crossed wall %d", got.Folds[0].ValLastAt)
	}
	he, err := HorizonEnd(got.Folds[0].TrainLastAt, "15m", 72)
	if err != nil {
		t.Fatal(err)
	}
	if he >= got.Folds[0].ValBoundaryStartAt {
		t.Fatalf("embargo failed %d >= %d", he, got.Folds[0].ValBoundaryStartAt)
	}
}

func TestCompileSetupValidation1_RequiresMatch(t *testing.T) {
	t.Parallel()
	_, err := CompileSetupValidation1(SetupLabelSet1{MatchOK: false})
	if err == nil {
		t.Fatal("expected MATCH gate")
	}
}
