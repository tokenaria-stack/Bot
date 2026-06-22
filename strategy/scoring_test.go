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

func TestEvaluateScalpSignal_BuyThreshold(t *testing.T) {
	decision := EvaluateScalpSignal(context.Background(), longSignalReport(), DefaultScalpFeeRate, nil)
	if decision.Action != BuyAction {
		t.Fatalf("Action = %q, want BUY (score=%d)", decision.Action, decision.Score)
	}
	if decision.Score < LongScoreThreshold() {
		t.Fatalf("score = %d, want >= %d", decision.Score, LongScoreThreshold())
	}
}

func TestEvaluateScalpSignal_ShortThreshold(t *testing.T) {
	decision := EvaluateScalpSignal(context.Background(), shortSignalReport(), DefaultScalpFeeRate, nil)
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
	decision := EvaluateScalpSignal(context.Background(), report, DefaultScalpFeeRate, nil)
	if decision.Action != WaitAction {
		t.Fatalf("Action = %q, want WAIT", decision.Action)
	}
}

func TestEvaluateScalpSignal_ShortWinsOverLong(t *testing.T) {
	report := Report{
		Close:           100,
		RSXMarker:       "SS",
		Falcon:          FalconSignals{VolCrossMarker: "red"},
		AOCrossZeroDown: true,
		AOCrossZeroUp:   true,
		Volatility:      scalpVolatilityOK(),
	}
	decision := EvaluateScalpSignal(context.Background(), report, DefaultScalpFeeRate, nil)
	if decision.Action != SellAction {
		t.Fatalf("Action = %q, want SELL", decision.Action)
	}
}

func TestScoreLong_Components(t *testing.T) {
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
	got := scoreLong(report)
	want := scoreRSXLL + scalpVolCrossScore + scalpRedCrossScore + scalpBreakoutScore +
		20 + scalpFib618Score + scalpExpansionScore + scalpJurikBullScore +
		scalpWozduxVolumeScore + scalpGeometryBounceScore + scalpGeometryTriangleScore +
		scalpAccumulationScore + scalpAOCrossScore
	if got != want {
		t.Fatalf("scoreLong = %d, want %d", got, want)
	}
}

func TestScoreShort_Components(t *testing.T) {
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
	got := scoreShort(report)
	want := scoreRSXSS + scalpVolCrossScore + scalpRedCrossScore + scalpBreakoutScore +
		25 + scalpFib618Score + scalpExpansionScore + scalpJurikBearScore +
		scalpWozduxVolumeScore + scalpGeometryBounceScore + scalpGeometryTriangleScore +
		scalpAccumulationScore + scalpAOCrossScore
	if got != want {
		t.Fatalf("scoreShort = %d, want %d", got, want)
	}
}

func TestEvaluateScalpSignal_FullMatrixLong(t *testing.T) {
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
	decision := EvaluateScalpSignal(context.Background(), report, DefaultScalpFeeRate, nil)
	if decision.Action != BuyAction {
		t.Fatalf("Action = %q, want BUY (score=%d)", decision.Action, decision.Score)
	}
}
