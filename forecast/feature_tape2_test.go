package forecast

import (
	"os"
	"path/filepath"
	"testing"
)

func testTape2Header(t *testing.T) Tape2Header {
	t.Helper()
	target, err := ResolveTargetSpec("research-15m-1m-c", TargetSpecDraft{
		HorizonBars: Spec2TargetHorizon, UpperATRMultiple: Spec2UpperATR, LowerATRMultiple: Spec2LowerATR,
		ATRPeriod: 14, DualHit: DualHitResolveFinerHistory, FinerTimeframe: "1m",
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := ResolveAnalysisRecipe("spec2", AnalysisRecipeDraft{
		RSXLength: Spec2RSXLength, RSXSignal: Spec2RSXSignal, RSXSource: Spec2RSXSource,
		DivLookback: Spec2TVLookback, EnableTV: true,
	}, "analysis:v2")
	if err != nil {
		t.Fatal(err)
	}
	primary := MarketKey{Venue: "BINANCE", Instrument: "BTCUSDT", Contract: "FUTURES_PERP", Timeframe: "15m"}
	s, err := ResolveFeatureSpec2(primary, target, analysis)
	if err != nil {
		t.Fatal(err)
	}
	bars := []CanonicalClosedBar{{OpenTime: 1, Open: 1, High: 2, Low: 0.5, Close: 1.2, Volume: 1}}
	d := SourceRangeDigest(s.Primary, bars)
	d1 := SourceRangeDigest(s.HTF1h, bars)
	d4 := SourceRangeDigest(s.HTF4h, bars)
	sid, _ := s.Identity()
	pid, _ := s.Plan.Identity()
	fid, _ := s.Features.Identity()
	aid, _ := s.Analysis.Identity()
	tid, _ := s.Target.Identity()
	return Tape2Header{
		FormatVersion: FeatureTapeFormatV2, SpecDigest: sid.Digest, PlanDigest: pid.Digest,
		FeaturesDigest: fid.Digest, AnalysisDigest: aid.Digest, TargetDigest: tid.Digest,
		Primary: s.Primary, HTF1h: s.HTF1h, HTF4h: s.HTF4h, FeatureIDs: FeatureSpec2IDs(),
		VectorLen: Spec2FeatureWidth, Q: s.Q, Demand: s.Demand,
		PrimarySource: d, HTF1hSource: d1, HTF4hSource: d4,
	}
}

func TestTape2_RoundtripNotReadyOmitsValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.featuretape2")
	h := testTape2Header(t)
	var vec FeatureVector2
	for i := range vec {
		vec[i] = float64(i) + 0.25
	}
	w, err := CreateTape2Writer(path, h)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteRow(100, NotReady, ReasonPrimaryWarmup, nil); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteRow(200, IsReady, "", vec.Slice()); err != nil {
		t.Fatal(err)
	}
	foot, err := w.Finish()
	if err != nil {
		t.Fatal(err)
	}
	hdr, rows, f2, err := ReadTape2(path)
	if err != nil {
		t.Fatal(err)
	}
	if hdr.FormatVersion != FeatureTapeFormatV2 || f2.ContentDigest != foot.ContentDigest {
		t.Fatal("digest")
	}
	if rows[0].Ready || rows[0].Values != (FeatureVector2{}) {
		t.Fatal("notready must not carry vector")
	}
	if !rows[1].Ready || rows[1].Values[0] != 0.25 {
		t.Fatal("ready")
	}
	if f2.ReadyCount != 1 || f2.NotReadyCount != 1 {
		t.Fatal(f2)
	}
}

func TestTape2_RejectsV1Header(t *testing.T) {
	h := testTape2Header(t)
	h.FormatVersion = FeatureTapeFormatV1
	if _, err := CreateTape2Writer(filepath.Join(t.TempDir(), "x"), h); err == nil {
		t.Fatal("v1 format on v2 writer")
	}
}

func TestTape2_RefuseExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t")
	if err := os.WriteFile(path, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTape2Writer(path, testTape2Header(t)); err == nil {
		t.Fatal("overwrite")
	}
}
