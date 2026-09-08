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
