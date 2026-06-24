package strategy

import (
	"context"
	"testing"

	"trading_bot/indicators"
)

func scalpVolatilityOK() VolatilityState {
	return VolatilityState{
		ATR:          1.0,
		Regime:       RegimeExpansion,
		LotModifier:  1.0,
		SafeStopDist: 1.5,
	}
}

func longSignalReport() Report {
	return Report{
		Close:         100,
		RSXMarker:     "L",
		Falcon:        FalconSignals{VolCrossMarker: "lime"},
		AOCrossZeroUp: true,
		Volatility:    scalpVolatilityOK(),
	}
}

func shortSignalReport() Report {
	return Report{
		Close:           100,
		RSXMarker:       "S",
		Falcon:          FalconSignals{VolCrossMarker: "red"},
		AOCrossZeroDown: true,
		Volatility:      scalpVolatilityOK(),
	}
}

func scalpDecisionFromReport(ctx context.Context, report Report) ScalpDecision {
	result := ProcessScore(ctx, report, DefaultScalpFeeRate, nil)
	return ScalpDecisionFromScoreResult(result, report)
}

func TestProcessScore_BuyThreshold(t *testing.T) {
	SetScoringMatrix(allEnabledScoringMatrix())
	t.Cleanup(ResetScoringMatrix)

	decision := scalpDecisionFromReport(context.Background(), longSignalReport())
	if decision.Action != BuyAction {
		t.Fatalf("Action = %q, want BUY (score=%d)", decision.Action, decision.Score)
	}
	if decision.Score < LongScoreThreshold() {
		t.Fatalf("score = %d, want >= %d", decision.Score, LongScoreThreshold())
	}
}

func TestProcessScore_ShortThreshold(t *testing.T) {
	SetScoringMatrix(allEnabledScoringMatrix())
	t.Cleanup(ResetScoringMatrix)

	decision := scalpDecisionFromReport(context.Background(), shortSignalReport())
	if decision.Action != SellAction {
		t.Fatalf("Action = %q, want SELL (score=%d)", decision.Action, decision.Score)
	}
}

func TestEvaluateScalpSignal_WaitBelowThreshold(t *testing.T) {
	report := Report{
		Close:      100,
		RSXMarker:  "L",
		Volatility: scalpVolatilityOK(),
	}
	decision := scalpDecisionFromReport(context.Background(), report)
	if decision.Action != WaitAction {
		t.Fatalf("Action = %q, want WAIT", decision.Action)
	}
}

func TestEvaluateScalpSignal_ShortWinsOverLong(t *testing.T) {
	SetScoringMatrix(allEnabledScoringMatrix())
	t.Cleanup(ResetScoringMatrix)

	report := Report{
		Close:           100,
		RSXMarker:       "SS",
		Falcon:          FalconSignals{VolCrossMarker: "red"},
		AOCrossZeroDown: true,
		AOCrossZeroUp:   true,
		Volatility:      scalpVolatilityOK(),
	}
	decision := scalpDecisionFromReport(context.Background(), report)
	if decision.Action != SellAction {
		t.Fatalf("Action = %q, want SELL", decision.Action)
	}
}

func TestScoreLong_Components(t *testing.T) {
	SetScoringMatrix(allEnabledScoringMatrix())
	t.Cleanup(ResetScoringMatrix)

	report := Report{
		RSXMarker:             "LL",
		Falcon:                FalconSignals{VolCrossMarker: "lime"},
		RedLineCrossGreenUp:   true,
		Geometry:              GeometryState{IsBullishBreakout: true},
		Divergence:            indicators.DivSignal{Score: 20},
		FibZones:              []indicators.FibZone{{Ratio: 0.618, IsActive: true}},
		Volatility:            VolatilityState{Regime: RegimeExpansion},
		JurikIsRising:         true,
		JurikValue:            60,
		WozduxVolumeSpikeUp:   true,
		GeometryBounceUp:      true,
		GeometryTriangle:      true,
		AccumulationRising:    true,
		AOCrossZeroUp:         true,
	}
	got := scalpDecisionFromReport(context.Background(), report).LongScore
	want := scoreRSXLL + scalpVolCrossScore + scalpRedCrossScore + scalpBreakoutScore +
		20 + scalpFib618Score + scalpExpansionScore + scalpJurikBullScore +
		scalpWozduxVolumeScore + scalpGeometryBounceScore + scalpGeometryTriangleScore +
		scalpAccumulationScore + scalpAOCrossScore
	if got != want {
		t.Fatalf("LongScore = %d, want %d", got, want)
	}
}

func TestScoreShort_Components(t *testing.T) {
	SetScoringMatrix(allEnabledScoringMatrix())
	t.Cleanup(ResetScoringMatrix)

	report := Report{
		RSXMarker:               "SS",
		Falcon:                  FalconSignals{VolCrossMarker: "red"},
		RedLineCrossGreenDown:   true,
		Geometry:                GeometryState{IsBearishBreakout: true},
		Divergence:              indicators.DivSignal{Score: -25},
		FibZones:                []indicators.FibZone{{Ratio: 0.618, IsActive: true}},
		Volatility:              VolatilityState{Regime: RegimeExpansion},
		JurikIsRising:           false,
		JurikValue:              40,
		WozduxVolumeSpikeDown:   true,
		GeometryBounceDown:      true,
		GeometryTriangle:        true,
		DistributionFalling:     true,
		AOCrossZeroDown:         true,
	}
	got := scalpDecisionFromReport(context.Background(), report).ShortScore
	want := scoreRSXSS + scalpVolCrossScore + scalpRedCrossScore + scalpBreakoutScore +
		25 + scalpFib618Score + scalpExpansionScore + scalpJurikBearScore +
		scalpWozduxVolumeScore + scalpGeometryBounceScore + scalpGeometryTriangleScore +
		scalpAccumulationScore + scalpAOCrossScore
	if got != want {
		t.Fatalf("ShortScore = %d, want %d", got, want)
	}
}

func TestProcessScore_FullMatrixLong(t *testing.T) {
	SetScoringMatrix(allEnabledScoringMatrix())
	t.Cleanup(ResetScoringMatrix)

	report := Report{
		Close:               100,
		RedLineCrossGreenUp: true,
		JurikValue:          60,
		JurikIsRising:       true,
		Geometry:            GeometryState{IsBullishBreakout: true},
		Divergence:          indicators.DivSignal{Score: 20},
		FibZones:            []indicators.FibZone{{Ratio: 0.618, IsActive: true}},
		Volatility:          scalpVolatilityOK(),
		WozduxVolumeSpikeUp: true,
		GeometryBounceUp:    true,
		GeometryTriangle:    true,
		AccumulationRising:  true,
		AOCrossZeroUp:       true,
	}
	decision := scalpDecisionFromReport(context.Background(), report)
	if decision.Action != BuyAction {
		t.Fatalf("Action = %q, want BUY (score=%d)", decision.Action, decision.Score)
	}
}
