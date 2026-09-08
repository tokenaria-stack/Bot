package decision

import (
	"fmt"
	"math"

	"trading_bot/forecast"
)

// DecisionLogicTargetUtilityRankGateV1 is the v1 runtime decision language.
const DecisionLogicTargetUtilityRankGateV1 forecast.LogicVersion = "decision:target-utility-rank-gate-v1"

// DirectionalIntent is a statistical opinion, not an order or trade.
type DirectionalIntent string

const (
	IntentUp      DirectionalIntent = "UP_INTENT"
	IntentDown    DirectionalIntent = "DOWN_INTENT"
	IntentAbstain DirectionalIntent = "ABSTAIN"
)

// ForecastEvidence is the frozen recipe/evidence payload: calibrated class
// probabilities in canonical class_order and signed directional rank in [-1,+1].
// This is the same schema as research ForecastEvidence, not a DecisionInput clone.
type ForecastEvidence struct {
	Probabilities   [3]float64
	DirectionalRank float64
}

// DecisionSpec is an immutable runtime rule. Barrier utilities are TargetSpec
// ATR-multiple copies (target-space units, not PnL). min_EU and min_rank are
// the only future search knobs; this chapter does not choose them.
type DecisionSpec struct {
	Logic                    forecast.LogicVersion
	TargetDigest             forecast.Digest
	UpperBarrierUtility      float64
	LowerBarrierUtility      float64
	MinExpectedTargetUtility float64
	MinAbsDirectionalRank    float64
	ClassOrder               [3]string
}

func canonicalClassOrder() [3]string {
	return [3]string{
		string(forecast.OutcomeUpFirst),
		string(forecast.OutcomeDownFirst),
		string(forecast.OutcomeTimeout),
	}
}

// ValidateDecisionSpec enforces structural v1 bounds. No repair.
func ValidateDecisionSpec(s DecisionSpec) error {
	if s.Logic != DecisionLogicTargetUtilityRankGateV1 {
		return fmt.Errorf("decision: unknown logic %q", s.Logic)
	}
	var z forecast.Digest
	if s.TargetDigest == z {
		return fmt.Errorf("decision: target_digest empty")
	}
	want := canonicalClassOrder()
	if s.ClassOrder != want {
		return fmt.Errorf("decision: class_order must be exact %v", want)
	}
	u, l := s.UpperBarrierUtility, s.LowerBarrierUtility
	if !finite(u) || !finite(l) {
		return fmt.Errorf("decision: barrier utility must be finite")
	}
	if u <= 0 || l <= 0 {
		return fmt.Errorf("decision: barrier utility must be > 0")
	}
	minEU := s.MinExpectedTargetUtility
	minR := s.MinAbsDirectionalRank
	if !finite(minEU) || !finite(minR) {
		return fmt.Errorf("decision: thresholds must be finite")
	}
	cap := math.Min(u, l)
	if !(minEU > 0 && minEU < cap) {
		return fmt.Errorf("decision: require 0 < min_EU < min(U,L)")
	}
	if !(minR > 0 && minR < 1) {
		return fmt.Errorf("decision: require 0 < min_abs_rank < 1")
	}
	return nil
}

// ValidateForecastEvidence uses the same fail-closed bounds as the frozen
// recipe projector / evidence rows: length 3, finite P in [0,1], finite rank in [-1,+1].
// No sum-to-1 epsilon, clip, or NaN→ABSTAIN.
func ValidateForecastEvidence(e ForecastEvidence) error {
	for i, p := range e.Probabilities {
		if !finite(p) {
			return fmt.Errorf("decision: probability[%d] not finite", i)
		}
		if p < 0 || p > 1 {
			return fmt.Errorf("decision: probability[%d] out of [0,1]", i)
		}
	}
	r := e.DirectionalRank
	if !finite(r) {
		return fmt.Errorf("decision: directional_rank not finite")
	}
	if r < -1 || r > 1 {
		return fmt.Errorf("decision: directional_rank out of [-1,+1]")
	}
	return nil
}

func finite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}
