package forecast

import "fmt"

// FeatureID names one deterministic measurement a FeatureRecipe may request.
// FeatureIDs measure — they never award points (FORECAST-SPEC-1 §5).
type FeatureID string

const (
	FeatureRSXValue      FeatureID = "rsx_value"
	FeatureRSXSignal     FeatureID = "rsx_signal"
	FeatureRSXSlope      FeatureID = "rsx_slope"
	FeatureTVBullPresent FeatureID = "tv_bull_present"
	FeatureTVBullAge     FeatureID = "tv_bull_age"
	FeatureFractalClassA FeatureID = "fractal_class_a"
)

// featureCapability is the static, registry-free bind-time requirement table
// (same shape as core/nodes.RSXWorkFromFactSources): which analytical
// capability a feature needs from the bound AnalysisRecipe.
var featureCapability = map[FeatureID]AnalysisCapability{
	FeatureRSXValue:      CapabilityRSX,
	FeatureRSXSignal:     CapabilityRSX,
	FeatureRSXSlope:      CapabilityRSX,
	FeatureTVBullPresent: CapabilityTV,
	FeatureTVBullAge:     CapabilityTV,
	FeatureFractalClassA: CapabilityFractal,
}

// ageFeatures lists features whose temporal meaning is "bars since a fact
// was confirmed" and therefore need a bounded FeatureHistoryBars contract
// (FORECAST-SPEC-1 §10/§11): age = min(actualAge, MaxAgeBars[feature]).
var ageFeatures = map[FeatureID]bool{
	FeatureTVBullAge: true,
}

const defaultMaxAgeBars = 256

// FeatureRecipeDraft is mutable, pre-resolution input.
type FeatureRecipeDraft struct {
	// Features is the ORDERED list of measurements to extract. Order is part
	// of the schema (FeaturePlan vector layout) and is preserved as given —
	// it is never silently re-sorted.
	Features []FeatureID
	// MaxAgeBars optionally bounds age features. Missing/zero entries for an
	// age feature resolve to defaultMaxAgeBars (FORECAST-SPEC-1 §11).
	MaxAgeBars map[FeatureID]int
}

// FeatureRecipe is a published, immutable "which measurements are
// extracted" configuration. Reusable across AnalysisRecipes — NOT
// permanently pinned to one (FORECAST-SPEC-1 §5).
type FeatureRecipe struct {
	Name       string // friendly — excluded from identity
	Features   []FeatureID
	MaxAgeBars map[FeatureID]int
	Logic      LogicVersion
}

// ResolveFeatureRecipe applies defaults, validates feature IDs and
// uniqueness, and returns the immutable published FeatureRecipe.
func ResolveFeatureRecipe(name string, draft FeatureRecipeDraft, logic LogicVersion) (FeatureRecipe, error) {
	if logic == "" {
		return FeatureRecipe{}, fmt.Errorf("forecast: feature recipe requires a LogicVersion")
	}
	if len(draft.Features) == 0 {
		return FeatureRecipe{}, fmt.Errorf("forecast: feature recipe requires at least one feature")
	}
	seen := make(map[FeatureID]bool, len(draft.Features))
	features := make([]FeatureID, 0, len(draft.Features))
	for _, f := range draft.Features {
		if _, ok := featureCapability[f]; !ok {
			return FeatureRecipe{}, fmt.Errorf("forecast: unknown feature id %q", f)
		}
		if seen[f] {
			return FeatureRecipe{}, fmt.Errorf("forecast: duplicate feature id %q", f)
		}
		seen[f] = true
		features = append(features, f)
	}
	maxAge := make(map[FeatureID]int)
	for _, f := range features {
		if !ageFeatures[f] {
			continue
		}
		n := draft.MaxAgeBars[f]
		if n <= 0 {
			n = defaultMaxAgeBars
		}
		maxAge[f] = n
	}
	return FeatureRecipe{Name: name, Features: features, MaxAgeBars: maxAge, Logic: logic}, nil
}

// HumanKey is a readable identity hint. Not identity.
func (r FeatureRecipe) HumanKey() string {
	return fmt.Sprintf("features-%d", len(r.Features))
}

type featureIdentityPayload struct {
	Features   []FeatureID
	MaxAgeBars map[FeatureID]int
	Logic      LogicVersion
}

// Identity returns the resolved-config full digest identity. Name is
// excluded.
func (r FeatureRecipe) Identity() (Identity, error) {
	return NewIdentity(r.HumanKey(), featureIdentityPayload{Features: r.Features, MaxAgeBars: r.MaxAgeBars, Logic: r.Logic}, r.Logic)
}

// FeatureHistoryBars is the maximum bounded age across this recipe's age
// features — the FEATURE-SIDE history contribution only. It does NOT
// include AnalysisRecipe/Jurik/detector reconstruction requirements (that
// contribution does not exist as a type in this chapter). The later
// complete closure is reserved under the name RequiredHistoryBars:
//
//	RequiredHistoryBars = max(AnalysisRuntime reconstruction requirement,
//	                          FeatureHistoryBars, ...)
//
// implemented in FEATURE-TAPE-1 once a real AnalysisRuntime binding exists
// (FORECAST-SPEC-1 §11).
func (r FeatureRecipe) FeatureHistoryBars() int {
	max := 0
	for _, n := range r.MaxAgeBars {
		if n > max {
			max = n
		}
	}
	return max
}
