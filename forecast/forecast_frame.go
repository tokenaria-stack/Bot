package forecast

import (
	"fmt"
	"math"
)

// probabilitySumTolerance is the small numerical tolerance for PUp+PDown+
// PTimeout ≈ 1 (FORECAST-SPEC-1 §26).
const probabilitySumTolerance = 1e-6

// ForecastFrame is the minimal future output payload contract
// (FORECAST-SPEC-1 §25). Full runtime evaluation does not exist yet — this
// type and its validators exist so the shape and fail-closed gate cannot
// drift once a real model is wired.
type ForecastFrame struct {
	// At is the closed-bar identity: Unix milliseconds, UTC. It is NOT a
	// claim that the forecast was actionable at that bar's open
	// (FORECAST-SPEC-1 §21).
	At int64

	PUp      float64
	PDown    float64
	PTimeout float64

	// LongRank / ShortRank are RankPercentile in [0,1]: the OOF percentile of
	// the directional logit relative to TIMEOUT, NOT a probability of
	// winning. UI may still label this "Ranking Confidence".
	LongRank  float64
	ShortRank float64

	// ArtifactID is the immutable ForecastArtifact digest that produced this
	// frame — never a friendly label like "BTC15 v18".
	ArtifactID Digest
}

// ValidateFeatureVector rejects any nonfinite value in a filled feature
// vector. A future FeaturePlan.Fill implementation MUST call this (or an
// equivalent check) before the vector is considered usable — NaN/±Inf must
// never reach a scaler/model (FORECAST-SPEC-1 §9).
func ValidateFeatureVector(v []float64) error {
	for i, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Errorf("forecast: feature vector index %d is not finite", i)
		}
	}
	return nil
}

// ValidateForecastFrame enforces probability sanity (FORECAST-SPEC-1 §26):
// PUp/PDown/PTimeout and LongRank/ShortRank must all be finite, probabilities
// must lie in [0,1] and sum to ~1, and ranks must lie in [0,1].
func ValidateForecastFrame(f ForecastFrame) error {
	fields := map[string]float64{
		"PUp": f.PUp, "PDown": f.PDown, "PTimeout": f.PTimeout,
		"LongRank": f.LongRank, "ShortRank": f.ShortRank,
	}
	for name, v := range fields {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("forecast: %s is not finite", name)
		}
	}
	for name, v := range map[string]float64{"PUp": f.PUp, "PDown": f.PDown, "PTimeout": f.PTimeout} {
		if v < 0 || v > 1 {
			return fmt.Errorf("forecast: %s=%v out of [0,1]", name, v)
		}
	}
	if sum := f.PUp + f.PDown + f.PTimeout; math.Abs(sum-1) > probabilitySumTolerance {
		return fmt.Errorf("forecast: PUp+PDown+PTimeout=%v, expected ~1", sum)
	}
	for name, v := range map[string]float64{"LongRank": f.LongRank, "ShortRank": f.ShortRank} {
		if v < 0 || v > 1 {
			return fmt.Errorf("forecast: %s=%v out of [0,1]", name, v)
		}
	}
	return nil
}

// PublishForecastFrame is the single fail-closed gate every future runtime
// path must go through before emitting a frame (FORECAST-SPEC-1 §8/§43):
// NotReady or an invalid/nonfinite probability set produces NO frame — never
// a zero-filled, stale, or best-effort substitute.
func PublishForecastFrame(ready Ready, f ForecastFrame) (ForecastFrame, error) {
	if !ready {
		return ForecastFrame{}, fmt.Errorf("forecast: not ready — withholding forecast frame (fail closed)")
	}
	if err := ValidateForecastFrame(f); err != nil {
		return ForecastFrame{}, err
	}
	return f, nil
}
