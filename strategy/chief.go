package strategy

// ChiefAnalyst is the final approval pipe between Analyst and order execution.
// Phase 1: transparent pass-through; logic will be added in later phases.
type ChiefAnalyst struct{}

// NewChiefAnalyst creates the chief approval pipe.
func NewChiefAnalyst() *ChiefAnalyst {
	return &ChiefAnalyst{}
}

// Approve forwards an entry decision unchanged toward execution (Shooter).
func (c *ChiefAnalyst) Approve(decision ScalpDecision, _ Report) ScalpDecision {
	if c == nil {
		return decision
	}
	return decision
}
