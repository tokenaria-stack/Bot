package forecast

import (
	"fmt"

	"trading_bot/indicators"
)

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
	Family           TargetFamily // "" => TargetFamilyATRFirstPassage
	HorizonBars      int          // required, > 0
	UpperATRMultiple float64      // required, > 0
	LowerATRMultiple float64      // required, > 0
	ATRPeriod        int          // 0 => CanonicalATRSpec.Period
	ATRMethod        indicators.ATRMethod
	ATRLogic         string        // "" => atr:wilder-rma-first-tr-v1
	DualHit          DualHitPolicy // "" => DualHitExcludeAmbiguous
}

// TargetSpec is a published, immutable first-passage event definition.
// Barriers are frozen at candidate close t using information known at t;
// future ATR/volatility/structure may never move them (FORECAST-SPEC-1 §15).
// ATR is indicators.ATRSpec (canonical numerical law). Changing ATR does not
// invalidate FeatureTape; it invalidates LabelSet / model / calibration.
type TargetSpec struct {
	Name             string // friendly — excluded from identity
	Family           TargetFamily
	HorizonBars      int
	UpperATRMultiple float64
	LowerATRMultiple float64
	ATR              indicators.ATRSpec
	DualHit          DualHitPolicy
	Logic            LogicVersion
}

// ResolveTargetSpec applies defaults, validates, and returns the immutable
// published TargetSpec. Omitted ATR fields resolve once to CanonicalATRSpec.
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
	canon := indicators.CanonicalATRSpec()
	atr := indicators.ATRSpec{
		Period: draft.ATRPeriod,
		Method: draft.ATRMethod,
		Logic:  draft.ATRLogic,
	}
	if atr.Period <= 0 {
		atr.Period = canon.Period
	}
	if atr.Method == "" {
		atr.Method = canon.Method
	}
	if atr.Logic == "" {
		atr.Logic = canon.Logic
	}
	if err := indicators.ValidateATRSpec(atr); err != nil {
		return TargetSpec{}, err
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
		ATR:              atr,
		DualHit:          dualHit,
		Logic:            logic,
	}, nil
}

// HumanKey is a readable identity hint. Not identity.
func (t TargetSpec) HumanKey() string {
	return fmt.Sprintf("%s-h%d-u%.2f-d%.2f-atr%d", t.Family, t.HorizonBars, t.UpperATRMultiple, t.LowerATRMultiple, t.ATR.Period)
}

type targetIdentityPayload struct {
	Family           TargetFamily
	HorizonBars      int
	UpperATRMultiple float64
	LowerATRMultiple float64
	ATR              indicators.ATRSpec
	DualHit          DualHitPolicy
	Logic            LogicVersion
}

// Identity returns the resolved-config full digest identity. Name is
// excluded. ATR Period/Method/Logic are part of TargetDigest.
func (t TargetSpec) Identity() (Identity, error) {
	return NewIdentity(t.HumanKey(), targetIdentityPayload{
		Family:           t.Family,
		HorizonBars:      t.HorizonBars,
		UpperATRMultiple: t.UpperATRMultiple,
		LowerATRMultiple: t.LowerATRMultiple,
		ATR:              t.ATR,
		DualHit:          t.DualHit,
		Logic:            t.Logic,
	}, t.Logic)
}
