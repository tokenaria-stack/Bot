package decision

import "fmt"

// ExpectedTargetUtility is the single directional target-space quantity:
// EU = U*P_UP - L*P_DOWN. TIMEOUT has utility 0 (no explicit term).
// EU_DOWN is defined as -EU; there is no second formula.
func ExpectedTargetUtility(e ForecastEvidence, s DecisionSpec) (float64, error) {
	if err := ValidateDecisionSpec(s); err != nil {
		return 0, err
	}
	if err := ValidateForecastEvidence(e); err != nil {
		return 0, err
	}
	return expectedTargetUtility(e, s), nil
}

func expectedTargetUtility(e ForecastEvidence, s DecisionSpec) float64 {
	pUp := e.Probabilities[0]
	pDown := e.Probabilities[1]
	return s.UpperBarrierUtility*pUp - s.LowerBarrierUtility*pDown
}

// ApplyDecision maps ForecastEvidence through a resolved DecisionSpec to a
// DirectionalIntent. Invalid evidence is an error, never ABSTAIN.
func ApplyDecision(e ForecastEvidence, s DecisionSpec) (DirectionalIntent, error) {
	if err := ValidateDecisionSpec(s); err != nil {
		return "", err
	}
	if err := ValidateForecastEvidence(e); err != nil {
		return "", err
	}
	eu := expectedTargetUtility(e, s)
	minEU := s.MinExpectedTargetUtility
	minR := s.MinAbsDirectionalRank
	rank := e.DirectionalRank
	up := eu >= minEU && rank >= minR
	down := -eu >= minEU && rank <= -minR
	if up && down {
		return "", fmt.Errorf("decision: UP and DOWN intents both true")
	}
	if up {
		return IntentUp, nil
	}
	if down {
		return IntentDown, nil
	}
	return IntentAbstain, nil
}
