package forecast

import (
	"math"
	"testing"

	"trading_bot/indicators"
)

const analysisV1 LogicVersion = "analysis:v1"
const analysisV2 LogicVersion = "analysis:v2"
const featuresV1 LogicVersion = "features:v1"
const planV1 LogicVersion = "plan:v1"

// A. MarketKey distinguishes spot/perp/venue/TF.
func TestMarketKey_DistinguishesVenueContractTF(t *testing.T) {
	perp := MarketKey{Venue: "BINANCE", Instrument: "BTCUSDT", Contract: "FUTURES_PERP", Timeframe: "15m"}
	spot := MarketKey{Venue: "BINANCE", Instrument: "BTCUSDT", Contract: "SPOT", Timeframe: "15m"}
	other := MarketKey{Venue: "OTHER", Instrument: "BTCUSDT", Contract: "FUTURES_PERP", Timeframe: "15m"}
	finer := MarketKey{Venue: "BINANCE", Instrument: "BTCUSDT", Contract: "FUTURES_PERP", Timeframe: "1s"}

	if perp == spot {
		t.Fatal("perp and spot must not be equal MarketKeys")
	}
	if perp == other {
		t.Fatal("different venue must not be equal MarketKeys")
	}
	if !perp.SameFamily(finer) {
		t.Fatal("same venue/instrument/contract, different TF, must be SameFamily")
	}
	if perp.SameFamily(spot) {
		t.Fatal("perp and spot must NOT be SameFamily (dual-hit resolution law)")
	}
	if err := (MarketKey{}).Validate(); err == nil {
		t.Fatal("empty MarketKey must fail Validate")
	}
	if err := perp.Validate(); err != nil {
		t.Fatalf("fully populated MarketKey must validate: %v", err)
	}
}

// B. Resolved semantically-identical config (omitted default vs explicit
// default) must produce the same digest.
func TestAnalysisRecipe_OmittedDefaultEqualsExplicitDefault(t *testing.T) {
	implicit, err := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 14}, analysisV1)
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 14, RSXSignal: defaultRSXSignal, RSXSource: defaultRSXSource}, analysisV1)
	if err != nil {
		t.Fatal(err)
	}
	idA, err := implicit.Identity()
	if err != nil {
		t.Fatal(err)
	}
	idB, err := explicit.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if idA.Digest != idB.Digest {
		t.Fatalf("omitted default and explicit default must resolve to the same digest: %s vs %s", idA.Digest, idB.Digest)
	}
}

// C. Friendly-name rename must not change machine identity.
func TestAnalysisRecipe_RenameDoesNotChangeIdentity(t *testing.T) {
	r1, _ := ResolveAnalysisRecipe("RSX Scalping", AnalysisRecipeDraft{RSXLength: 14}, analysisV1)
	r2, _ := ResolveAnalysisRecipe("RSX Renamed Later", AnalysisRecipeDraft{RSXLength: 14}, analysisV1)
	id1, _ := r1.Identity()
	id2, _ := r2.Identity()
	if id1.Digest != id2.Digest {
		t.Fatal("renaming a recipe must not change its identity digest")
	}
}

// D. A real semantic config change must produce a different digest.
func TestAnalysisRecipe_RealChangeProducesDifferentDigest(t *testing.T) {
	r14, _ := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 14}, analysisV1)
	r21, _ := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 21}, analysisV1)
	id14, _ := r14.Identity()
	id21, _ := r21.Identity()
	if id14.Digest == id21.Digest {
		t.Fatal("RSX length 14 vs 21 must produce different identities")
	}
}

func TestAnalysisRecipe_DivLookbackChangesIdentity(t *testing.T) {
	a, _ := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 14, DivLookback: 30, EnableTV: true}, analysisV1)
	b, _ := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 14, DivLookback: 90, EnableTV: true}, analysisV1)
	ida, _ := a.Identity()
	idb, _ := b.Identity()
	if ida.Digest == idb.Digest {
		t.Fatal("DivLookback must change identity")
	}
}

// E. A logic-version bump must change identity even with identical
// resolved parameters (stale-cache-after-correctness-fix law).
func TestAnalysisRecipe_LogicVersionBumpChangesIdentity(t *testing.T) {
	v1, _ := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 14}, analysisV1)
	v2, _ := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 14}, analysisV2)
	id1, _ := v1.Identity()
	id2, _ := v2.Identity()
	if id1.Digest == id2.Digest {
		t.Fatal("bumping AnalysisLogicVersion must change identity even with identical parameters")
	}
}

// F. Full digest is retained and compared; the display abbreviation is
// never treated as identity.
func TestDigest_FullDigestIsAuthoritativeNotShort(t *testing.T) {
	r14, _ := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 14}, analysisV1)
	r21, _ := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 21}, analysisV1)
	id14, _ := r14.Identity()
	id21, _ := r21.Identity()

	if len(id14.Digest.String()) != 64 {
		t.Fatalf("full digest must be 64 hex chars (SHA-256), got %d", len(id14.Digest.String()))
	}
	if len(id14.Digest.Short()) != 16 {
		t.Fatalf("short digest must be 16 hex chars, got %d", len(id14.Digest.Short()))
	}
	if id14.Digest.Short() != id14.Digest.String()[:16] {
		t.Fatal("Short() must be a strict prefix of the full digest")
	}
	// The real identity comparison must use the full digest, not Short().
	if id14.Digest == id21.Digest {
		t.Fatal("different configs must not share a full digest")
	}
}

// G. FeatureRecipe + compatible AnalysisRecipe → FeaturePlan bind succeeds.
func TestBindFeaturePlan_CompatibleSucceeds(t *testing.T) {
	analysis, err := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 14, EnableTV: true}, analysisV1)
	if err != nil {
		t.Fatal(err)
	}
	features, err := ResolveFeatureRecipe("f", FeatureRecipeDraft{
		Features: []FeatureID{FeatureRSXValue, FeatureRSXSignal, FeatureTVBullPresent, FeatureTVBullAge},
	}, featuresV1)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BindFeaturePlan(analysis, features, planV1)
	if err != nil {
		t.Fatalf("expected compatible bind to succeed: %v", err)
	}
	if plan.VectorLen() != 4 {
		t.Fatalf("expected VectorLen 4, got %d", plan.VectorLen())
	}
	if plan.FeatureHistoryBars != defaultMaxAgeBars {
		t.Fatalf("expected FeatureHistoryBars %d, got %d", defaultMaxAgeBars, plan.FeatureHistoryBars)
	}
}

// H. FeatureRecipe requiring an unavailable analytical capability must fail
// bind — never zero-fill / never produce a partial plan.
func TestBindFeaturePlan_IncompatibleFailsClosed(t *testing.T) {
	analysis, err := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 14}, analysisV1) // no EnableFractal
	if err != nil {
		t.Fatal(err)
	}
	features, err := ResolveFeatureRecipe("f", FeatureRecipeDraft{
		Features: []FeatureID{FeatureRSXValue, FeatureFractalClassA},
	}, featuresV1)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BindFeaturePlan(analysis, features, planV1)
	if err == nil {
		t.Fatal("expected bind failure when AnalysisRecipe lacks Fractal capability")
	}
	if plan.VectorLen() != 0 || len(plan.Schema) != 0 {
		t.Fatalf("failed bind must not return a partial/zero-filled plan, got %+v", plan)
	}
}

// I. FeaturePlan.VectorLen is fixed by the compiled schema.
func TestFeaturePlan_FixedVectorLen(t *testing.T) {
	analysis, _ := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{}, analysisV1)
	features, err := ResolveFeatureRecipe("f", FeatureRecipeDraft{
		Features: []FeatureID{FeatureRSXValue, FeatureRSXSignal, FeatureRSXSlope},
	}, featuresV1)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BindFeaturePlan(analysis, features, planV1)
	if err != nil {
		t.Fatal(err)
	}
	if plan.VectorLen() != 3 {
		t.Fatalf("expected fixed VectorLen 3, got %d", plan.VectorLen())
	}
	if plan.VectorLen() != len(plan.Schema) {
		t.Fatal("VectorLen must always equal len(Schema)")
	}
}

// J. A NotReady fill must never produce a ForecastFrame (stale buffer /
// fail-closed law).
func TestPublishForecastFrame_NotReadyProducesNoFrame(t *testing.T) {
	f := ForecastFrame{At: 1000, PUp: 0.5, PDown: 0.3, PTimeout: 0.2, LongRank: 0.9, ShortRank: 0.1}
	_, err := PublishForecastFrame(NotReady, f)
	if err == nil {
		t.Fatal("expected error: NotReady must never publish a ForecastFrame")
	}
	// A Ready, valid frame must publish successfully.
	got, err := PublishForecastFrame(IsReady, f)
	if err != nil {
		t.Fatalf("expected Ready+valid frame to publish: %v", err)
	}
	if got != f {
		t.Fatal("published frame must equal the input frame unchanged")
	}
}

// K. Nonfinite feature/probability values must be rejected.
func TestValidate_RejectsNonfiniteValues(t *testing.T) {
	if err := ValidateFeatureVector([]float64{1, 2, math.NaN()}); err == nil {
		t.Fatal("expected NaN in feature vector to be rejected")
	}
	if err := ValidateFeatureVector([]float64{1, math.Inf(1)}); err == nil {
		t.Fatal("expected +Inf in feature vector to be rejected")
	}
	if err := ValidateFeatureVector([]float64{1, math.Inf(-1)}); err == nil {
		t.Fatal("expected -Inf in feature vector to be rejected")
	}
	if err := ValidateFeatureVector([]float64{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("finite vector must be accepted: %v", err)
	}

	bad := ForecastFrame{PUp: math.NaN(), PDown: 0.3, PTimeout: 0.2}
	if err := ValidateForecastFrame(bad); err == nil {
		t.Fatal("expected NaN probability to be rejected")
	}
	outOfRange := ForecastFrame{PUp: 1.5, PDown: -0.5, PTimeout: 0.0}
	if err := ValidateForecastFrame(outOfRange); err == nil {
		t.Fatal("expected out-of-[0,1] probability to be rejected")
	}
	badSum := ForecastFrame{PUp: 0.5, PDown: 0.5, PTimeout: 0.5}
	if err := ValidateForecastFrame(badSum); err == nil {
		t.Fatal("expected probabilities summing far from 1 to be rejected")
	}
	good := ForecastFrame{PUp: 0.5, PDown: 0.3, PTimeout: 0.2, LongRank: 0.9, ShortRank: 0.1}
	if err := ValidateForecastFrame(good); err != nil {
		t.Fatalf("valid finite/sane frame must be accepted: %v", err)
	}
}

// TargetSpec: AMBIGUOUS is a dataset status, not a model-fitted class, and
// dual-hit exclusion is the default when unspecified.
func TestTargetSpec_ResolveDefaultsAndOutcomes(t *testing.T) {
	ts, err := ResolveTargetSpec("medium", TargetSpecDraft{
		HorizonBars: 24, UpperATRMultiple: 1.5, LowerATRMultiple: 1.0,
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	if ts.Family != TargetFamilyATRFirstPassage {
		t.Fatalf("expected default family, got %s", ts.Family)
	}
	if ts.DualHit != DualHitExcludeAmbiguous {
		t.Fatalf("expected default dual-hit policy to be exclude-ambiguous, got %s", ts.DualHit)
	}
	if ts.ATR.Period != indicators.DefaultATRPeriod {
		t.Fatalf("expected default ATR period, got %d", ts.ATR.Period)
	}
	if ts.ATR.Method != indicators.ATRMethodWilderRMA || ts.ATR.Logic != indicators.ATRLogicWilderRMAFirstTRV1 {
		t.Fatalf("expected canonical ATRSpec, got %+v", ts.ATR)
	}
	if OutcomeAmbiguous == OutcomeUpFirst || OutcomeAmbiguous == OutcomeDownFirst || OutcomeAmbiguous == OutcomeTimeout {
		t.Fatal("AMBIGUOUS must remain distinct from all three model outcomes")
	}
	if _, err := ResolveTargetSpec("bad", TargetSpecDraft{HorizonBars: 0, UpperATRMultiple: 1, LowerATRMultiple: 1}, "labels:v1"); err == nil {
		t.Fatal("expected horizon <= 0 to be rejected")
	}
}

// ForecastArtifactPinned: identical pins produce identical digests; a
// different MarketKey (e.g. spot vs perp) must refuse to match.
func TestForecastArtifactPinned_MarketMismatchChangesDigest(t *testing.T) {
	analysis, _ := ResolveAnalysisRecipe("a", AnalysisRecipeDraft{RSXLength: 14}, analysisV1)
	features, _ := ResolveFeatureRecipe("f", FeatureRecipeDraft{Features: []FeatureID{FeatureRSXValue}}, featuresV1)
	plan, err := BindFeaturePlan(analysis, features, planV1)
	if err != nil {
		t.Fatal(err)
	}
	planID, _ := plan.Identity()
	analysisID, _ := analysis.Identity()
	featuresID, _ := features.Identity()
	target, _ := ResolveTargetSpec("t", TargetSpecDraft{HorizonBars: 24, UpperATRMultiple: 1.5, LowerATRMultiple: 1.0}, "labels:v1")
	targetID, _ := target.Identity()

	perp := ForecastArtifactPinned{
		Market:   MarketKey{Venue: "BINANCE", Instrument: "BTCUSDT", Contract: "FUTURES_PERP", Timeframe: "15m"},
		Analysis: analysisID, Features: featuresID, Plan: planID, Target: targetID,
		SchemaVersion: "schema:v1",
	}
	spot := perp
	spot.Market = MarketKey{Venue: "BINANCE", Instrument: "BTCUSDT", Contract: "SPOT", Timeframe: "15m"}

	dPerp, err := perp.Digest()
	if err != nil {
		t.Fatal(err)
	}
	dSpot, err := spot.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if dPerp == dSpot {
		t.Fatal("spot vs perp MarketKey must produce different artifact pin digests")
	}
}

func TestTargetSpec_ATRIdentity(t *testing.T) {
	base := TargetSpecDraft{HorizonBars: 24, UpperATRMultiple: 1.5, LowerATRMultiple: 1.0}
	a, err := ResolveTargetSpec("n1", base, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ResolveTargetSpec("n2", TargetSpecDraft{
		HorizonBars: 24, UpperATRMultiple: 1.5, LowerATRMultiple: 1.0, ATRPeriod: 14,
		ATRMethod: indicators.ATRMethodWilderRMA, ATRLogic: indicators.ATRLogicWilderRMAFirstTRV1,
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	ida, _ := a.Identity()
	idb, _ := b.Identity()
	if ida.Digest != idb.Digest {
		t.Fatal("omitted ATR defaults must match explicit canonical ATRSpec")
	}
	c, err := ResolveTargetSpec("n1", TargetSpecDraft{
		HorizonBars: 24, UpperATRMultiple: 1.5, LowerATRMultiple: 1.0, ATRPeriod: 21,
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	idc, _ := c.Identity()
	if ida.Digest == idc.Digest {
		t.Fatal("ATR period must change TargetDigest")
	}
	mut := a
	mut.ATR.Method = "not_wilder"
	idm, _ := mut.Identity()
	if idm.Digest == ida.Digest {
		t.Fatal("ATR Method must change TargetDigest")
	}
	mut = a
	mut.ATR.Logic = "atr:wilder-sma-seed-v2"
	idl, _ := mut.Identity()
	if idl.Digest == ida.Digest {
		t.Fatal("ATR Logic must change TargetDigest")
	}
	if _, err := ResolveTargetSpec("bad", TargetSpecDraft{
		HorizonBars: 24, UpperATRMultiple: 1.5, LowerATRMultiple: 1.0, ATRPeriod: 14, ATRLogic: "nope",
	}, "labels:v1"); err == nil {
		t.Fatal("unknown ATR logic must refuse resolve")
	}
}
