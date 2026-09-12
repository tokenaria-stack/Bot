package market

import (
	"fmt"

	"trading_bot/forecast"
)

// CompileResearchDecisionValidation compiles evidence-local geometry.
// at must be strictly increasing evidence timestamps only.
func CompileResearchDecisionValidation(at []int64) (forecast.CompiledValidationPlan, forecast.ValidationPlan, forecast.TargetSpec, error) {
	spec, err := ResearchTargetSpec()
	if err != nil {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, err
	}
	plan, err := ResearchDecisionValidationPlan()
	if err != nil {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, err
	}
	if plan.Logic != forecast.DecisionValidationLogicWalkForwardV1 {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, fmt.Errorf("market: decision plan logic %q", plan.Logic)
	}
	if plan.TargetH != spec.HorizonBars {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, fmt.Errorf("market: TargetH %d != TargetSpec.HorizonBars %d", plan.TargetH, spec.HorizonBars)
	}
	if plan.HoldoutStartAt != ResearchHoldoutStartAt() {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, fmt.Errorf("market: holdout wall mismatch")
	}
	compiled, err := forecast.CompileValidationPlan(at, plan.Timeframe, plan)
	if err != nil {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, err
	}
	return compiled, plan, spec, nil
}

// CompileResearchDecisionValidationC compiles DecisionValidationPlan-C from At[].
// Same compiler as V1; Target C (H=72) and ResearchDecisionValidationPlanC.
func CompileResearchDecisionValidationC(at []int64) (forecast.CompiledValidationPlan, forecast.ValidationPlan, forecast.TargetSpec, error) {
	spec, err := ResearchTargetSpecC()
	if err != nil {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, err
	}
	plan, err := ResearchDecisionValidationPlanC()
	if err != nil {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, err
	}
	if plan.Logic != forecast.DecisionValidationLogicWalkForwardV1 {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, fmt.Errorf("market: decision plan logic %q", plan.Logic)
	}
	if plan.TargetH != spec.HorizonBars {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, fmt.Errorf("market: TargetH %d != TargetSpec.HorizonBars %d", plan.TargetH, spec.HorizonBars)
	}
	if plan.TargetH != 72 {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, fmt.Errorf("market: DecisionValidationPlan-C TargetH=%d want 72", plan.TargetH)
	}
	if plan.HoldoutStartAt != ResearchHoldoutStartAt() {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, fmt.Errorf("market: holdout wall mismatch")
	}
	compiled, err := forecast.CompileValidationPlan(at, plan.Timeframe, plan)
	if err != nil {
		return forecast.CompiledValidationPlan{}, forecast.ValidationPlan{}, forecast.TargetSpec{}, err
	}
	return compiled, plan, spec, nil
}

// BindResearchTarget returns the frozen research TargetSpec whose Identity digest equals want.
// Only V1 ResearchTargetSpec and Target C are legal. Not a model registry.
func BindResearchTarget(want forecast.Digest) (forecast.TargetSpec, error) {
	c, err := ResearchTargetSpecC()
	if err != nil {
		return forecast.TargetSpec{}, err
	}
	cid, err := c.Identity()
	if err != nil {
		return forecast.TargetSpec{}, err
	}
	if cid.Digest == want {
		return c, nil
	}
	v1, err := ResearchTargetSpec()
	if err != nil {
		return forecast.TargetSpec{}, err
	}
	vid, err := v1.Identity()
	if err != nil {
		return forecast.TargetSpec{}, err
	}
	if vid.Digest == want {
		return v1, nil
	}
	return forecast.TargetSpec{}, fmt.Errorf("market: unknown research TargetDigest %s", want)
}
