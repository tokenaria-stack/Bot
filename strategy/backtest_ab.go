package strategy

import (
	"context"

	"trading_bot/exchange"
)

// ABRunSpec is one scoring-matrix variant in an A/B backtest grid.
type ABRunSpec struct {
	Label      string
	Matrix     ScoringMatrix
	Navigators map[string]NavigatorUISettings
	MtfOptions map[string]bool
}

// BaselineABMatrix returns Config A: LTF baseline without HTF oscillators.
func BaselineABMatrix(base ScoringMatrix) ScoringMatrix {
	m := base
	m.UseHTFOscillators = false
	return m
}

// HTFRegimeABMatrix returns Config B: HTF Wozduh/RSX regime, quieter LTF noise.
func HTFRegimeABMatrix(base ScoringMatrix) ScoringMatrix {
	m := base
	m.UseHTFOscillators = true
	m.UseWozduhCross = false
	m.UseWozduhSpike = false
	m.UseRedCross = false
	return m
}

// DefaultABRunSpecs builds the standard Baseline vs HTF Regime pair from a base matrix.
func DefaultABRunSpecs(base ScoringMatrix) []ABRunSpec {
	navBase := DefaultLiveNavigatorPanes()
	navHTF := cloneNavigatorPanes(navBase)
	ApplyMtfOptionsToNavigators(navHTF, map[string]bool{
		"4h": true,
		"1d": true,
	})

	return []ABRunSpec{
		{
			Label:      "A (Baseline LTF)",
			Matrix:     BaselineABMatrix(base),
			Navigators: cloneNavigatorPanes(navBase),
		},
		{
			Label:      "B (HTF Regime)",
			Matrix:     HTFRegimeABMatrix(base),
			Navigators: navHTF,
			MtfOptions: map[string]bool{"4h": true, "1d": true},
		},
	}
}

func cloneNavigatorPanes(in map[string]NavigatorUISettings) map[string]NavigatorUISettings {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]NavigatorUISettings, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// RunBacktestSimulation executes the same engine path as POST /api/backtest/run.
func RunBacktestSimulation(ctx context.Context, cfg BacktestConfig, candles []exchange.Candle) (*BacktestRunResult, error) {
	engine := NewBacktestEngine(cfg)
	return engine.Run(ctx, candles)
}

// BuildBacktestEngineConfig mirrors server/webserver.go handleBacktestRun wiring.
func BuildBacktestEngineConfig(
	symbol, interval string,
	matrix *ScoringMatrix,
	navigators map[string]NavigatorUISettings,
	mtfOptions map[string]bool,
	htf *exchange.HTFProvider,
	entryAnalyst *Analyst,
	slippagePct float64,
	longThreshold, shortThreshold int,
	rsxSettings *RSXSettings,
) BacktestConfig {
	navs := cloneNavigatorPanes(navigators)
	if len(navs) == 0 {
		navs = map[string]NavigatorUISettings{}
	}
	ApplyMtfOptionsToNavigators(navs, mtfOptions)
	if matrix != nil {
		EnsureBacktestNavigatorsForMatrix(navs, *matrix)
	}

	return BacktestConfig{
		Symbol:         symbol,
		Interval:       interval,
		EntryAnalyst:   entryAnalyst,
		FeeRate:        DefaultScalpFeeRate,
		SlippagePct:    slippagePct,
		Matrix:         matrix,
		Navigators:     navs,
		HTF:            htf,
		LongThreshold:  longThreshold,
		ShortThreshold: shortThreshold,
		RSXSettings:    rsxSettings,
	}
}
