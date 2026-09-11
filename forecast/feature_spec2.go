package forecast

import (
	"fmt"

	"trading_bot/data"
)

func errSpec2(msg string) error { return fmt.Errorf("forecast: feature-spec-2: %s", msg) }

// DirectionalityKind is FEATURE-SPEC-2 symmetry metadata (one table, no extra digest family).
type DirectionalityKind string

const (
	DirMirroredPair     DirectionalityKind = "MIRRORED_PAIR"
	DirSignedBipolar    DirectionalityKind = "SIGNED_BIPOLAR"
	DirNeutralMagnitude DirectionalityKind = "NEUTRAL_MAGNITUDE"
)

// HistoryDemand is native-bar demand per MarketKey. IIR (RSX/ATR/TV) uses the
// certified source prefix from research start — sliding windows cannot equal
// infinite IIR history (FEATURE-TAPE-2 owns that prefix-parity).
type HistoryDemand struct {
	Primary15mWindowBars int
	HTF1hWindowBars      int
	HTF4hWindowBars      int
	IIRFromSourceStart   bool
}

// FeatureSpec2 is the CatBoost V1 sensory contract. No Fill, no Outcome, no model.
type FeatureSpec2 struct {
	Target   TargetSpec
	Analysis AnalysisRecipe
	Features FeatureRecipe
	Plan     FeaturePlan
	Primary  MarketKey
	HTF1h    MarketKey
	HTF4h    MarketKey
	Q        int
	Demand   HistoryDemand
}

type featureSpec2IdentityPayload struct {
	TargetDigest                Digest
	Analysis                    Digest
	Features                    Digest
	Plan                        Digest
	Primary                     MarketKey
	HTF1h                       MarketKey
	HTF4h                       MarketKey
	H, Q                        int
	U, L                        float64
	Width                       int
	IDs                         []FeatureID
	MaxAge                      map[FeatureID]int
	MaxGap, MaxSpan, PatternAge int
	SameBarIsConjunction        bool
	JoinLaw                     string
	Demand                      HistoryDemand
	Logic                       LogicVersion
}

func init() {
	for id, cap := range spec2Capability() {
		featureCapability[id] = cap
	}
	for id := range spec2AgeCaps() {
		ageFeatures[id] = true
	}
}

func spec2Capability() map[FeatureID]AnalysisCapability {
	m := map[FeatureID]AnalysisCapability{}
	rsx := []FeatureID{
		FeatureRSXMinusSignal, FeatureRSXDelta1, FeatureRSXDeltaQ,
		FeatureRSXSignalCrossUpPresent, FeatureRSXSignalCrossUpAge, FeatureRSXSignalCrossDownPresent, FeatureRSXSignalCrossDownAge,
		FeatureRSX50CrossUpPresent, FeatureRSX50CrossUpAge, FeatureRSX50CrossDownPresent, FeatureRSX50CrossDownAge,
		FeatureHTF1hRSXValue, FeatureHTF1hRSXMinusSignal, FeatureHTF1hRSXDelta1,
		FeatureHTF1hSignalCrossUpP, FeatureHTF1hSignalCrossUpAge, FeatureHTF1hSignalCrossDnP, FeatureHTF1hSignalCrossDnAge,
		FeatureHTF4hRSXValue, FeatureHTF4hRSXMinusSignal, FeatureHTF4hRSXDelta1,
		FeatureHTF4hSignalCrossUpP, FeatureHTF4hSignalCrossUpAge, FeatureHTF4hSignalCrossDnP, FeatureHTF4hSignalCrossDnAge,
	}
	for _, id := range rsx {
		m[id] = CapabilityRSX
	}
	tv := []FeatureID{
		FeatureTVBearPresent, FeatureTVBearAge,
		FeatureTVPivotHighPresent, FeatureTVPivotHighAge, FeatureTVPivotLowPresent, FeatureTVPivotLowAge,
		FeaturePatTVToCrossBullPresent, FeaturePatTVToCrossBullAge, FeaturePatTVToCrossBearPresent, FeaturePatTVToCrossBearAge,
		FeaturePatCrossToTVBullPresent, FeaturePatCrossToTVBullAge, FeaturePatCrossToTVBearPresent, FeaturePatCrossToTVBearAge,
		FeaturePatPivotToCrossBullPresent, FeaturePatPivotToCrossBullAge, FeaturePatPivotToCrossBearPresent, FeaturePatPivotToCrossBearAge,
		FeaturePatSamebarTVCrossBullPresent, FeaturePatSamebarTVCrossBullAge, FeaturePatSamebarTVCrossBearPresent, FeaturePatSamebarTVCrossBearAge,
		FeatureHTF1hTVBullPresent, FeatureHTF1hTVBullAge, FeatureHTF1hTVBearPresent, FeatureHTF1hTVBearAge,
		FeatureHTF4hTVBullPresent, FeatureHTF4hTVBullAge, FeatureHTF4hTVBearPresent, FeatureHTF4hTVBearAge,
	}
	for _, id := range tv {
		m[id] = CapabilityTV
	}
	ohlc := []FeatureID{
		FeaturePriceDisplacementQATR, FeaturePriceDisplacementHATR, FeaturePriceRangePositionH,
		FeaturePricePathEfficiencyH, FeatureATROverPrice, FeatureATRChangeQ,
	}
	for _, id := range ohlc {
		m[id] = CapabilityOHLC
	}
	return m
}

func spec2AgeCaps() map[FeatureID]int {
	a36 := []FeatureID{
		FeatureTVBullAge, FeatureTVBearAge, FeatureTVPivotHighAge, FeatureTVPivotLowAge,
		FeaturePatTVToCrossBullAge, FeaturePatTVToCrossBearAge,
		FeaturePatCrossToTVBullAge, FeaturePatCrossToTVBearAge,
		FeaturePatPivotToCrossBullAge, FeaturePatPivotToCrossBearAge,
		FeaturePatSamebarTVCrossBullAge, FeaturePatSamebarTVCrossBearAge,
	}
	a12 := []FeatureID{
		FeatureRSXSignalCrossUpAge, FeatureRSXSignalCrossDownAge,
		FeatureRSX50CrossUpAge, FeatureRSX50CrossDownAge,
		FeatureHTF1hTVBullAge, FeatureHTF1hTVBearAge, FeatureHTF1hSignalCrossUpAge, FeatureHTF1hSignalCrossDnAge,
		FeatureHTF4hTVBullAge, FeatureHTF4hTVBearAge, FeatureHTF4hSignalCrossUpAge, FeatureHTF4hSignalCrossDnAge,
	}
	m := map[FeatureID]int{}
	for _, id := range a36 {
		m[id] = Spec2TVPivotAgePrimary
	}
	for _, id := range a12 {
		m[id] = Spec2SimpleCrossAgePrim
	}
	// HTF ages are 12 native bars (same integer; unit is the HTF).
	for _, id := range []FeatureID{
		FeatureHTF1hTVBullAge, FeatureHTF1hTVBearAge, FeatureHTF1hSignalCrossUpAge, FeatureHTF1hSignalCrossDnAge,
		FeatureHTF4hTVBullAge, FeatureHTF4hTVBearAge, FeatureHTF4hSignalCrossUpAge, FeatureHTF4hSignalCrossDnAge,
	} {
		m[id] = Spec2HTFFactAgeNative
	}
	return m
}

// FeatureSpec2Mirrors is the complete MIRRORED_PAIR table.
func FeatureSpec2Mirrors() map[FeatureID]FeatureID {
	pairs := [][2]FeatureID{
		{FeatureTVBullPresent, FeatureTVBearPresent},
		{FeatureTVBullAge, FeatureTVBearAge},
		{FeatureTVPivotHighPresent, FeatureTVPivotLowPresent},
		{FeatureTVPivotHighAge, FeatureTVPivotLowAge},
		{FeatureRSXSignalCrossUpPresent, FeatureRSXSignalCrossDownPresent},
		{FeatureRSXSignalCrossUpAge, FeatureRSXSignalCrossDownAge},
		{FeatureRSX50CrossUpPresent, FeatureRSX50CrossDownPresent},
		{FeatureRSX50CrossUpAge, FeatureRSX50CrossDownAge},
		{FeaturePatTVToCrossBullPresent, FeaturePatTVToCrossBearPresent},
		{FeaturePatTVToCrossBullAge, FeaturePatTVToCrossBearAge},
		{FeaturePatCrossToTVBullPresent, FeaturePatCrossToTVBearPresent},
		{FeaturePatCrossToTVBullAge, FeaturePatCrossToTVBearAge},
		{FeaturePatPivotToCrossBullPresent, FeaturePatPivotToCrossBearPresent},
		{FeaturePatPivotToCrossBullAge, FeaturePatPivotToCrossBearAge},
		{FeaturePatSamebarTVCrossBullPresent, FeaturePatSamebarTVCrossBearPresent},
		{FeaturePatSamebarTVCrossBullAge, FeaturePatSamebarTVCrossBearAge},
		{FeatureHTF1hTVBullPresent, FeatureHTF1hTVBearPresent},
		{FeatureHTF1hTVBullAge, FeatureHTF1hTVBearAge},
		{FeatureHTF1hSignalCrossUpP, FeatureHTF1hSignalCrossDnP},
		{FeatureHTF1hSignalCrossUpAge, FeatureHTF1hSignalCrossDnAge},
		{FeatureHTF4hTVBullPresent, FeatureHTF4hTVBearPresent},
		{FeatureHTF4hTVBullAge, FeatureHTF4hTVBearAge},
		{FeatureHTF4hSignalCrossUpP, FeatureHTF4hSignalCrossDnP},
		{FeatureHTF4hSignalCrossUpAge, FeatureHTF4hSignalCrossDnAge},
	}
	m := map[FeatureID]FeatureID{}
	for _, p := range pairs {
		m[p[0]], m[p[1]] = p[1], p[0]
	}
	return m
}

func FeatureSpec2Directionality(id FeatureID) DirectionalityKind {
	if _, ok := FeatureSpec2Mirrors()[id]; ok {
		return DirMirroredPair
	}
	switch id {
	case FeatureRSXMinusSignal, FeatureRSXDelta1, FeatureRSXDeltaQ,
		FeaturePriceDisplacementQATR, FeaturePriceDisplacementHATR, FeatureATRChangeQ,
		FeatureHTF1hRSXMinusSignal, FeatureHTF1hRSXDelta1, FeatureHTF4hRSXMinusSignal, FeatureHTF4hRSXDelta1:
		return DirSignedBipolar
	default:
		return DirNeutralMagnitude
	}
}

func ValidateFeatureSpec2Symmetry(ids []FeatureID) error {
	return ValidateFeatureSpec2SymmetryWith(ids, FeatureSpec2Mirrors())
}

func ValidateFeatureSpec2SymmetryWith(ids []FeatureID, mirrors map[FeatureID]FeatureID) error {
	have := map[FeatureID]bool{}
	for _, id := range ids {
		have[id] = true
	}
	for id, other := range mirrors {
		if id == other || other == "" {
			return errSpec2("invalid mirror mapping for " + string(id))
		}
		if mirrors[other] != id {
			return errSpec2("asymmetric mirror map")
		}
		if !have[id] || !have[other] {
			return errSpec2("missing mirror for " + string(id))
		}
	}
	for _, id := range ids {
		if FeatureSpec2Directionality(id) != DirMirroredPair {
			continue
		}
		other, ok := mirrors[id]
		if !ok || !have[other] {
			return errSpec2("missing mirror for " + string(id))
		}
	}
	return nil
}

func DeriveSpec2HistoryDemand(divLookback, h, q, atrPeriod int) HistoryDemand {
	prim := divLookback
	for _, n := range []int{h + 1, q + 1, Spec2TVPivotAgePrimary + 1, Spec2SimpleCrossAgePrim + 1,
		Spec2MaxGapBars + Spec2PatternActiveAge, atrPeriod + 1} {
		if n > prim {
			prim = n
		}
	}
	htf := divLookback
	for _, n := range []int{2, Spec2HTFFactAgeNative + 1, atrPeriod + 1} {
		if n > htf {
			htf = n
		}
	}
	return HistoryDemand{
		Primary15mWindowBars: prim,
		HTF1hWindowBars:      htf,
		HTF4hWindowBars:      htf,
		IIRFromSourceStart:   true,
	}
}

// KnowledgeCloseTime is canonical CloseTime of a bar open (nextOpen-1).
func KnowledgeCloseTime(openMs int64, tf string) (int64, error) {
	return data.BarCloseTimeMs(openMs, tf)
}

// SelectLatestClosedHTF returns the index of the latest HTF open whose CloseTime
// is <= primaryKnowledgeTime. It does not skip NotReady rows (caller applies Ready).
func SelectLatestClosedHTF(htfOpens []int64, htfTF string, primaryKnowledgeTime int64) (int, error) {
	best := -1
	for i, open := range htfOpens {
		ct, err := KnowledgeCloseTime(open, htfTF)
		if err != nil {
			return -1, err
		}
		if ct <= primaryKnowledgeTime {
			best = i
		}
	}
	if best < 0 {
		return -1, errSpec2("no closed HTF row at knowledge time")
	}
	return best, nil
}

// JoinHTFReady: latest closed NotReady ⇒ primary NotReady. No stale fallback.
func JoinHTFReady(latestClosed Ready) Ready {
	if latestClosed != IsReady {
		return NotReady
	}
	return IsReady
}

func HTFKeys(primary MarketKey) (h1, h4 MarketKey, err error) {
	if err = primary.Validate(); err != nil {
		return MarketKey{}, MarketKey{}, err
	}
	h1 = MarketKey{Venue: primary.Venue, Instrument: primary.Instrument, Contract: primary.Contract, Timeframe: Spec2HTF1h}
	h4 = MarketKey{Venue: primary.Venue, Instrument: primary.Instrument, Contract: primary.Contract, Timeframe: Spec2HTF4h}
	if !primary.SameFamily(h1) || !primary.SameFamily(h4) {
		return MarketKey{}, MarketKey{}, errSpec2("HTF not same family")
	}
	return h1, h4, nil
}

// ResolveFeatureSpec2 binds Target C + analysis:v2 signal-14 + 64-column features:v2.
func ResolveFeatureSpec2(primary MarketKey, target TargetSpec, analysis AnalysisRecipe) (FeatureSpec2, error) {
	if err := primary.Validate(); err != nil {
		return FeatureSpec2{}, err
	}
	if primary.Timeframe != Spec2PrimaryTF {
		return FeatureSpec2{}, errSpec2("primary timeframe must be 15m")
	}
	if target.HorizonBars != Spec2TargetHorizon || target.UpperATRMultiple != Spec2UpperATR || target.LowerATRMultiple != Spec2LowerATR {
		return FeatureSpec2{}, errSpec2("wrong TargetSpec (require H=72 U=L=2.0)")
	}
	if target.Family != TargetFamilyATRFirstPassage {
		return FeatureSpec2{}, errSpec2("target family")
	}
	q, err := HorizonQuarterQ(target.HorizonBars)
	if err != nil {
		return FeatureSpec2{}, err
	}
	if analysis.Logic != "analysis:v2" {
		return FeatureSpec2{}, errSpec2("AnalysisLogicVersion")
	}
	if analysis.Config.RSXLength != Spec2RSXLength || analysis.Config.RSXSignal != Spec2RSXSignal ||
		analysis.Config.RSXSource != Spec2RSXSource || analysis.Config.DivLookback != Spec2TVLookback {
		return FeatureSpec2{}, errSpec2("analysis RSX/TV settings")
	}
	if !analysis.Config.EnableTV || analysis.Config.EnableFractal {
		return FeatureSpec2{}, errSpec2("analysis capabilities (TV on, fractal off)")
	}
	ids := FeatureSpec2IDs()
	if len(ids) != Spec2FeatureWidth {
		return FeatureSpec2{}, errSpec2("width")
	}
	if err := ValidateFeatureSpec2Symmetry(ids); err != nil {
		return FeatureSpec2{}, err
	}
	ages := spec2AgeCaps()
	features, err := ResolveFeatureRecipe("catboost-v1-64", FeatureRecipeDraft{Features: ids, MaxAgeBars: ages}, FeaturesLogicV2)
	if err != nil {
		return FeatureSpec2{}, err
	}
	for id, want := range ages {
		if features.MaxAgeBars[id] != want {
			return FeatureSpec2{}, errSpec2("age cap")
		}
	}
	if features.MaxAgeBars[FeatureTVBullAge] == defaultMaxAgeBars {
		return FeatureSpec2{}, errSpec2("must not default TV age to 256")
	}
	plan, err := BindFeaturePlan(analysis, features, PlanLogicV2)
	if err != nil {
		return FeatureSpec2{}, err
	}
	if plan.VectorLen() != Spec2FeatureWidth {
		return FeatureSpec2{}, errSpec2("plan width")
	}
	h1, h4, err := HTFKeys(primary)
	if err != nil {
		return FeatureSpec2{}, err
	}
	demand := DeriveSpec2HistoryDemand(analysis.Config.DivLookback, target.HorizonBars, q, target.ATR.Period)
	return FeatureSpec2{
		Target: target, Analysis: analysis, Features: features, Plan: plan,
		Primary: primary, HTF1h: h1, HTF4h: h4, Q: q, Demand: demand,
	}, nil
}

func (s FeatureSpec2) Identity() (Identity, error) {
	tid, err := s.Target.Identity()
	if err != nil {
		return Identity{}, err
	}
	aid, err := s.Analysis.Identity()
	if err != nil {
		return Identity{}, err
	}
	fid, err := s.Features.Identity()
	if err != nil {
		return Identity{}, err
	}
	pid, err := s.Plan.Identity()
	if err != nil {
		return Identity{}, err
	}
	return NewIdentity("feature-spec-2", featureSpec2IdentityPayload{
		TargetDigest: tid.Digest, Analysis: aid.Digest, Features: fid.Digest, Plan: pid.Digest,
		Primary: s.Primary, HTF1h: s.HTF1h, HTF4h: s.HTF4h,
		H: s.Target.HorizonBars, Q: s.Q, U: s.Target.UpperATRMultiple, L: s.Target.LowerATRMultiple,
		Width: Spec2FeatureWidth, IDs: FeatureSpec2IDs(), MaxAge: spec2AgeCaps(),
		MaxGap: Spec2MaxGapBars, MaxSpan: Spec2MaxSpanBars, PatternAge: Spec2PatternActiveAge,
		SameBarIsConjunction: true,
		JoinLaw:              "latest_htf_CloseTime<=primary_CloseTime; NotReady_latest=>NotReady_primary; no_stale_fallback",
		Demand:               s.Demand, Logic: FeaturesLogicV2,
	}, FeaturesLogicV2, PlanLogicV2)
}
