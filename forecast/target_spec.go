package forecast

import "fmt"

// TargetFamily names a first-passage target construction. v1 defines exactly
// one family (FORECAST-SPEC-1 §15/§24: "first model: one TargetSpec only").
type TargetFamily string

const TargetFamilyATRFirstPassage TargetFamily = "atr_first_passage"

// DualHitPolicy is the explicit law used when one target-TF candle breaches
// both barriers and OHLCV alone cannot order the touches (FORECAST-SPEC-1
// §17). The labeler must never guess.
type DualHitPolicy string

const (
	// DualHitResolveFinerHistory resolves ordering using strictly finer
	// authoritative history from the SAME MarketKey family (MarketKey.SameFamily).
	DualHitResolveFinerHistory DualHitPolicy = "resolve_finer_history"
	// DualHitExcludeAmbiguous marks the candidate TARGET_AMBIGUOUS and
	// excludes it from model fitting. Required when no reliable finer
	// history exists.
	DualHitExcludeAmbiguous DualHitPolicy = "exclude_ambiguous"
)

// TargetOutcome is the labeled event for one candidate. AMBIGUOUS is a
// dataset-quality status, never a fourth class the model predicts
// (FORECAST-SPEC-1 §15/§17).
type TargetOutcome string

const (
	OutcomeUpFirst   TargetOutcome = "UP_FIRST"
	OutcomeDownFirst TargetOutcome = "DOWN_FIRST"
	OutcomeTimeout   TargetOutcome = "TIMEOUT"
	OutcomeAmbiguous TargetOutcome = "AMBIGUOUS"
)

// TargetSpecDraft is mutable, pre-resolution input.
type TargetSpecDraft struct {
	Family           TargetFamily  // "" => TargetFamilyATRFirstPassage
	HorizonBars      int           // required, > 0
	UpperATRMultiple float64       // required, > 0
	LowerATRMultiple float64       // required, > 0
	ATRPeriod        int           // 0 => default
	DualHit          DualHitPolicy // "" => DualHitExcludeAmbiguous
}

// TargetSpec is a published, immutable first-passage event definition.
// Barriers are frozen at candidate close t using information known at t;
// future ATR/volatility/structure may never move them (FORECAST-SPEC-1 §15).
type TargetSpec struct {
	Name             string // friendly — excluded from identity
	Family           TargetFamily
	HorizonBars      int
	UpperATRMultiple float64
	LowerATRMultiple float64
	ATRPeriod        int
	DualHit          DualHitPolicy
	Logic            LogicVersion
}

const defaultATRPeriod = 14

// ResolveTargetSpec applies defaults, validates, and returns the immutable
// published TargetSpec.
func ResolveTargetSpec(name string, draft TargetSpecDraft, logic LogicVersion) (TargetSpec, error) {
	if logic == "" {
		return TargetSpec{}, fmt.Errorf("forecast: target spec requires a LogicVersion")
	}
	family := draft.Family
	if family == "" {
		family = TargetFamilyATRFirstPassage
	}
	if draft.HorizonBars <= 0 {
		return TargetSpec{}, fmt.Errorf("forecast: target spec horizon bars must be > 0")
	}
	if draft.UpperATRMultiple <= 0 || draft.LowerATRMultiple <= 0 {
		return TargetSpec{}, fmt.Errorf("forecast: target spec barrier multiples must be > 0")
	}
	atrPeriod := draft.ATRPeriod
	if atrPeriod <= 0 {
		atrPeriod = defaultATRPeriod
	}
	dualHit := draft.DualHit
	if dualHit == "" {
		dualHit = DualHitExcludeAmbiguous
	}
	return TargetSpec{
		Name:             name,
		Family:           family,
		HorizonBars:      draft.HorizonBars,
		UpperATRMultiple: draft.UpperATRMultiple,
		LowerATRMultiple: draft.LowerATRMultiple,
		ATRPeriod:        atrPeriod,
		DualHit:          dualHit,
		Logic:            logic,
	}, nil
}

// HumanKey is a readable identity hint, e.g. "atr_first_passage-h24-u1.50-d1.00".
// Not identity.
func (t TargetSpec) HumanKey() string {
	return fmt.Sprintf("%s-h%d-u%.2f-d%.2f", t.Family, t.HorizonBars, t.UpperATRMultiple, t.LowerATRMultiple)
}

type targetIdentityPayload struct {
	Family           TargetFamily
	HorizonBars      int
	UpperATRMultiple float64
	LowerATRMultiple float64
	ATRPeriod        int
	DualHit          DualHitPolicy
	Logic            LogicVersion
}

// Identity returns the resolved-config full digest identity. Name is
// excluded.
func (t TargetSpec) Identity() (Identity, error) {
	return NewIdentity(t.HumanKey(), targetIdentityPayload{
		Family:           t.Family,
		HorizonBars:      t.HorizonBars,
		UpperATRMultiple: t.UpperATRMultiple,
		LowerATRMultiple: t.LowerATRMultiple,
		ATRPeriod:        t.ATRPeriod,
		DualHit:          t.DualHit,
		Logic:            t.Logic,
	}, t.Logic)
}
