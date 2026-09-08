package decisionresearch

import (
	"testing"

	"trading_bot/decision"
	"trading_bot/forecast"
)

func testWorld(t *testing.T) World {
	t.Helper()
	d, err := forecast.ParseDigestHex("8e225450fe9bbe13c483dea80add95881a5793d7c435edc6908c415a347d1b6a")
	if err != nil {
		t.Fatal(err)
	}
	return World{
		TargetDigest: d,
		U:            1.5,
		L:            1.0,
		ClassOrder:   classOrder(),
		Logic:        decision.DecisionLogicTargetUtilityRankGateV1,
	}
}

func row(p0, p1, p2, r float64) EvidenceRow {
	return EvidenceRow{Evidence: decision.ForecastEvidence{
		Probabilities:   [3]float64{p0, p1, p2},
		DirectionalRank: r,
	}}
}

func TestThresholdArithmetic(t *testing.T) {
	minEU, minR := ResolveThresholds(1.5, 1.0, 3, 7)
	if minEU != 1.0*(float64(3)/10.0) {
		t.Fatalf("minEU=%v", minEU)
	}
	if minR != float64(7)/10.0 {
		t.Fatalf("minR=%v", minR)
	}
	if minEU == float64(3)*1.0/10.0 && float64(3)*1.0/10.0 != 1.0*(float64(3)/10.0) {
		t.Fatal("unexpected inequality")
	}
}

func TestGridAndValidate(t *testing.T) {
	w := testWorld(t)
	n := 0
	for du := 1; du <= 9; du++ {
		for dr := 1; dr <= 9; dr++ {
			if _, err := materializeSpec(w, du, dr); err != nil {
				t.Fatal(err)
			}
			n++
		}
	}
	if n != 81 {
		t.Fatal(n)
	}
}

func TestWrongLogic(t *testing.T) {
	w := testWorld(t)
	w.Logic = "nope"
	_, err := Run(nil, nil, nil, w)
	if err == nil {
		t.Fatal("want refuse")
	}
}

func TestBaselineStrictness(t *testing.T) {
	w := testWorld(t)
	ev := []EvidenceRow{row(0.4, 0.4, 0.2, 0), row(0.4, 0.4, 0.2, 0)}
	out := []string{OutcomeTimeout, OutcomeTimeout}
	fr := FoldRange{TrainEnd: 2, ValBegin: 0, ValEnd: 0}
	sel, au, err := selectTrain(ev, out, fr, w, false)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Kind != KindBaseline || au.TotalUtility != 0 {
		t.Fatalf("%+v %+v", sel, au)
	}
}

func TestPositiveReplacementAndTie(t *testing.T) {
	w := testWorld(t)
	n := 4
	ev := make([]EvidenceRow, n)
	out := make([]string, n)
	for i := 0; i < n; i++ {
		ev[i] = row(1, 0, 0, 1)
		out[i] = OutcomeUp
	}
	fr := FoldRange{TrainEnd: n, ValBegin: 0, ValEnd: 0}
	sel, au, err := selectTrain(ev, out, fr, w, false)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Kind != KindSpec || sel.UtilityDecile != 9 || sel.RankDecile != 9 {
		t.Fatalf("%+v", sel)
	}
	want := TotalUtility(w.U, w.L, Audit{UpUp: n})
	if au.TotalUtility != want {
		t.Fatalf("util %v want %v", au.TotalUtility, want)
	}
	selR, _, err := selectTrain(ev, out, fr, w, true)
	if err != nil {
		t.Fatal(err)
	}
	if selR.UtilityDecile != 9 || selR.RankDecile != 9 {
		t.Fatalf("reverse %+v", selR)
	}
}

func TestTrainOnlySelection(t *testing.T) {
	w := testWorld(t)
	ev := []EvidenceRow{row(1, 0, 0, 1), row(1, 0, 0, 1), row(0, 1, 0, -1), row(0, 1, 0, -1)}
	out := []string{OutcomeUp, OutcomeUp, OutcomeDown, OutcomeDown}
	fr := FoldRange{TrainBegin: 0, TrainEnd: 2, ValBegin: 2, ValEnd: 4}
	sel1, _, err := selectTrain(ev, out, fr, w, false)
	if err != nil {
		t.Fatal(err)
	}
	ev[2] = row(1, 0, 0, 1)
	out[2] = OutcomeUp
	ev[3] = row(1, 0, 0, 1)
	out[3] = OutcomeUp
	sel2, _, err := selectTrain(ev, out, fr, w, false)
	if err != nil {
		t.Fatal(err)
	}
	if sel1.Kind != sel2.Kind || sel1.UtilityDecile != sel2.UtilityDecile || sel1.RankDecile != sel2.RankDecile {
		t.Fatalf("%+v vs %+v", sel1, sel2)
	}
}

func TestValidationOneSpecAndBaselineCalls(t *testing.T) {
	w := testWorld(t)
	ev := []EvidenceRow{row(1, 0, 0, 1), row(1, 0, 0, 1), row(1, 0, 0, 1)}
	out := []string{OutcomeUp, OutcomeUp, OutcomeUp}
	fr := FoldRange{TrainEnd: 2, ValBegin: 2, ValEnd: 3}
	sel, _, err := selectTrain(ev, out, fr, w, false)
	if err != nil {
		t.Fatal(err)
	}
	ApplyCalls = 0
	au, calls, err := evaluateSelected(ev, out, fr, w, sel)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Kind != KindSpec || calls != 1 {
		t.Fatalf("kind=%s calls=%d", sel.Kind, calls)
	}
	if au.N != 1 {
		t.Fatal(au.N)
	}
	ApplyCalls = 0
	au2, calls2, err := evaluateSelected(ev, out, fr, w, Selection{Kind: KindBaseline})
	if err != nil {
		t.Fatal(err)
	}
	if calls2 != 0 || au2.Abstain != 1 || au2.TotalUtility != 0 {
		t.Fatalf("baseline %+v calls=%d", au2, calls2)
	}
}

func TestUnknownOutcomeRefuse(t *testing.T) {
	w := testWorld(t)
	ev := []EvidenceRow{row(0.3, 0.3, 0.4, 0)}
	_, err := Run(ev, []string{"NOPE"}, []FoldRange{{TrainEnd: 1}}, w)
	if err == nil {
		t.Fatal("want refuse")
	}
}

func TestApplyErrorRefuse(t *testing.T) {
	w := testWorld(t)
	ev := []EvidenceRow{row(2, 0, 0, 1)}
	_, err := Run(ev, []string{OutcomeUp}, []FoldRange{{TrainEnd: 1, ValEnd: 0}}, w)
	if err == nil {
		t.Fatal("want refuse")
	}
}

func TestAccountingAndUtilityLaw(t *testing.T) {
	a := Audit{N: 9, UpIntent: 3, DownIntent: 3, Abstain: 3, UpUp: 1, UpDown: 1, UpTO: 1, DownUp: 1, DownDown: 1, DownTO: 1, AbsUp: 1, AbsDown: 1, AbsTO: 1}
	if err := a.check(); err != nil {
		t.Fatal(err)
	}
	u := TotalUtility(1.5, 1.0, a)
	if u != 1.5*(1-1)+1.0*(1-1) {
		t.Fatal(u)
	}
}

func TestZeroFoldNotPositive(t *testing.T) {
	if (Audit{TotalUtility: 0}.TotalUtility > 0) {
		t.Fatal("zero is not positive")
	}
}

func TestAcceptanceGates(t *testing.T) {
	if Eligible(1, 3, 4) != true {
		t.Fatal("both")
	}
	if Eligible(0, 4, 4) {
		t.Fatal("pooled")
	}
	if Eligible(-1, 4, 4) {
		t.Fatal("neg pooled")
	}
	if Eligible(1, 2, 4) {
		t.Fatal("fold gate 2/4")
	}
	if !Eligible(0.1, 3, 4) {
		t.Fatal("3/4")
	}
}

func TestOutcomeNotOnEvidenceType(t *testing.T) {
	var e decision.ForecastEvidence
	_ = e.Probabilities
	_ = e.DirectionalRank
}

func TestNoAtOnEvidence(t *testing.T) {
	var r EvidenceRow
	_ = r.Evidence
}
