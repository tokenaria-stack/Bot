package market

import (
	"testing"

	"trading_bot/forecast"
)

func TestResearchValidationPlanC_PolicyAndV1Unchanged(t *testing.T) {
	v1, err := ResearchValidationPlan()
	if err != nil {
		t.Fatal(err)
	}
	c, err := ResearchValidationPlanC()
	if err != nil {
		t.Fatal(err)
	}
	if v1.TargetH != 24 || v1.ValidationSpanBars != 17568 || v1.MinTrainRows != 35040 || v1.HoldoutStartAt != ResearchHoldoutStartAt() {
		t.Fatalf("v1 plan mutated %+v", v1)
	}
	if c.HoldoutStartAt != 1767225600000 || c.HoldoutStartAt != ResearchHoldoutStartAt() {
		t.Fatal(c.HoldoutStartAt)
	}
	if c.FoldCount != 4 || c.ExtraGapBars != 0 || c.MinTrainRows != 35040 || c.ValidationSpanBars != 17568 {
		t.Fatalf("policy %+v", c)
	}
	if c.TargetH != 72 {
		t.Fatal(c.TargetH)
	}
	causal, err := c.TotalCausalBars()
	if err != nil || causal != 72 {
		t.Fatal(causal, err)
	}
	if c.Timeframe != "15m" || c.Logic != forecast.ValidationLogicWalkForwardV1 {
		t.Fatal(c.Timeframe, c.Logic)
	}
	id1, err := v1.Identity()
	if err != nil {
		t.Fatal(err)
	}
	idc, err := c.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if id1.Digest == idc.Digest {
		t.Fatal("Plan-C digest must differ from H=24")
	}
	if c.ValidationSpanBars == 8640 {
		t.Fatal("DecisionResearch span")
	}
}
