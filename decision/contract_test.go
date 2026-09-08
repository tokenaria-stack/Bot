package decision

import (
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"trading_bot/forecast"
)

func validSpec(t *testing.T) DecisionSpec {
	t.Helper()
	d, err := forecast.ParseDigestHex("8e225450fe9bbe13c483dea80add95881a5793d7c435edc6908c415a347d1b6a")
	if err != nil {
		t.Fatal(err)
	}
	return DecisionSpec{
		Logic:                    DecisionLogicTargetUtilityRankGateV1,
		TargetDigest:             d,
		UpperBarrierUtility:      1.5,
		LowerBarrierUtility:      1.0,
		MinExpectedTargetUtility: 0.25,
		MinAbsDirectionalRank:    0.5,
		ClassOrder:               canonicalClassOrder(),
	}
}

func TestValidateDecisionSpec(t *testing.T) {
	t.Parallel()
	ok := validSpec(t)
	if err := ValidateDecisionSpec(ok); err != nil {
		t.Fatal(err)
	}
	nan := ok
	nan.UpperBarrierUtility = math.NaN()
	if err := ValidateDecisionSpec(nan); err == nil {
		t.Fatal("NaN U")
	}
	inf := ok
	inf.MinAbsDirectionalRank = math.Inf(1)
	if err := ValidateDecisionSpec(inf); err == nil {
		t.Fatal("Inf rank")
	}
	u0 := ok
	u0.UpperBarrierUtility = 0
	if err := ValidateDecisionSpec(u0); err == nil {
		t.Fatal("U<=0")
	}
	lneg := ok
	lneg.LowerBarrierUtility = -1
	if err := ValidateDecisionSpec(lneg); err == nil {
		t.Fatal("L<=0")
	}
	eu0 := ok
	eu0.MinExpectedTargetUtility = 0
	if err := ValidateDecisionSpec(eu0); err == nil {
		t.Fatal("min_EU<=0")
	}
	// U=1.5 L=1.0 min_EU=1.2 is unreachable for DOWN
	side := ok
	side.MinExpectedTargetUtility = 1.2
	if err := ValidateDecisionSpec(side); err == nil {
		t.Fatal("min_EU>=min(U,L)")
	}
	eq := ok
	eq.MinExpectedTargetUtility = 1.0
	if err := ValidateDecisionSpec(eq); err == nil {
		t.Fatal("min_EU==min(U,L)")
	}
	r0 := ok
	r0.MinAbsDirectionalRank = 0
	if err := ValidateDecisionSpec(r0); err == nil {
		t.Fatal("min_rank<=0")
	}
	r1 := ok
	r1.MinAbsDirectionalRank = 1
	if err := ValidateDecisionSpec(r1); err == nil {
		t.Fatal("min_rank>=1")
	}
	ord := ok
	ord.ClassOrder = [3]string{"DOWN_FIRST", "UP_FIRST", "TIMEOUT"}
	if err := ValidateDecisionSpec(ord); err == nil {
		t.Fatal("class reorder")
	}
}

func TestValidateForecastEvidence(t *testing.T) {
	t.Parallel()
	good := ForecastEvidence{Probabilities: [3]float64{0.4, 0.3, 0.3}, DirectionalRank: 0.1}
	if err := ValidateForecastEvidence(good); err != nil {
		t.Fatal(err)
	}
	if err := ValidateForecastEvidence(ForecastEvidence{Probabilities: [3]float64{math.NaN(), 0.3, 0.3}}); err == nil {
		t.Fatal("NaN P")
	}
	if err := ValidateForecastEvidence(ForecastEvidence{Probabilities: [3]float64{1.1, 0, 0}}); err == nil {
		t.Fatal("P>1")
	}
	if err := ValidateForecastEvidence(ForecastEvidence{Probabilities: [3]float64{0.4, 0.3, 0.3}, DirectionalRank: math.Inf(-1)}); err == nil {
		t.Fatal("Inf rank")
	}
	if err := ValidateForecastEvidence(ForecastEvidence{Probabilities: [3]float64{0.4, 0.3, 0.3}, DirectionalRank: 1.01}); err == nil {
		t.Fatal("rank>1")
	}
}

func TestApplyDecision_InvalidEvidenceNotAbstain(t *testing.T) {
	t.Parallel()
	s := validSpec(t)
	intent, err := ApplyDecision(ForecastEvidence{Probabilities: [3]float64{-0.1, 0.5, 0.6}, DirectionalRank: 0}, s)
	if err == nil || intent == IntentAbstain {
		t.Fatalf("invalid evidence must error, got %q %v", intent, err)
	}
}

func TestExpectedTargetUtilityAndIdentity(t *testing.T) {
	t.Parallel()
	s := validSpec(t)
	e := ForecastEvidence{Probabilities: [3]float64{0.5, 0.25, 0.25}, DirectionalRank: 0}
	eu, err := ExpectedTargetUtility(e, s)
	if err != nil {
		t.Fatal(err)
	}
	want := 1.5*0.5 - 1.0*0.25
	if eu != want {
		t.Fatalf("EU=%v want %v", eu, want)
	}
	if -eu != -want {
		t.Fatal("EU_DOWN identity")
	}
}

func TestApplyDecision_BoundariesAndGates(t *testing.T) {
	t.Parallel()
	s := validSpec(t)
	// EU = 1.5*p0 - 1.0*p1; minEU=0.25 minR=0.5 (binary-exact fixtures)
	upE := ForecastEvidence{Probabilities: [3]float64{0.5, 0.5, 0}, DirectionalRank: 0.5}
	got, err := ApplyDecision(upE, s)
	if err != nil || got != IntentUp {
		t.Fatalf("exact UP: %q %v", got, err)
	}
	downE := ForecastEvidence{Probabilities: [3]float64{0.25, 0.625, 0.125}, DirectionalRank: -0.5}
	got, err = ApplyDecision(downE, s)
	if err != nil || got != IntentDown {
		t.Fatalf("exact DOWN: %q %v eu=%v", got, err, expectedTargetUtility(downE, s))
	}
	rankDisagree := ForecastEvidence{Probabilities: [3]float64{0.5, 0.5, 0}, DirectionalRank: -0.5}
	got, err = ApplyDecision(rankDisagree, s)
	if err != nil || got != IntentAbstain {
		t.Fatalf("rank disagree: %q %v", got, err)
	}
	rankDisagreeDown := ForecastEvidence{Probabilities: [3]float64{0.25, 0.625, 0.125}, DirectionalRank: 0.5}
	got, err = ApplyDecision(rankDisagreeDown, s)
	if err != nil || got != IntentAbstain {
		t.Fatalf("rank disagree down: %q %v", got, err)
	}
	weakEU := ForecastEvidence{Probabilities: [3]float64{0.25, 0.25, 0.5}, DirectionalRank: 0.75}
	got, err = ApplyDecision(weakEU, s)
	if err != nil || got != IntentAbstain {
		t.Fatalf("weak EU: %q %v", got, err)
	}
	lowTO := ForecastEvidence{Probabilities: [3]float64{0.5, 0.5, 0}, DirectionalRank: 0.5}
	highTO := ForecastEvidence{Probabilities: [3]float64{0.25, 0.25, 0.5}, DirectionalRank: 0.5}
	a, err := ApplyDecision(lowTO, s)
	if err != nil || a != IntentUp {
		t.Fatalf("low timeout: %q %v", a, err)
	}
	b, err := ApplyDecision(highTO, s)
	if err != nil || b != IntentAbstain {
		t.Fatalf("TIMEOUT mass shrinks |EU|: %q %v", b, err)
	}
}

func TestNoTimeoutKnobAndLegacyIsolation(t *testing.T) {
	t.Parallel()
	files := []string{"apply.go", "contract.go"}
	for _, name := range files {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		if strings.Contains(text, "max_timeout") || strings.Contains(text, "timeout_threshold") {
			t.Fatalf("%s timeout knob", name)
		}
		if name != "contract.go" {
			continue
		}
	}
	src, err := os.ReadFile("apply.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, banned := range []string{"BUY", "SELL", "WAIT", "LotMod", "StopDist", "ScoreDecision", "AvailableAt"} {
		if strings.Contains(text, banned) {
			t.Fatalf("legacy %s", banned)
		}
	}
}

func TestMutualExclusivity(t *testing.T) {
	t.Parallel()
	s := validSpec(t)
	minEU, minR := s.MinExpectedTargetUtility, s.MinAbsDirectionalRank
	for _, e := range []ForecastEvidence{
		{Probabilities: [3]float64{0.5, 0.1, 0.4}, DirectionalRank: 0.9},
		{Probabilities: [3]float64{0.1, 0.6, 0.3}, DirectionalRank: -0.9},
		{Probabilities: [3]float64{1.0 / 3, 1.0 / 3, 1.0 / 3}, DirectionalRank: 0},
		{Probabilities: [3]float64{0.5, 0.5, 0}, DirectionalRank: 0.5},
		{Probabilities: [3]float64{0.25, 0.625, 0.125}, DirectionalRank: -0.5},
	} {
		eu := expectedTargetUtility(e, s)
		up := eu >= minEU && e.DirectionalRank >= minR
		down := -eu >= minEU && e.DirectionalRank <= -minR
		if up && down {
			t.Fatalf("both intents true eu=%v rank=%v", eu, e.DirectionalRank)
		}
		got, err := ApplyDecision(e, s)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case up && got != IntentUp:
			t.Fatalf("want UP got %q", got)
		case down && got != IntentDown:
			t.Fatalf("want DOWN got %q", got)
		case !up && !down && got != IntentAbstain:
			t.Fatalf("want ABSTAIN got %q", got)
		}
	}
}

func TestApplyDecisionSignature(t *testing.T) {
	t.Parallel()
	ft := reflect.TypeOf(ApplyDecision)
	if ft.NumIn() != 2 || ft.NumOut() != 2 {
		t.Fatalf("signature in=%d out=%d", ft.NumIn(), ft.NumOut())
	}
	if ft.In(0) != reflect.TypeOf(ForecastEvidence{}) || ft.In(1) != reflect.TypeOf(DecisionSpec{}) {
		t.Fatalf("inputs %v %v", ft.In(0), ft.In(1))
	}
	if ft.Out(0) != reflect.TypeOf(DirectionalIntent("")) {
		t.Fatalf("out0 %v", ft.Out(0))
	}
}
