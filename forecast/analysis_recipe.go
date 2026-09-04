package forecast

import "fmt"

// AnalysisCapability names an analytical family an AnalysisRecipe may
// compute (e.g. RSX, TV divergence, Fractal facts). This is a static
// vocabulary, not a plugin registry — mirrors core/nodes.RSXWorkMask's
// bit-family shape, expressed as identity-level capability instead of a
// runtime demand mask.
type AnalysisCapability string

const (
	CapabilityRSX     AnalysisCapability = "rsx"
	CapabilityTV      AnalysisCapability = "tv"
	CapabilityFractal AnalysisCapability = "fractal"
)

// AnalysisRecipeDraft is mutable, pre-resolution input. Zero values mean
// "use the default" — see ResolveAnalysisRecipe. Field names here are
// illustrative placeholders for the SPEC-1 identity mechanism; they do not
// reopen or bind the frozen RSX/Wozduh implementations.
type AnalysisRecipeDraft struct {
	RSXLength     int    // 0 => default
	RSXSignal     int    // 0 => default
	RSXSource     string // "" => default
	DivLookback   int    // 0 => default; TV detector lookback (actual Frame setting)
	EnableTV      bool
	EnableFractal bool
}

// AnalysisRecipeConfig is the resolved (post-default, post-validate)
// numerical configuration. This — and only this, plus Logic — is what gets
// digested. A friendly Name is never part of it (FORECAST-SPEC-1 §18).
type AnalysisRecipeConfig struct {
	RSXLength     int    `json:"rsx_length"`
	RSXSignal     int    `json:"rsx_signal"`
	RSXSource     string `json:"rsx_source"`
	DivLookback   int    `json:"div_lookback"`
	EnableTV      bool   `json:"enable_tv"`
	EnableFractal bool   `json:"enable_fractal"`
}

// AnalysisRecipe is a published, immutable "how analytical truth is
// computed" configuration. It owns indicator periods/sources/detector
// lookbacks — never paint, model weights, feature selection, TargetSpec, or
// Decision/execution thresholds (FORECAST-SPEC-1 §4/§14).
type AnalysisRecipe struct {
	Name   string // friendly, renameable — excluded from identity
	Config AnalysisRecipeConfig
	Logic  LogicVersion // initialization/warmup/reconstruction semantics live here
}

const (
	defaultRSXLength   = 14
	defaultRSXSignal   = 9
	defaultRSXSource   = "hlc3"
	defaultDivLookback = 90 // matches market.RSXLookbackDefault; bind uses actual Frame settings
)

// ResolveAnalysisRecipe applies defaults, validates, and returns the
// immutable published AnalysisRecipe. Two semantically identical drafts
// (e.g. signal omitted vs. signal explicitly set to its default) resolve to
// the same Identity regardless of which fields were explicit
// (FORECAST-SPEC-1 §13).
func ResolveAnalysisRecipe(name string, draft AnalysisRecipeDraft, logic LogicVersion) (AnalysisRecipe, error) {
	if logic == "" {
		return AnalysisRecipe{}, fmt.Errorf("forecast: analysis recipe requires a LogicVersion")
	}
	cfg := AnalysisRecipeConfig{
		RSXLength:     draft.RSXLength,
		RSXSignal:     draft.RSXSignal,
		RSXSource:     draft.RSXSource,
		DivLookback:   draft.DivLookback,
		EnableTV:      draft.EnableTV,
		EnableFractal: draft.EnableFractal,
	}
	if cfg.RSXLength == 0 {
		cfg.RSXLength = defaultRSXLength
	}
	if cfg.RSXSignal == 0 {
		cfg.RSXSignal = defaultRSXSignal
	}
	if cfg.RSXSource == "" {
		cfg.RSXSource = defaultRSXSource
	}
	if cfg.DivLookback == 0 {
		cfg.DivLookback = defaultDivLookback
	}
	if cfg.RSXLength < 0 || cfg.RSXSignal < 0 {
		return AnalysisRecipe{}, fmt.Errorf("forecast: analysis recipe periods must be >= 0")
	}
	return AnalysisRecipe{Name: name, Config: cfg, Logic: logic}, nil
}

// HumanKey is a readable identity hint, e.g. "rsx-l14-s9-hlc3". Not identity.
func (r AnalysisRecipe) HumanKey() string {
	return fmt.Sprintf("rsx-l%d-s%d-%s-tv%d", r.Config.RSXLength, r.Config.RSXSignal, r.Config.RSXSource, r.Config.DivLookback)
}

type analysisIdentityPayload struct {
	Config AnalysisRecipeConfig
	Logic  LogicVersion
}

// Identity returns the resolved-config full digest identity. Name is
// excluded; renaming an AnalysisRecipe never changes its Identity.
func (r AnalysisRecipe) Identity() (Identity, error) {
	return NewIdentity(r.HumanKey(), analysisIdentityPayload{Config: r.Config, Logic: r.Logic}, r.Logic)
}

// Capabilities reports which analytical families this resolved recipe
// computes. FeaturePlan binding uses this to REFUSE (never zero-fill) a
// FeatureRecipe that requires an unavailable capability.
func (r AnalysisRecipe) Capabilities() map[AnalysisCapability]bool {
	caps := map[AnalysisCapability]bool{CapabilityRSX: true}
	if r.Config.EnableTV {
		caps[CapabilityTV] = true
	}
	if r.Config.EnableFractal {
		caps[CapabilityFractal] = true
	}
	return caps
}
