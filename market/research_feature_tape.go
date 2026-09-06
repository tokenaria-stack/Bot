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
