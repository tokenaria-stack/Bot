package forecast

import "fmt"

// Ready distinguishes "not enough analytical truth to evaluate" from "fully
// evaluable" (FORECAST-SPEC-1 §7). It is a different concept from a feature
// being legitimately absent while the system IS Ready.
type Ready bool

const (
	NotReady Ready = false
	IsReady  Ready = true
)

// FeaturePlan is a compiled, flat binding of one AnalysisRecipe and one
// FeatureRecipe — not a node graph, registry, plugin system, or event bus
// (FORECAST-SPEC-1 §6).
//
// FEATURE-TAPE-1 will add the real hot-path method directly on this type
// once a live AnalysisRuntime source exists:
//
//	func (p *FeaturePlan) Fill(src <AnalysisRuntime>, dst []float64) (Ready, error)
//
// Fill MUST NOT allocate, MUST write exactly VectorLen() values into a
// caller-owned dst reused across bars, and MUST leave dst undefined (never
// silently reused as the current bar's vector — Stale Buffer Law,
// FORECAST-SPEC-1 §8) whenever it returns NotReady. No separate filler
// interface/abstraction is introduced ahead of that real consumer.
type FeaturePlan struct {
	Analysis Identity
	Features Identity
	Logic    LogicVersion // versions the AnalysisRecipe+FeatureRecipe combination law itself
	Schema   []FeatureID  // fixed vector layout, in FeatureRecipe order

	// FeatureHistoryBars is the FEATURE-SIDE history contribution only (see
	// FeatureRecipe.FeatureHistoryBars). It is NOT the complete "reconstructable
	// live" requirement — AnalysisRecipe/Jurik/detector reconstruction is not
	// folded in here yet. The complete closure is reserved under the name
	// RequiredHistoryBars, implemented in FEATURE-TAPE-1.
	FeatureHistoryBars int
}

// VectorLen is the fixed feature-vector length this plan projects into. It
// never varies per bar (FORECAST-SPEC-1 §6).
func (p FeaturePlan) VectorLen() int { return len(p.Schema) }

type featurePlanIdentityPayload struct {
	Analysis Digest
	Features Digest
	Logic    LogicVersion
	Schema   []FeatureID
}

// Identity returns this plan's full digest identity.
func (p FeaturePlan) Identity() (Identity, error) {
	humanKey := fmt.Sprintf("plan-%s-%s", p.Analysis.HumanKey, p.Features.HumanKey)
	return NewIdentity(humanKey, featurePlanIdentityPayload{
		Analysis: p.Analysis.Digest,
		Features: p.Features.Digest,
		Logic:    p.Logic,
		Schema:   p.Schema,
	}, p.Logic)
}

// BindFeaturePlan compiles one AnalysisRecipe and one FeatureRecipe into a
// FeaturePlan. Bind FAILS (returns an error, never a partial/zero-filled
// plan) when a requested feature's required AnalysisCapability is absent
// from the AnalysisRecipe (FORECAST-SPEC-1 §6/§43).
func BindFeaturePlan(analysis AnalysisRecipe, features FeatureRecipe, planLogic LogicVersion) (FeaturePlan, error) {
	if planLogic == "" {
		return FeaturePlan{}, fmt.Errorf("forecast: feature plan requires a LogicVersion")
	}
	caps := analysis.Capabilities()
	for _, f := range features.Features {
		need, ok := featureCapability[f]
		if !ok {
			return FeaturePlan{}, fmt.Errorf("forecast: feature %q has no capability mapping", f)
		}
		if !caps[need] {
			return FeaturePlan{}, fmt.Errorf("forecast: feature %q requires capability %q, analysis recipe %q does not provide it", f, need, analysis.HumanKey())
		}
	}
	aID, err := analysis.Identity()
	if err != nil {
		return FeaturePlan{}, err
	}
	fID, err := features.Identity()
	if err != nil {
		return FeaturePlan{}, err
	}
	return FeaturePlan{
		Analysis:           aID,
		Features:           fID,
		Logic:              planLogic,
		Schema:             append([]FeatureID(nil), features.Features...),
		FeatureHistoryBars: features.FeatureHistoryBars(),
	}, nil
}
