package strategy

import (
	"context"
	"errors"
	"fmt"
)

// Entry risk errors returned by RiskManager.ValidateEntry.
var (
	ErrFeeBarrier  = errors.New("fee barrier")
	ErrClimaxStorm = errors.New("climax storm")
	ErrMacroFilter = errors.New("macro filter")
	ErrAIVeto      = errors.New("ai veto")
)

// RiskManager is the unified entry gate for all pre-trade vetoes.
type RiskManager struct {
	feeRate     float64
	memory      scalpMemory
	sandboxMode bool
}

// NewRiskManager creates an entry risk gate with fee rate and optional AI memory.
func NewRiskManager(feeRate float64, memory scalpMemory, sandboxMode bool) *RiskManager {
	if feeRate <= 0 {
		feeRate = DefaultScalpFeeRate
	}
	return &RiskManager{
		feeRate:     feeRate,
		memory:      memory,
		sandboxMode: sandboxMode || SandboxModeEnabled(),
	}
}

// SetSandboxMode updates sandbox bypass at runtime (e.g. after config load).
func (rm *RiskManager) SetSandboxMode(enabled bool) {
	rm.sandboxMode = enabled || SandboxModeEnabled()
}

// ValidateEntry runs all entry vetoes for the given side ("BUY" or "SELL").
// Returns nil when the trade may proceed. Sandbox mode bypasses all checks.
func (rm *RiskManager) ValidateEntry(report *Report, signal string) error {
	if rm == nil {
		return nil
	}
	if rm.sandboxMode || SandboxModeEnabled() {
		return nil
	}
	if report == nil {
		return fmt.Errorf("nil report")
	}

	minMove := report.Close * (rm.feeRate * scalpFeeMultiplier)
	if report.Volatility.ATR < minMove {
		return ErrFeeBarrier
	}

	if report.Volatility.Regime == RegimeClimax {
		return ErrClimaxStorm
	}

	if !IsStrongRSXReversalMarker(report.RSXMarker) {
		switch signal {
		case "BUY":
			if !report.JurikIsRising && report.JurikValue < 50 {
				return ErrMacroFilter
			}
		case "SELL":
			if report.JurikIsRising && report.JurikValue > 50 {
				return ErrMacroFilter
			}
		}
	}

	if err := rm.aiVeto(context.Background(), report); err != nil {
		return err
	}

	return nil
}

func (rm *RiskManager) aiVeto(ctx context.Context, report *Report) error {
	if rm.memory == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	winRate, count, err := rm.memory.PredictWinRate(ctx, report.VectorSnapshot(), aiSearchLimit)
	if err != nil || count < aiVetoMinNeighbors || winRate >= aiVetoMaxWinRate {
		return nil
	}
	return ErrAIVeto
}

// RiskErrorLabel maps a risk error to a dashboard-friendly label.
func RiskErrorLabel(err error) string {
	switch {
	case errors.Is(err, ErrFeeBarrier):
		return "Veto: Fee Barrier"
	case errors.Is(err, ErrClimaxStorm):
		return "Veto: CLIMAX"
	case errors.Is(err, ErrMacroFilter):
		return "Veto: Macro Filter"
	case errors.Is(err, ErrAIVeto):
		return "AI Warning"
	default:
		return "Veto Active"
	}
}

// TelemetryBrainStatus maps scoring + entry risk to a dashboard status label.
func TelemetryBrainStatus(decision ScalpDecision, report Report, entryRisk *RiskManager) string {
	if decision.Action == BuyAction || decision.Action == SellAction {
		if entryRisk != nil {
			if err := entryRisk.ValidateEntry(&report, string(decision.Action)); err != nil {
				return RiskErrorLabel(err)
			}
		}
		return "Clear"
	}
	if decision.LongScore == 0 && decision.ShortScore == 0 {
		return "Analyzing..."
	}
	if scoringBelowThreshold(decision) {
		return "Below Threshold"
	}
	return "Clear"
}
