package market

import (
	"fmt"

	"trading_bot/exchange"
	"trading_bot/forecast"
)

// ResearchMarketKey is the first four-feature experiment market (USD-M BTCUSDT 15m).
func ResearchMarketKey() forecast.MarketKey {
	return forecast.MarketKey{
		Venue:      "BINANCE",
		Instrument: "BTCUSDT",
		Contract:   "FUTURES_PERP",
		Timeframe:  "15m",
	}
}

// ResearchSourceStartMs is the earliest OpenTime allowed for a research FeatureTape
// on this MarketKey. FUTURES_PERP uses Binance USD-M listing genesis — not the
// chart continuous-contract spot stitch.
func ResearchSourceStartMs(key forecast.MarketKey) int64 {
	if key.Contract == "FUTURES_PERP" {
		return exchange.BinanceFuturesGenesisMs
	}
	return 0
}

// ResearchRSXSettings is live default RSX (length 14, signal 9, hlc3, div lookback 90).
func ResearchRSXSettings() RSXSettings {
	return NormalizeRSXSettings(defaultRSXSettings())
}

// ResearchFeaturePlan binds the frozen four-column FeatureRecipe to default RSX
// settings under the given AnalysisLogicVersion. FeatureRecipe / plan logic stay features:v1 / plan:v1.
func ResearchFeaturePlan(analysisLogic forecast.LogicVersion) (forecast.FeaturePlan, error) {
	analysis, err := AnalysisRecipeFromRSXSettings(ResearchRSXSettings(), true, false, analysisLogic)
	if err != nil {
		return forecast.FeaturePlan{}, err
	}
	features, err := forecast.ResolveFeatureRecipe("tape1a", forecast.FeatureRecipeDraft{
		Features: []forecast.FeatureID{
			forecast.FeatureRSXValue,
			forecast.FeatureRSXSignal,
			forecast.FeatureTVBullPresent,
			forecast.FeatureTVBullAge,
		},
	}, "features:v1")
	if err != nil {
		return forecast.FeaturePlan{}, err
	}
	plan, err := forecast.BindFeaturePlan(analysis, features, "plan:v1")
	if err != nil {
		return forecast.FeaturePlan{}, err
	}
	return plan, nil
}

// ResearchTargetSpec is the intended first-passage research target (15m→1m).
func ResearchTargetSpec() (forecast.TargetSpec, error) {
	return forecast.ResolveTargetSpec("research-15m-1m", forecast.TargetSpecDraft{
		HorizonBars:      24,
		UpperATRMultiple: 1.5,
		LowerATRMultiple: 1.0,
		ATRPeriod:        14,
		DualHit:          forecast.DualHitResolveFinerHistory,
		FinerTimeframe:   "1m",
	}, "labels:v1")
}

// ResearchHoldoutStartAt is the sealed experiment wall (2026-01-01 00:00:00 UTC).
// Shared by model VALIDATION-PLAN-1 and DECISION-VALIDATION-PLAN-1.
func ResearchHoldoutStartAt() int64 {
	return 1767225600000
}

// ResearchValidationPlan is the pinned BTCUSDT 15m walk-forward experiment.
// TargetH is taken from ResearchTargetSpec, not a parallel constant.
func ResearchValidationPlan() (forecast.ValidationPlan, error) {
	spec, err := ResearchTargetSpec()
	if err != nil {
		return forecast.ValidationPlan{}, err
	}
	key := ResearchMarketKey()
	return forecast.ResolveValidationPlan(forecast.ValidationPlanDraft{
		Timeframe:          key.Timeframe,
		HoldoutStartAt:     ResearchHoldoutStartAt(),
		ValidationSpanBars: 17568, // 183 elapsed days at 15m
		FoldCount:          4,
		ExtraGapBars:       0,
		MinTrainRows:       35040,
	}, spec, forecast.ValidationLogicWalkForwardV1)
}

// ResearchDecisionValidationPlan is the pinned decision-development geometry.
// Same sealed wall and TargetSpec as the model experiment; different span/identity.
// TargetH is copied from ResearchTargetSpec, not a parallel constant.
func ResearchDecisionValidationPlan() (forecast.ValidationPlan, error) {
	spec, err := ResearchTargetSpec()
	if err != nil {
		return forecast.ValidationPlan{}, err
	}
	key := ResearchMarketKey()
	return forecast.ResolveValidationPlan(forecast.ValidationPlanDraft{
		Timeframe:          key.Timeframe,
		HoldoutStartAt:     ResearchHoldoutStartAt(),
		ValidationSpanBars: 8640, // 90 elapsed days at 15m
		FoldCount:          4,
		ExtraGapBars:       0,
		MinTrainRows:       35040, // one 365-day 15m year of evidence rows
	}, spec, forecast.DecisionValidationLogicWalkForwardV1)
}

// ResearchOOFMatrixFileName is a navigation slot name under research/oof/.
// Authority is header provenance + ContentDigest, not this filename.
func ResearchOOFMatrixFileName(key forecast.MarketKey, tapeContent, labelContent, valPlan forecast.Digest) string {
	return key.Venue + "_" + key.Instrument + "_" + key.Contract + "_" + key.Timeframe +
		"_tape-" + tapeContent.Short() + "_labels-" + labelContent.Short() + "_valplan-" + valPlan.Short() + ".oofmatrix"
}

func ResearchFeaturePlanMust(analysisLogic forecast.LogicVersion) (forecast.FeaturePlan, error) {
	plan, err := ResearchFeaturePlan(analysisLogic)
	if err != nil {
		return forecast.FeaturePlan{}, err
	}
	if plan.VectorLen() != 4 {
		return forecast.FeaturePlan{}, fmt.Errorf("market: research FeaturePlan VectorLen=%d want 4", plan.VectorLen())
	}
	return plan, nil
}
