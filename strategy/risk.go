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

// AnalyzeSignals inspects the marker for the given side ("BUY" or "SELL").
// Phase 1: always approves — legacy vetoes removed.
func (a *Analyst) AnalyzeSignals(_ *Marker, _ string) error {
	return nil
}

// ApplyExecutionVetoes runs analyst and chief gates on a raw ScoreDecision.
// Preserves RawAction, LongScore, ShortScore, and Factors; mutates FinalAction on veto.
func ApplyExecutionVetoes(decision ScoreDecision, marker *Marker, analyst *Analyst, chief *ChiefAnalyst) ScoreDecision {
	if decision.RawAction == "" {
		decision.RawAction = decision.FinalAction
	}
	if marker != nil && !marker.HasMinBars(minScoreBars) {
		decision.IsVetoed = true
		decision.VetoReason = "System Warmup: Not enough history"
		decision.FinalAction = WaitAction
		releaseFactorsUnlessActionable(&decision)
		return decision
	}
	if !decision.HasRawSignal() {
		releaseFactorsUnlessActionable(&decision)
		return decision
	}
	if analyst != nil {
		if err := analyst.AnalyzeSignals(marker, string(decision.RawAction)); err != nil {
			decision.IsVetoed = true
			decision.VetoReason = "Analyst blocked: " + err.Error()
			decision.FinalAction = WaitAction
			releaseFactorsUnlessActionable(&decision)
			return decision
		}
	}
	if chief != nil {
		chief.Approve(&decision)
	}
	releaseFactorsUnlessActionable(&decision)
	return decision
}

// RiskErrorLabel maps a risk error to a dashboard-friendly label (legacy compat).
func RiskErrorLabel(_ error) string {
	return "Clear"
}

// TelemetryBrainStatus maps scoring to a dashboard status label.
func TelemetryBrainStatus(decision ScoreDecision, _ *Analyst) string {
	if decision.IsVetoed {
		return "Vetoed"
	}
	if decision.RawAction == BuyAction || decision.RawAction == SellAction {
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
