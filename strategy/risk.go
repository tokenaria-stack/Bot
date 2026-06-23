package strategy

// Analyst is the signal analysis gate (formerly entry risk vetoes).
// Phase 1: transparent pass-through pipe.
type Analyst struct {
	sandboxMode bool
}

// NewAnalyst creates the signal analysis module.
func NewAnalyst(sandboxMode bool) *Analyst {
	return &Analyst{sandboxMode: sandboxMode || SandboxModeEnabled()}
}

// SetSandboxMode updates sandbox flag at runtime (e.g. after config load).
func (a *Analyst) SetSandboxMode(enabled bool) {
	if a == nil {
		return
	}
	a.sandboxMode = enabled || SandboxModeEnabled()
}

// AnalyzeSignals inspects the report for the given side ("BUY" or "SELL").
// Phase 1: always approves — legacy vetoes removed.
func (a *Analyst) AnalyzeSignals(_ *Report, _ string) error {
	return nil
}

// RiskErrorLabel maps a risk error to a dashboard-friendly label (legacy compat).
func RiskErrorLabel(_ error) string {
	return "Clear"
}

// TelemetryBrainStatus maps scoring to a dashboard status label.
func TelemetryBrainStatus(decision ScalpDecision, _ Report, _ *Analyst) string {
	if decision.Action == BuyAction || decision.Action == SellAction {
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
