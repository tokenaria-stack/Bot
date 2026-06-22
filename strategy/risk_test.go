package strategy

import (
	"context"
	"errors"
	"testing"

	"trading_bot/vector_db"
)

func TestRiskManager_FeeBarrier(t *testing.T) {
	rm := NewRiskManager(0.001, nil, false)
	report := &Report{
		Close: 100,
		Volatility: VolatilityState{
			ATR: 0.01,
		},
		JurikValue:    55,
		JurikIsRising: true,
	}
	if err := rm.ValidateEntry(report, "BUY"); !errors.Is(err, ErrFeeBarrier) {
		t.Fatalf("ValidateEntry() = %v, want ErrFeeBarrier", err)
	}
}

func TestRiskManager_ClimaxStorm(t *testing.T) {
	rm := NewRiskManager(0.0001, nil, false)
	report := &Report{
		Close: 100,
		Volatility: VolatilityState{
			ATR:    1.0,
			Regime: RegimeClimax,
		},
		JurikValue:    55,
		JurikIsRising: true,
	}
	if err := rm.ValidateEntry(report, "BUY"); !errors.Is(err, ErrClimaxStorm) {
		t.Fatalf("ValidateEntry() = %v, want ErrClimaxStorm", err)
	}
}

func TestRiskManager_MacroFilterLong(t *testing.T) {
	rm := NewRiskManager(0.0001, nil, false)
	report := &Report{
		Close: 100,
		Volatility: VolatilityState{
			ATR:    1.0,
			Regime: RegimeExpansion,
		},
		JurikValue:    40,
		JurikIsRising: false,
	}
	if err := rm.ValidateEntry(report, "BUY"); !errors.Is(err, ErrMacroFilter) {
		t.Fatalf("ValidateEntry() = %v, want ErrMacroFilter", err)
	}
}

func TestRiskManager_AIVeto(t *testing.T) {
	memory := stubScalpMemory{winRate: 0.2, count: 5}
	rm := NewRiskManager(0.0001, memory, false)
	report := &Report{
		Close: 100,
		Volatility: VolatilityState{
			ATR:    1.0,
			Regime: RegimeExpansion,
		},
		JurikValue:    60,
		JurikIsRising: true,
	}
	if err := rm.ValidateEntry(report, "BUY"); !errors.Is(err, ErrAIVeto) {
		t.Fatalf("ValidateEntry() = %v, want ErrAIVeto", err)
	}
}

func TestRiskManager_SandboxBypass(t *testing.T) {
	rm := NewRiskManager(0.001, nil, true)
	report := &Report{
		Close: 100,
		Volatility: VolatilityState{
			ATR:    0.001,
			Regime: RegimeClimax,
		},
		JurikValue:    10,
		JurikIsRising: false,
	}
	if err := rm.ValidateEntry(report, "BUY"); err != nil {
		t.Fatalf("sandbox ValidateEntry() = %v, want nil", err)
	}
}

func TestTelemetryBrainStatus_RiskVeto(t *testing.T) {
	rm := NewRiskManager(0.001, nil, false)
	report := Report{
		Close: 100,
		RSXMarker: "L",
		Falcon: FalconSignals{VolCrossMarker: "lime"},
		Volatility: VolatilityState{
			ATR: 0.01,
		},
		JurikValue:    55,
		JurikIsRising: true,
	}
	decision := EvaluateScalpSignal(context.Background(), report, DefaultScalpFeeRate, nil)
	if decision.Action != BuyAction {
		t.Fatalf("expected BUY signal for telemetry test, got %q", decision.Action)
	}
	status := TelemetryBrainStatus(decision, report, rm)
	if status != "Veto: Fee Barrier" {
		t.Fatalf("status = %q, want Veto: Fee Barrier", status)
	}
}

// Ensure stubScalpMemory satisfies scalpMemory via risk tests importing scoring_test helper.
type stubScalpMemory struct {
	winRate float64
	count   int
	err     error
}

func (s stubScalpMemory) PredictWinRate(_ context.Context, _ vector_db.ReportSnapshot, _ uint64) (float64, int, error) {
	return s.winRate, s.count, s.err
}
