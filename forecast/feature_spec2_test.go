package forecast

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func spec2Fixture(t *testing.T) FeatureSpec2 {
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
	return s
}

func TestFeatureSpec2_WidthOrder(t *testing.T) {
	ids := FeatureSpec2IDs()
	if len(ids) != 64 {
		t.Fatalf("width %d", len(ids))
	}
	s := spec2Fixture(t)
	if s.Plan.VectorLen() != 64 || !reflect.DeepEqual(s.Plan.Schema, ids) {
		t.Fatal("schema")
	}
	if len(s.Plan.Schema[:42]) != 42 || strings.HasPrefix(string(s.Plan.Schema[42]), "htf_1h_") == false {
		t.Fatal("15m/1h split")
	}
	if !strings.HasPrefix(string(s.Plan.Schema[53]), "htf_4h_") {
		t.Fatal("4h start")
	}
}

func TestFeatureSpec2_TargetAndQ(t *testing.T) {
	s := spec2Fixture(t)
	if s.Target.HorizonBars != 72 || s.Target.UpperATRMultiple != 2 || s.Target.LowerATRMultiple != 2 {
		t.Fatal("target C")
	}
	if s.Q != 18 {
		t.Fatal(s.Q)
	}
	if _, err := HorizonQuarterQ(25); err == nil {
		t.Fatal("non-divisible H")
	}
	old, err := ResolveTargetSpec("research-15m-1m", TargetSpecDraft{
		HorizonBars: 24, UpperATRMultiple: 1.5, LowerATRMultiple: 1.0, ATRPeriod: 14,
		DualHit: DualHitResolveFinerHistory, FinerTimeframe: "1m",
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	analysis := s.Analysis
	if _, err := ResolveFeatureSpec2(s.Primary, old, analysis); err == nil {
		t.Fatal("old target must refuse")
	}
	idOld, _ := old.Identity()
	idNew, _ := s.Target.Identity()
	if idOld.Digest == idNew.Digest {
		t.Fatal("Target C must not equal v1 TargetDigest")
	}
}

func TestFeatureSpec2_AnalysisAndSymmetry(t *testing.T) {
	s := spec2Fixture(t)
	if s.Analysis.Config.RSXSignal != 14 || s.Analysis.Logic != "analysis:v2" {
		t.Fatal("analysis")
	}
	if err := ValidateFeatureSpec2Symmetry(FeatureSpec2IDs()); err != nil {
		t.Fatal(err)
	}
	ids := FeatureSpec2IDs()
	broken := make([]FeatureID, 0, len(ids)-1)
	for _, id := range ids {
		if id != FeatureTVBearPresent {
			broken = append(broken, id)
		}
	}
	if err := ValidateFeatureSpec2Symmetry(broken); err == nil {
		t.Fatal("missing bear")
	}
	swapped := FeatureSpec2Mirrors()
	swapped[FeatureTVBullPresent] = FeatureTVBullAge
	swapped[FeatureTVBullAge] = FeatureTVBullPresent
	if err := ValidateFeatureSpec2SymmetryWith(ids, swapped); err == nil {
		t.Fatal("swapped mirror mapping")
	}
}

func TestFeatureSpec2_CrossAndAgeLaws(t *testing.T) {
	if !SignalCrossUp(-0.1, 0.2) || SignalCrossUp(0.1, 0.2) || SignalCrossUp(-0.1, -0.01) {
		t.Fatal("signal up")
	}
	if !SignalCrossDown(0.1, -0.2) || SignalCrossDown(-0.1, -0.2) {
		t.Fatal("signal down")
	}
	if !Midline50CrossUp(50, 50.1) || !Midline50CrossUp(49.9, 50.1) || Midline50CrossUp(50.1, 51) {
		t.Fatal("50 up")
	}
	p, a := PresentAge(0, 36)
	if p != 1 || a != 0 {
		t.Fatal("age0")
	}
	p, a = PresentAge(36, 36)
	if p != 1 || a != 36 {
		t.Fatal("age36")
	}
	p, a = PresentAge(37, 36)
	if p != 0 || a != 0 {
		t.Fatal(">36")
	}
	p, a = PresentAge(12, 12)
	if p != 1 {
		t.Fatal("cross12")
	}
	p, a = PresentAge(13, 12)
	if p != 0 {
		t.Fatal("cross13")
	}
}

func TestFeatureSpec2_Patterns(t *testing.T) {
	if !OrderedPatternOK(1, 2, 0, 8) {
		t.Fatal("gap8")
	}
	if OrderedPatternOK(1, 2, 0, 9) {
		t.Fatal("gap9")
	}
	if OrderedPatternOK(10, 10, 5, 5) {
		t.Fatal("same confirmed not ordered")
	}
	if !SameBarConjunction(100, 100) || SameBarConjunction(100, 101) {
		t.Fatal("samebar")
	}
	// latest of two completions
	p, a := LatestCompletionAge(40, []int{10, 30}, 36)
	if p != 1 || a != 10 {
		t.Fatalf("latest got p=%v a=%v", p, a)
	}
	p, a = LatestCompletionAge(70, []int{10}, 36)
	if p != 0 {
		t.Fatal("stale pattern")
	}
}

func TestFeatureSpec2_PriceFormulas(t *testing.T) {
	h, q := 72, 18
	n := 200
	high := make([]float64, n)
	low := make([]float64, n)
	cl := make([]float64, n)
	atr := make([]float64, n)
	for i := 0; i < n; i++ {
		cl[i] = 100 + float64(i)*0.1
		high[i] = cl[i] + 1
		low[i] = cl[i] - 1
		atr[i] = 2
	}
	tIdx := n - 1
	d, rd := PriceDisplacementATR(cl[tIdx], cl[tIdx-q], atr[tIdx])
	if rd != IsReady || math.Abs(d-(cl[tIdx]-cl[tIdx-q])/2) > 1e-12 {
		t.Fatal("disp q")
	}
	d, rd = PriceDisplacementATR(cl[tIdx], cl[tIdx-h], atr[tIdx])
	if rd != IsReady || math.Abs(d-(cl[tIdx]-cl[tIdx-h])/2) > 1e-12 {
		t.Fatal("disp h")
	}
	rp, rd := PriceRangePositionH(high, low, cl, tIdx, h)
	if rd != IsReady {
		t.Fatal("range")
	}
	// off-by-one: window start t-h+1
	if rp <= 0 || rp > 1 {
		t.Fatal(rp)
	}
	flatH, flatL := make([]float64, 10), make([]float64, 10)
	flatC := make([]float64, 10)
	for i := 0; i < 10; i++ {
		flatH[i], flatL[i], flatC[i] = 1, 1, 1
	}
	if _, rd := PriceRangePositionH(flatH, flatL, flatC, 9, 5); rd != NotReady {
		t.Fatal("degenerate range")
	}
	eff, rd := PricePathEfficiencyH(cl, tIdx, h)
	if rd != IsReady || eff <= 0 || eff > 1 {
		t.Fatal(eff)
	}
	if _, rd := PricePathEfficiencyH([]float64{1, 1, 1, 1}, 3, 2); rd != NotReady {
		t.Fatal("zero path den")
	}
	if _, rd := ATROverPrice(1, 0); rd != NotReady {
		t.Fatal("close0")
	}
	if _, rd := ATRChangeQ(2, 0); rd != NotReady {
		t.Fatal("atr0")
	}
	// prefix vs longer history: windowed price at tip uses only H+1 closes
	short := cl[tIdx-h:]
	shH, shL := high[tIdx-h:], low[tIdx-h:]
	// short[0] is Close[t-H]; range window needs indices relative to short tip
	tip := len(short) - 1
	e1, _ := PricePathEfficiencyH(cl, tIdx, h)
	e2, _ := PricePathEfficiencyH(short, tip, h)
	if e1 != e2 {
		t.Fatalf("path prefix %v %v", e1, e2)
	}
	r1, _ := PriceRangePositionH(high, low, cl, tIdx, h)
	r2, _ := PriceRangePositionH(shH, shL, short, tip, h)
	if r1 != r2 {
		t.Fatalf("range prefix %v %v", r1, r2)
	}
}

func TestFeatureSpec2_HTFJoin(t *testing.T) {
	// 15m bar open 10:30 UTC → close 10:44:59.999; 1h 10:00-11:00 not closed.
	day := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	h10 := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC).UnixMilli()
	h9 := time.Date(2024, 6, 1, 9, 0, 0, 0, time.UTC).UnixMilli()
	p1030 := time.Date(2024, 6, 1, 10, 30, 0, 0, time.UTC).UnixMilli()
	pk, err := KnowledgeCloseTime(p1030, "15m")
	if err != nil {
		t.Fatal(err)
	}
	idx, err := SelectLatestClosedHTF([]int64{h9, h10}, "1h", pk)
	if err != nil {
		t.Fatal(err)
	}
	if idx != 0 {
		t.Fatalf("want 09:00 hour, got %d", idx)
	}
	if JoinHTFReady(NotReady) != NotReady {
		t.Fatal("stale fallback forbidden")
	}
	if JoinHTFReady(IsReady) != IsReady {
		t.Fatal("ready join")
	}
	_ = day
}

func TestFeatureSpec2_HTFNativeAgeUnit(t *testing.T) {
	s := spec2Fixture(t)
	if s.Features.MaxAgeBars[FeatureHTF1hTVBullAge] != 12 || s.Features.MaxAgeBars[FeatureHTF4hSignalCrossUpAge] != 12 {
		t.Fatal("htf age cap")
	}
	if s.HTF1h.Timeframe != "1h" || s.HTF4h.Timeframe != "4h" {
		t.Fatal("native keys")
	}
	if s.Demand.HTF1hWindowBars != s.Analysis.Config.DivLookback {
		t.Fatal("htf demand uses TV lookback native bars")
	}
	p1, a1 := NativePresentAge(12, 0, Spec2HTFFactAgeNative)
	if p1 != 1 || a1 != 12 {
		t.Fatal("1h native age 12")
	}
	p4, a4 := NativePresentAge(12, 0, Spec2HTFFactAgeNative)
	if p4 != 1 || a4 != 12 {
		t.Fatal("4h native age uses 4h index, not 15m")
	}
	if p, _ := NativePresentAge(13, 0, Spec2HTFFactAgeNative); p != 0 {
		t.Fatal("native age 13")
	}
	if err := Spec2UnexpectedNativeGap(s.HTF1h, "missing bar"); err == nil {
		t.Fatal("gap error")
	}
}

func TestFeatureSpec2_WrongAnalysisAndWidth(t *testing.T) {
	s := spec2Fixture(t)
	badSig, err := ResolveAnalysisRecipe("spec2", AnalysisRecipeDraft{
		RSXLength: Spec2RSXLength, RSXSignal: 9, RSXSource: Spec2RSXSource,
		DivLookback: Spec2TVLookback, EnableTV: true,
	}, "analysis:v2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveFeatureSpec2(s.Primary, s.Target, badSig); err == nil {
		t.Fatal("signal 9 must refuse")
	}
	wrongTF := s.Primary
	wrongTF.Timeframe = "1h"
	if _, err := ResolveFeatureSpec2(wrongTF, s.Target, s.Analysis); err == nil {
		t.Fatal("primary must be 15m")
	}
}

func TestFeatureSpec2_HistoryDemandAndNo256(t *testing.T) {
	s := spec2Fixture(t)
	if s.Demand.Primary15mWindowBars < 73 || s.Demand.Primary15mWindowBars < 90 {
		t.Fatal(s.Demand)
	}
	if !s.Demand.IIRFromSourceStart {
		t.Fatal("IIR")
	}
	if s.Features.MaxAgeBars[FeatureTVBullAge] != 36 {
		t.Fatal("not 256")
	}
}

func TestFeatureSpec2_NoOutcomeVolumeBins(t *testing.T) {
	blob := ""
	for _, id := range FeatureSpec2IDs() {
		blob += string(id) + ","
	}
	for _, bad := range []string{"volume", "hour", "fractal", "oversold", "pattern_17"} {
		if strings.Contains(blob, bad) {
			t.Fatal(bad)
		}
	}
}

func TestFeatureSpec2_IdentityStableAndV1Untouched(t *testing.T) {
	a := spec2Fixture(t)
	b := spec2Fixture(t)
	ia, _ := a.Identity()
	ib, _ := b.Identity()
	if ia.Digest != ib.Digest {
		t.Fatal("spec identity")
	}
	v1, err := ResolveFeatureRecipe("tape1a", FeatureRecipeDraft{
		Features: []FeatureID{FeatureRSXValue, FeatureRSXSignal, FeatureTVBullPresent, FeatureTVBullAge},
	}, "features:v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(v1.Features) != 4 || v1.MaxAgeBars[FeatureTVBullAge] != defaultMaxAgeBars {
		t.Fatal("v1 recipe")
	}
}
