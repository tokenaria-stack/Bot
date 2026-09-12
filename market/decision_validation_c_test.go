package market

import (
	"testing"

	"trading_bot/forecast"
)

func TestResearchDecisionValidationPlanC_Rules(t *testing.T) {
	t.Parallel()
	spec, err := ResearchTargetSpecC()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := ResearchDecisionValidationPlanC()
	if err != nil {
		t.Fatal(err)
	}
	v1, err := ResearchDecisionValidationPlan()
	if err != nil {
		t.Fatal(err)
	}
	if plan.Logic != forecast.DecisionValidationLogicWalkForwardV1 {
		t.Fatalf("logic %q", plan.Logic)
	}
	if plan.HoldoutStartAt != ResearchHoldoutStartAt() || plan.HoldoutStartAt != v1.HoldoutStartAt {
		t.Fatal("holdout wall")
	}
	if plan.TargetH != spec.HorizonBars || plan.TargetH != 72 {
		t.Fatalf("TargetH %d want 72 from Target C", plan.TargetH)
	}
	if v1.TargetH == plan.TargetH {
		t.Fatal("Plan-C must not reuse V1 TargetH=24")
	}
	if plan.ValidationSpanBars != 8640 || plan.FoldCount != 4 || plan.MinTrainRows != 35040 || plan.ExtraGapBars != 0 {
		t.Fatalf("decision rules %+v", plan)
	}
	if plan.ValidationSpanBars != v1.ValidationSpanBars {
		t.Fatal("must keep decision span 8640, not CatBoost 17568")
	}
	pid, err := plan.Identity()
	if err != nil {
		t.Fatal(err)
	}
	v1id, err := v1.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if pid.Digest == v1id.Digest {
		t.Fatal("Plan-C identity must differ from V1")
	}
}

func TestCompileResearchDecisionValidationC_MinTrainRefuse(t *testing.T) {
	t.Parallel()
	_, _, _, err := CompileResearchDecisionValidationC([]int64{1, 2, 3})
	if err == nil {
		t.Fatal("expected refuse")
	}
}

func TestBindResearchTarget_V1AndC(t *testing.T) {
	t.Parallel()
	c, err := ResearchTargetSpecC()
	if err != nil {
		t.Fatal(err)
	}
	cid, err := c.Identity()
	if err != nil {
		t.Fatal(err)
	}
	got, err := BindResearchTarget(cid.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if got.HorizonBars != 72 || got.UpperATRMultiple != 2 || got.LowerATRMultiple != 2 {
		t.Fatalf("C spec %+v", got)
	}
	v1, err := ResearchTargetSpec()
	if err != nil {
		t.Fatal(err)
	}
	vid, err := v1.Identity()
	if err != nil {
		t.Fatal(err)
	}
	gotV1, err := BindResearchTarget(vid.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if gotV1.HorizonBars != 24 {
		t.Fatalf("V1 spec H=%d", gotV1.HorizonBars)
	}
	_, err = BindResearchTarget(forecast.Digest{})
	if err == nil {
		t.Fatal("empty digest must refuse")
	}
}
