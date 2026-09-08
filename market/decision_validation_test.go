package market

import (
	"testing"
	"time"

	"trading_bot/data"
	"trading_bot/forecast"
)

func TestResearchDecisionValidationPlan_Rules(t *testing.T) {
	t.Parallel()
	spec, err := ResearchTargetSpec()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := ResearchDecisionValidationPlan()
	if err != nil {
		t.Fatal(err)
	}
	model, err := ResearchValidationPlan()
	if err != nil {
		t.Fatal(err)
	}
	if plan.Logic != forecast.DecisionValidationLogicWalkForwardV1 {
		t.Fatalf("logic %q", plan.Logic)
	}
	if plan.Logic == model.Logic {
		t.Fatal("decision plan must not reuse model logic identity")
	}
	if plan.HoldoutStartAt != ResearchHoldoutStartAt() || plan.HoldoutStartAt != model.HoldoutStartAt {
		t.Fatalf("holdout wall")
	}
	if plan.TargetH != spec.HorizonBars {
		t.Fatalf("TargetH %d != spec %d", plan.TargetH, spec.HorizonBars)
	}
	if plan.ValidationSpanBars != 8640 || plan.FoldCount != 4 || plan.MinTrainRows != 35040 || plan.ExtraGapBars != 0 {
		t.Fatalf("current decision rules %+v", plan)
	}
	if model.ValidationSpanBars == plan.ValidationSpanBars {
		t.Fatal("decision span must not silently equal model 183-day span")
	}
}

func TestCompileResearchDecisionValidation_MinTrainRefuse(t *testing.T) {
	t.Parallel()
	start := int64(1_699_999_200_000)
	ats := make([]int64, 100)
	at := start
	for i := 0; i < 100; i++ {
		ats[i] = at
		n, err := data.NextBarOpen(at, "15m")
		if err != nil {
			t.Fatal(err)
		}
		at = n
	}
	_, _, _, err := CompileResearchDecisionValidation(ats)
	if err == nil {
		t.Fatal("expected MinTrainRows refuse")
	}
}

func TestCompileResearchDecisionValidation_CalendarAudit(t *testing.T) {
	if testing.Short() {
		t.Skip("dense At[] calendar audit")
	}
	open, err := data.CurrentBarOpen(time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC).UnixMilli(), "15m")
	if err != nil {
		t.Fatal(err)
	}
	holdout := ResearchHoldoutStartAt()
	var ats []int64
	at := open
	for at < holdout {
		ats = append(ats, at)
		n, err := data.NextBarOpen(at, "15m")
		if err != nil {
			t.Fatal(err)
		}
		at = n
	}
	compiled, plan, _, err := CompileResearchDecisionValidation(ats)
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]int64{
		{time.Date(2025, 1, 5, 18, 0, 0, 0, time.UTC).UnixMilli(), time.Date(2025, 4, 5, 18, 0, 0, 0, time.UTC).UnixMilli()},
		{time.Date(2025, 4, 5, 18, 0, 0, 0, time.UTC).UnixMilli(), time.Date(2025, 7, 4, 18, 0, 0, 0, time.UTC).UnixMilli()},
		{time.Date(2025, 7, 4, 18, 0, 0, 0, time.UTC).UnixMilli(), time.Date(2025, 10, 2, 18, 0, 0, 0, time.UTC).UnixMilli()},
		{time.Date(2025, 10, 2, 18, 0, 0, 0, time.UTC).UnixMilli(), time.Date(2025, 12, 31, 18, 0, 0, 0, time.UTC).UnixMilli()},
	}
	if len(compiled.Folds) != 4 {
		t.Fatal(len(compiled.Folds))
	}
	for i, f := range compiled.Folds {
		if f.ValBoundaryStartAt != want[i][0] || f.ValBoundaryEndAt != want[i][1] {
			t.Fatalf("fold %d bounds %d %d want %v", i, f.ValBoundaryStartAt, f.ValBoundaryEndAt, want[i])
		}
		if f.TrainEnd-f.TrainBegin < plan.MinTrainRows {
			t.Fatalf("fold0 train")
		}
	}
}

func TestCompileResearchDecisionValidation_UnsortedRefuse(t *testing.T) {
	t.Parallel()
	_, _, _, err := CompileResearchDecisionValidation([]int64{2, 1})
	if err == nil {
		t.Fatal("expected unsorted refuse")
	}
}
