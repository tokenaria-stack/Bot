package forecast

import (
	"strings"
	"testing"

	"trading_bot/data"
)

func TestIgnitionAge0_NotRememberedPresent(t *testing.T) {
	t.Parallel()
	var row FeatureRow2
	row.Ready = IsReady
	row.Values[spec2Col(FeatureRSX50CrossUpPresent)] = 1
	row.Values[spec2Col(FeatureRSX50CrossUpAge)] = 5
	if ignitionAge0(row, FeatureRSX50CrossUpPresent, FeatureRSX50CrossUpAge) {
		t.Fatal("present=1 age=5 is memory, not ignition")
	}
	row.Values[spec2Col(FeatureRSX50CrossUpAge)] = 0
	if !ignitionAge0(row, FeatureRSX50CrossUpPresent, FeatureRSX50CrossUpAge) {
		t.Fatal("present=1 age=0 is ignition")
	}
	row.Ready = NotReady
	if ignitionAge0(row, FeatureRSX50CrossUpPresent, FeatureRSX50CrossUpAge) {
		t.Fatal("NotReady cannot ignite")
	}
}

func TestZoneExitLawExact(t *testing.T) {
	t.Parallel()
	mk := func(rsx float64) FeatureRow2 {
		var r FeatureRow2
		r.Ready = IsReady
		r.Values[spec2Col(FeatureRSXValue)] = rsx
		return r
	}
	if !zoneExitLong(mk(30), mk(30.01)) {
		t.Fatal("prev<=30 && cur>30")
	}
	if zoneExitLong(mk(30), mk(30)) {
		t.Fatal("cur must be >30")
	}
	if zoneExitLong(mk(30.01), mk(31)) {
		t.Fatal("prev must be <=30")
	}
	if !zoneExitShort(mk(70), mk(69.99)) {
		t.Fatal("prev>=70 && cur<70")
	}
	if zoneExitShort(mk(70), mk(70)) {
		t.Fatal("cur must be <70")
	}
}

func TestTicketBarriersMirror(t *testing.T) {
	t.Parallel()
	uL, lL := ticketBarriers(100, 2, 1, true)
	if uL != 104 || lL != 98 {
		t.Fatalf("LONG k=1 v=2: upper=%g lower=%g", uL, lL)
	}
	uS, lS := ticketBarriers(100, 2, 1, false)
	if uS != 102 || lS != 96 {
		t.Fatalf("SHORT k=1 v=2: upper=%g lower=%g", uS, lS)
	}
	if uL-100 != 100-lS || 100-lL != uS-100 {
		t.Fatal("LONG/SHORT must mirror TP/SL distances")
	}
}

func TestLastClosed1hATR_NoStaleFallback(t *testing.T) {
	t.Parallel()
	step, err := data.IntervalDurationMs("1h")
	if err != nil {
		t.Fatal(err)
	}
	open0 := int64(1_609_459_200_000)
	h1 := []CanonicalClosedBar{
		{OpenTime: open0, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1},
		{OpenTime: open0 + step, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1},
	}
	atr := []float64{2, 0}
	ct1, err := data.BarCloseTimeMs(h1[1].OpenTime, "1h")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lastClosedHTFATR(h1, atr, "1h", ct1); ok {
		t.Fatal("ATR<=0 on latest closed 1h must skip; must not fall back to prior Ready ATR")
	}
	ct0, err := data.BarCloseTimeMs(h1[0].OpenTime, "1h")
	if err != nil {
		t.Fatal(err)
	}
	v, ok := lastClosedHTFATR(h1, atr, "1h", ct0)
	if !ok || v != 2 {
		t.Fatalf("first bar usable v=%g ok=%v", v, ok)
	}
	if _, ok := lastClosedHTFATR(h1, atr, "1h", h1[0].OpenTime); ok {
		t.Fatal("forming/unclosed 1h must be invisible")
	}
}

func TestPathExcursionContinuesAfterFirstHit(t *testing.T) {
	t.Parallel()
	const h = 72
	bars := stampBars(t, boundBars(h+5))
	t0 := 2
	entry := bars[t0].Close
	bars[t0+1].Low = entry - 2
	bars[t0+1].High = entry
	bars[t0+10].High = entry + 5
	mfe, mae, _, _, _, _, _, _, _, _, complete, err := pathExcursion(bars, t0, h, "15m", entry, 1, true)
	if err != nil || !complete {
		t.Fatalf("complete=%v err=%v", complete, err)
	}
	if mae < 2 {
		t.Fatalf("MAE want >=2 got %g", mae)
	}
	if mfe < 5 {
		t.Fatalf("MFE must continue after adverse hit, got %g", mfe)
	}
}

func TestEvaluateBarriers_UsesCanonicalFinerDualHit(t *testing.T) {
	t.Parallel()
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	primary, c, parent, upper, lower := dualHitFixture(t, spec)
	mins := minuteBars(t, primary[parent].OpenTime, 15, 100, 100.2, 99.8)
	mins[0].High = upper
	mins[0].Low = 100
	fm := finerMarketOf(testTapeHeader().Market, "1m")
	got, err := EvaluateBarriers(primary, c, upper, lower, spec.HorizonBars, "15m", NewFinerBarrierResolver(fm, "15m", mins))
	if err != nil {
		t.Fatal(err)
	}
	_, rows, _ := genFinerLabels(t, spec, primary[c:c+1], primary, mins)
	if got.Outcome != rows[0].Outcome || got.Reason != rows[0].Reason || got.HitAt != rows[0].HitAt {
		t.Fatalf("geometry walker diverged from LabelSet: got %+v want %+v", got, rows[0])
	}
	if got.Outcome != OutcomeUpFirst {
		t.Fatalf("expected finer UP, got %+v", got)
	}
}

func TestCandidateGeometry1_NoWinnerAndNoCatBoost(t *testing.T) {
	t.Parallel()
	step15, _ := data.IntervalDurationMs("15m")
	step1h, _ := data.IntervalDurationMs("1h")
	base := int64(1_609_459_200_000)
	const n15 = 90
	primary := make([]CanonicalClosedBar, n15)
	for i := 0; i < n15; i++ {
		primary[i] = CanonicalClosedBar{
			OpenTime: base + int64(i)*step15,
			Open:     100, High: 101, Low: 99, Close: 100, Volume: 1,
		}
	}
	n1h := (n15 + 3) / 4
	h1 := make([]CanonicalClosedBar, n1h)
	for i := 0; i < n1h; i++ {
		h1[i] = CanonicalClosedBar{
			OpenTime: base + int64(i)*step1h,
			Open:     100, High: 101, Low: 99, Close: 100, Volume: 1,
		}
	}
	tape := make([]FeatureRow2, 3)
	for i := 0; i < 3; i++ {
		var v FeatureVector2
		v[spec2Col(FeatureRSXValue)] = 50
		if i == 1 {
			v[spec2Col(FeatureRSX50CrossUpPresent)] = 1
			v[spec2Col(FeatureRSX50CrossUpAge)] = 0
		}
		if i == 2 {
			v[spec2Col(FeatureRSX50CrossUpPresent)] = 1
			v[spec2Col(FeatureRSX50CrossUpAge)] = 1
		}
		tape[i] = FeatureRow2{At: primary[10+i].OpenTime, Ready: IsReady, Values: v}
	}
	fm := testTapeHeader().Market
	fm.Timeframe = "1m"
	rep, err := RunCandidateGeometry1(CandidateGeometryInput{
		PrimaryTF: "15m", FinerTF: "1m", HTF1hTF: "1h",
		FinerMarket: fm, Primary: primary, HTF1h: h1, Tape: tape,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Cells) != 32 {
		t.Fatalf("cells %d want 32", len(rep.Cells))
	}
	txt := FormatCandidateGeometry1(rep)
	if !strings.Contains(txt, "NO TARGET SELECTED") {
		t.Fatal("report must refuse a winner")
	}
	if strings.Contains(strings.ToLower(txt), "winner") || strings.Contains(txt, "argmax") {
		t.Fatal("must not select a geometry")
	}
	var n50 int
	for _, p := range rep.Populations {
		if p.Family == GeomFamily50Cross && p.Side == GeomSideLong {
			n50 = p.N
		}
	}
	if n50 != 1 {
		t.Fatalf("50-cross LONG ignitions=%d (age=1 memory must be excluded)", n50)
	}
	rep2, err := RunCandidateGeometry1(CandidateGeometryInput{
		PrimaryTF: "15m", FinerTF: "1m", HTF1hTF: "1h",
		FinerMarket: fm, Primary: primary, HTF1h: h1, Tape: tape,
	})
	if err != nil {
		t.Fatal(err)
	}
	if FormatCandidateGeometry1(rep) != FormatCandidateGeometry1(rep2) {
		t.Fatal("synthetic determinism mismatch")
	}
}
