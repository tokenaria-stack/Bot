package forecast

// ForecastArtifactPinned is the SPEC-1 minimal pin contract a future
// ForecastArtifact must satisfy (FORECAST-SPEC-1 §24). Weights, scaler,
// temperature, and RankPercentile CDFs are added in FORECAST-RUNTIME-1; this
// chapter only fixes what identity an artifact must pin so live can never
// independently mix "model vN + whatever AnalysisRecipe is on screen".
type ForecastArtifactPinned struct {
	Market   MarketKey
	Analysis Identity
	Features Identity
	Plan     Identity
	Target   Identity
	// SchemaVersion is the feature-vector schema version this artifact was
	// trained against. A runtime must REFUSE to load an artifact whose
	// SchemaVersion (or any pinned Identity digest) does not match the
	// runtime's own resolved identities.
	SchemaVersion LogicVersion
}

// Digest returns the full digest identity of this pin set. A future
// ForecastFrame.ArtifactID must reference an artifact's payload digest — not
// this pin-only digest — once weights/calibration exist; this method exists
// so the pin contract itself is comparable in this chapter's tests.
func (a ForecastArtifactPinned) Digest() (Digest, error) {
	return computeDigest(a)
}
