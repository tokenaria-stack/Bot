package strategy

// ChiefAnalyst is the final approval gate before execution.
type ChiefAnalyst struct{}

// NewChiefAnalyst creates the chief approval module.
func NewChiefAnalyst() *ChiefAnalyst {
	return &ChiefAnalyst{}
}

// Approve may veto a raw signal. Mutates decision in place; never clears Factors or scores.
func (c *ChiefAnalyst) Approve(decision *ScoreDecision) {
	if c == nil || decision == nil {
		return
	}
	// Phase 1: pass-through.
	//
	// Future veto example:
	// if someChiefConditionFails {
	//     decision.IsVetoed = true
	//     decision.VetoReason = "Chief: macro regime blocked"
	//     decision.FinalAction = WaitAction
	//     return
	// }
}
