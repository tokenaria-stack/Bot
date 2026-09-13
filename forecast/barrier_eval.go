package forecast

import (
	"fmt"
)

// FinerBarrierResolver is the LABEL-SET-1B 1m dual-hit owner, exposed so a
// second consumer (CANDIDATE-GEOMETRY-1) can reuse the same path truth
// without copying the walker. It is not a TargetSpec and not a ticket engine.
type FinerBarrierResolver struct {
	inner *finerResolve
}

// NewFinerBarrierResolver constructs the canonical finer dual-hit resolver.
func NewFinerBarrierResolver(market MarketKey, primaryTF string, bars []CanonicalClosedBar) *FinerBarrierResolver {
	return &FinerBarrierResolver{inner: newFinerResolve(market, primaryTF, bars)}
}

// EvaluateBarriers runs LABEL-SET-1A firstPassage on explicit price barriers,
// then LABEL-SET-1B finer resolution when the primary bar is a dual-hit.
//
// This is the shared primitive TargetSpec labeling already uses internally.
// Geometry research must call this instead of scanning High/Low itself.
func EvaluateBarriers(bars []CanonicalClosedBar, t int, upper, lower float64, horizon int, interval string, finer *FinerBarrierResolver) (LabelRow, error) {
	if t < 0 || t >= len(bars) {
		return LabelRow{}, fmt.Errorf("forecast: barrier candidate index %d out of range", t)
	}
	if !isFinite(upper) || !isFinite(lower) {
		return LabelRow{}, fmt.Errorf("forecast: nonfinite explicit barriers at %d", bars[t].OpenTime)
	}
	if upper <= lower {
		return LabelRow{}, fmt.Errorf("forecast: upper barrier must exceed lower")
	}
	outcome, hitAt, reason, err := firstPassage(bars, t, upper, lower, horizon, interval)
	if err != nil {
		return LabelRow{}, err
	}
	row := LabelRow{At: bars[t].OpenTime, Outcome: outcome, HitAt: hitAt, Reason: reason}
	if outcome == OutcomeAmbiguous && reason == ReasonDualHit && finer != nil && finer.inner != nil {
		o, r, err := finer.inner.resolve(bars[t].OpenTime, hitAt, upper, lower)
		if err != nil {
			return LabelRow{}, err
		}
		if o == OutcomeUpFirst || o == OutcomeDownFirst {
			row = LabelRow{At: bars[t].OpenTime, Outcome: o, HitAt: hitAt, Reason: ReasonNone}
		} else {
			row = LabelRow{At: bars[t].OpenTime, Outcome: OutcomeAmbiguous, HitAt: hitAt, Reason: r}
		}
	}
	if err := validateLabelRow(row); err != nil {
		return LabelRow{}, err
	}
	return row, nil
}
