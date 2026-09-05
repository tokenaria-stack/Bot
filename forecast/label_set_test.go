package forecast

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"trading_bot/data"
	"trading_bot/indicators"
)

func labelStep(t *testing.T) int64 {
	t.Helper()
	step, err := data.IntervalDurationMs("15m")
	if err != nil {
		t.Fatal(err)
	}
	return step
}

func labelAt(t *testing.T, i int) int64 {
	t.Helper()
	base := int64(1_699_999_200_000)
	got, err := data.CurrentBarOpen(base, "15m")
	if err != nil {
		t.Fatal(err)
	}
	if got != base {
		t.Fatalf("fixture base %d is not a 15m open (floor %d)", base, got)
	}
	return base + int64(i)*labelStep(t)
}

func ohlc(i int, o, h, l, c float64) CanonicalClosedBar {
	return CanonicalClosedBar{
		OpenTime: 0, // filled by stamp
		Open:     o,
		High:     h,
		Low:      l,
		Close:    c,
		Volume:   1,
	}
}

func stampBars(t *testing.T, bars []CanonicalClosedBar) []CanonicalClosedBar {
	t.Helper()
	out := append([]CanonicalClosedBar(nil), bars...)
	for i := range out {
		out[i].OpenTime = labelAt(t, i)
	}
	return out
}

func testLabelSpec(t *testing.T, horizon int, up, down float64, period int) TargetSpec {
	t.Helper()
	spec, err := ResolveTargetSpec("t", TargetSpecDraft{
		HorizonBars:      horizon,
		UpperATRMultiple: up,
		LowerATRMultiple: down,
		ATRPeriod:        period,
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func writeLabelTape(t *testing.T, dir string, bars []CanonicalClosedBar, readyAt map[int64]bool) (string, TapeFooter) {
	t.Helper()
	path := filepath.Join(dir, "t.featuretape")
	h := testTapeHeader()
	w, err := CreateTapeWriter(path, h)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bars {
		ready := true
		if readyAt != nil {
			if v, ok := readyAt[b.OpenTime]; ok {
				ready = v
			}
		}
		if ready {
			if err := w.WriteRow(b.OpenTime, IsReady, []float64{50, 49, 0, 1}); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := w.WriteRow(b.OpenTime, NotReady, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	src := SourceRangeDigest(h.Market, bars)
	if err := w.Finish(src); err != nil {
		t.Fatal(err)
	}
	_, _, ft, err := ReadTape(path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return path, ft
}

func boundBars(n int) []CanonicalClosedBar {
	const close, wing = 100.0, 1.0
	bars := make([]CanonicalClosedBar, n)
	for i := 0; i < n; i++ {
		bars[i] = ohlc(i, close, close+wing, close-wing, close)
	}
	return bars
}

func atrThrough(t *testing.T, spec TargetSpec, bars []CanonicalClosedBar, idx int) float64 {
	t.Helper()
	high := make([]float64, idx+1)
	low := make([]float64, idx+1)
	cl := make([]float64, idx+1)
	for i := 0; i <= idx; i++ {
		high[i], low[i], cl[i] = bars[i].High, bars[i].Low, bars[i].Close
	}
	s, err := indicators.ATRSeries(spec.ATR, high, low, cl)
	if err != nil {
		t.Fatal(err)
	}
	return s[idx]
}

func genLabels(t *testing.T, spec TargetSpec, tapeBars, primary []CanonicalClosedBar, readyAt map[int64]bool) (LabelHeader, []LabelRow, LabelFooter) {
	t.Helper()
	dir := t.TempDir()
	tapePath, _ := writeLabelTape(t, dir, tapeBars, readyAt)
	out := filepath.Join(dir, "t.labelset")
	if err := GenerateLabelSet(out, tapePath, spec, primary, nil); err != nil {
		t.Fatal(err)
	}
	h, rows, f, err := ReadLabelSet(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	return h, rows, f
}

func TestLabelSet_UpFirst(t *testing.T) {
	spec := testLabelSpec(t, 5, 1, 1, 2)
	primary := stampBars(t, boundBars(10))
	c := 4
	atr := atrThrough(t, spec, primary, c)
	upper := primary[c].Close + spec.UpperATRMultiple*atr
	primary[c+3].High = upper
	primary[c+3].Low = primary[c].Close
	tape := primary[c : c+1]
	_, rows, _ := genLabels(t, spec, tape, primary, nil)
	if len(rows) != 1 || rows[0].Outcome != OutcomeUpFirst || rows[0].HitAt != primary[c+3].OpenTime || rows[0].Reason != ReasonNone {
		t.Fatalf("got %+v", rows)
	}
}

func TestLabelSet_DownFirst(t *testing.T) {
	spec := testLabelSpec(t, 5, 1, 1, 2)
	primary := stampBars(t, boundBars(10))
	c := 4
	atr := atrThrough(t, spec, primary, c)
	lower := primary[c].Close - spec.LowerATRMultiple*atr
	primary[c+2].Low = lower
	primary[c+2].High = primary[c].Close
	tape := primary[c : c+1]
	_, rows, _ := genLabels(t, spec, tape, primary, nil)
	if len(rows) != 1 || rows[0].Outcome != OutcomeDownFirst || rows[0].HitAt != primary[c+2].OpenTime || rows[0].Reason != ReasonNone {
		t.Fatalf("got %+v", rows)
	}
}

func TestLabelSet_Timeout(t *testing.T) {
	spec := testLabelSpec(t, 4, 2, 2, 2)
	primary := stampBars(t, boundBars(12))
	c := 4
	tape := primary[c : c+1]
	_, rows, _ := genLabels(t, spec, tape, primary, nil)
	if rows[0].Outcome != OutcomeTimeout || rows[0].HitAt != 0 || rows[0].Reason != ReasonNone {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestLabelSet_PrimaryDualHit(t *testing.T) {
	spec := testLabelSpec(t, 5, 1, 1, 2)
	primary := stampBars(t, boundBars(10))
	c := 4
	atr := atrThrough(t, spec, primary, c)
	upper := primary[c].Close + spec.UpperATRMultiple*atr
	lower := primary[c].Close - spec.LowerATRMultiple*atr
	primary[c+1].High = upper
	primary[c+1].Low = lower
	tape := primary[c : c+1]
	_, rows, _ := genLabels(t, spec, tape, primary, nil)
	if rows[0].Outcome != OutcomeAmbiguous || rows[0].Reason != ReasonDualHit || rows[0].HitAt != primary[c+1].OpenTime {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestLabelSet_ATRCausalityNoFutureLeak(t *testing.T) {
	spec := testLabelSpec(t, 5, 2, 2, 2)
	primary := stampBars(t, boundBars(12))
	c := 5
	wantATR := atrThrough(t, spec, primary, c)
	fullHigh := make([]float64, c+6)
	fullLow := make([]float64, c+6)
	fullCl := make([]float64, c+6)
	for i := 0; i <= c+5; i++ {
		fullHigh[i], fullLow[i], fullCl[i] = primary[i].High, primary[i].Low, primary[i].Close
	}
	full, err := indicators.ATRSeries(spec.ATR, fullHigh, fullLow, fullCl)
	if err != nil {
		t.Fatal(err)
	}
	if math.Float64bits(full[c]) != math.Float64bits(wantATR) {
		t.Fatal("ATR[t] must equal prefix-through-t and ignore later bars")
	}
	tape := primary[c : c+1]
	_, before, _ := genLabels(t, spec, tape, primary, nil)
	mut := append([]CanonicalClosedBar(nil), primary...)
	// Change later Close (IIR input) without moving High/Low, so first-passage is unchanged.
	mut[c+1].Close = 100.25
	_, after, _ := genLabels(t, spec, tape, mut, nil)
	if before[0] != after[0] {
		t.Fatalf("future ATR-changing mutation without a hit must not change the label: before=%+v after=%+v", before[0], after[0])
	}
}

func TestLabelSet_CandidateBarNotScanned(t *testing.T) {
	spec := testLabelSpec(t, 4, 1, 1, 2)
	primary := stampBars(t, boundBars(12))
	c := 4
	atr := atrThrough(t, spec, primary, c)
	upper := primary[c].Close + spec.UpperATRMultiple*atr
	lower := primary[c].Close - spec.LowerATRMultiple*atr
	primary[c].High = upper + 10
	primary[c].Low = lower - 10
	tape := primary[c : c+1]
	_, rows, _ := genLabels(t, spec, tape, primary, nil)
	if rows[0].Outcome != OutcomeTimeout {
		t.Fatalf("candidate High/Low must not count, got %+v", rows[0])
	}
}

func TestLabelSet_ATRZero(t *testing.T) {
	spec := testLabelSpec(t, 3, 1, 1, 2)
	raw := make([]CanonicalClosedBar, 8)
	for i := range raw {
		raw[i] = ohlc(i, 100, 100, 100, 100)
	}
	primary := stampBars(t, raw)
	tape := primary[3:4]
	_, rows, _ := genLabels(t, spec, tape, primary, nil)
	if rows[0].Outcome != OutcomeAmbiguous || rows[0].Reason != ReasonATRZero || rows[0].HitAt != 0 {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestLabelSet_TruncatedButEarlyHit(t *testing.T) {
	spec := testLabelSpec(t, 8, 1, 1, 2)
	primary := stampBars(t, boundBars(7))
	c := 4
	atr := atrThrough(t, spec, primary, c)
	upper := primary[c].Close + spec.UpperATRMultiple*atr
	primary[c+1].High = upper
	primary[c+1].Low = primary[c].Close
	tape := primary[c : c+1]
	_, rows, _ := genLabels(t, spec, tape, primary, nil)
	if rows[0].Outcome != OutcomeUpFirst || rows[0].HitAt != primary[c+1].OpenTime {
		t.Fatalf("early hit on short tail must remain UP_FIRST, got %+v", rows[0])
	}
}

func TestLabelSet_TruncatedWithoutHit(t *testing.T) {
	spec := testLabelSpec(t, 8, 2, 2, 2)
	primary := stampBars(t, boundBars(7))
	c := 4
	tape := primary[c : c+1]
	_, rows, _ := genLabels(t, spec, tape, primary, nil)
	if rows[0].Outcome != OutcomeAmbiguous || rows[0].Reason != ReasonTruncatedHorizon || rows[0].HitAt != 0 {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestLabelSet_ATRHistoryGapInPrefixRefuses(t *testing.T) {
	spec := testLabelSpec(t, 4, 2, 2, 2)
	full := stampBars(t, boundBars(10))
	c := 4
	gapped := append(append([]CanonicalClosedBar{}, full[:2]...), full[3:]...)
	tape := []CanonicalClosedBar{full[c]}
	dir := t.TempDir()
	tapePath, _ := writeLabelTape(t, dir, tape, nil)
	err := GenerateLabelSet(filepath.Join(dir, "t.labelset"), tapePath, spec, gapped, nil)
	if err == nil || !strings.Contains(err.Error(), "ATR history has a primary gap") {
		t.Fatalf("prefix gap must refuse generation, got %v", err)
	}
}

func TestLabelSet_ATRHistoryGapBetweenCandidatesRefuses(t *testing.T) {
	spec := testLabelSpec(t, 3, 2, 2, 2)
	full := stampBars(t, boundBars(12))
	gapped := append(append([]CanonicalClosedBar{}, full[:6]...), full[7:]...)
	tape := []CanonicalClosedBar{full[4], full[8]}
	dir := t.TempDir()
	tapePath, _ := writeLabelTape(t, dir, tape, nil)
	err := GenerateLabelSet(filepath.Join(dir, "t.labelset"), tapePath, spec, gapped, nil)
	if err == nil || !strings.Contains(err.Error(), "ATR history has a primary gap") {
		t.Fatalf("inter-candidate gap must refuse generation, got %v", err)
	}
}

func TestLabelSet_PrimaryGapBeforeOutcome(t *testing.T) {
	spec := testLabelSpec(t, 5, 2, 2, 2)
	full := stampBars(t, boundBars(10))
	c := 4
	// drop the first future bar (c+1)
	gapped := append(append([]CanonicalClosedBar{}, full[:c+1]...), full[c+2:]...)
	tape := gapped[c : c+1]
	_, rows, _ := genLabels(t, spec, tape, gapped, nil)
	if rows[0].Outcome != OutcomeAmbiguous || rows[0].Reason != ReasonPrimaryGap || rows[0].HitAt != 0 {
		t.Fatalf("must not skip a missing primary interval, got %+v", rows[0])
	}
}

func TestLabelSet_GapAfterDefinitiveHit(t *testing.T) {
	spec := testLabelSpec(t, 5, 1, 1, 2)
	full := stampBars(t, boundBars(12))
	c := 4
	atr := atrThrough(t, spec, full, c)
	upper := full[c].Close + spec.UpperATRMultiple*atr
	full[c+1].High = upper
	full[c+1].Low = full[c].Close
	gapped := append(append([]CanonicalClosedBar{}, full[:c+3]...), full[c+5:]...)
	tape := gapped[c : c+1]
	_, rows, _ := genLabels(t, spec, tape, gapped, nil)
	if rows[0].Outcome != OutcomeUpFirst || rows[0].HitAt != full[c+1].OpenTime {
		t.Fatalf("later gap must not unwind an earlier hit, got %+v", rows[0])
	}
}

func TestLabelSet_HorizonNthBarIncludedAndNextExcluded(t *testing.T) {
	spec := testLabelSpec(t, 3, 1, 1, 2)
	primary := stampBars(t, boundBars(12))
	c := 4
	atr := atrThrough(t, spec, primary, c)
	upper := primary[c].Close + spec.UpperATRMultiple*atr
	tape := primary[c : c+1]

	onH := append([]CanonicalClosedBar{}, primary...)
	onH[c+3].High = upper
	onH[c+3].Low = primary[c].Close
	_, rowsH, ftH := genLabels(t, spec, tape, onH, nil)
	if rowsH[0].Outcome != OutcomeUpFirst || rowsH[0].HitAt != onH[c+3].OpenTime {
		t.Fatalf("H-th subsequent bar must be visible, got %+v", rowsH[0])
	}
	wantConsumed := onH[:consumedEndIndex(c, spec.HorizonBars, len(onH))+1]
	if len(wantConsumed) != c+spec.HorizonBars+1 {
		t.Fatalf("consumed len %d want %d", len(wantConsumed), c+spec.HorizonBars+1)
	}
	if ftH.LabelSourceRangeDigest != LabelSourceRangeDigest(testTapeHeader().Market, wantConsumed) {
		t.Fatal("consumed range must be [0 .. i+H] inclusive")
	}

	beyond := append([]CanonicalClosedBar{}, primary...)
	beyond[c+4].High = upper
	beyond[c+4].Low = primary[c].Close
	_, rowsB, ftB := genLabels(t, spec, tape, beyond, nil)
	if rowsB[0].Outcome != OutcomeTimeout || rowsB[0].HitAt != 0 {
		t.Fatalf("H+1 subsequent bar must be invisible, got %+v", rowsB[0])
	}
	wantTimeout := primary[:consumedEndIndex(c, spec.HorizonBars, len(primary))+1]
	if ftB.LabelSourceRangeDigest != LabelSourceRangeDigest(testTapeHeader().Market, wantTimeout) {
		t.Fatal("H+1 bar is outside consumed source")
	}
}

func TestLabelSet_ReadyFalseStillLabeled(t *testing.T) {
	spec := testLabelSpec(t, 4, 2, 2, 2)
	primary := stampBars(t, boundBars(10))
	c := 4
	tape := primary[c : c+1]
	ready := map[int64]bool{primary[c].OpenTime: false}
	_, rows, _ := genLabels(t, spec, tape, primary, ready)
	if len(rows) != 1 || rows[0].At != primary[c].OpenTime || rows[0].Outcome != OutcomeTimeout {
		t.Fatalf("Ready=false must still produce one label row, got %+v", rows)
	}
}

func TestLabelSet_MissingCandidateRefuses(t *testing.T) {
	spec := testLabelSpec(t, 3, 1, 1, 2)
	primary := stampBars(t, boundBars(8))
	tapeBars := primary[3:5]
	dir := t.TempDir()
	tapePath, _ := writeLabelTape(t, dir, tapeBars, nil)
	src := append([]CanonicalClosedBar{}, primary[:4]...)
	src = append(src, primary[5:]...)
	err := GenerateLabelSet(filepath.Join(dir, "t.labelset"), tapePath, spec, src, nil)
	if err == nil || !strings.Contains(err.Error(), "missing from primary") {
		t.Fatalf("expected missing-candidate refuse, got %v", err)
	}
}

func TestLabelSet_OneToOneJoin(t *testing.T) {
	spec := testLabelSpec(t, 3, 2, 2, 2)
	primary := stampBars(t, boundBars(12))
	tape := primary[4:8]
	hdr, rows, ft := genLabels(t, spec, tape, primary, nil)
	if ft.RowCount != len(tape) || len(rows) != len(tape) {
		t.Fatalf("rowcount tape=%d labels=%d footer=%d", len(tape), len(rows), ft.RowCount)
	}
	for i := range tape {
		if rows[i].At != tape[i].OpenTime {
			t.Fatalf("At sequence diverged at %d", i)
		}
	}
	if hdr.LabelLogicVersion != LabelLogicFirstPassagePrimaryV1 {
		t.Fatalf("logic %s", hdr.LabelLogicVersion)
	}
}

func TestLabelSet_IdentityRefusals(t *testing.T) {
	spec := testLabelSpec(t, 3, 1, 1, 2)
	primary := stampBars(t, boundBars(8))
	tape := primary[3:4]
	dir := t.TempDir()
	tapePath, tf := writeLabelTape(t, dir, tape, nil)
	wrong := testTapeHeader().Market
	wrong.Contract = "SPOT"
	err := GenerateLabelSet(filepath.Join(dir, "a.labelset"), tapePath, spec, primary, &LabelExpect{Market: &wrong})
	if err == nil {
		t.Fatal("expected MarketKey mismatch")
	}
	tid, _ := spec.Identity()
	bogus := tid.Digest
	bogus[0] ^= 0xff
	err = GenerateLabelSet(filepath.Join(dir, "b.labelset"), tapePath, spec, primary, &LabelExpect{Target: &bogus})
	if err == nil {
		t.Fatal("expected TargetDigest mismatch")
	}
	badContent := tf.ContentDigest
	badContent[1] ^= 0xff
	err = GenerateLabelSet(filepath.Join(dir, "c.labelset"), tapePath, spec, primary, &LabelExpect{TapeContent: &badContent})
	if err == nil {
		t.Fatal("expected FeatureTape ContentDigest mismatch")
	}
	malformed := filepath.Join(dir, "bad.featuretape")
	if err := os.WriteFile(malformed, []byte("{not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := GenerateLabelSet(filepath.Join(dir, "d.labelset"), malformed, spec, primary, nil); err == nil {
		t.Fatal("expected malformed tape refuse")
	}
}

func TestLabelSet_TargetIdentity(t *testing.T) {
	base := testLabelSpec(t, 5, 1.5, 1.0, 14)
	a, _ := base.Identity()
	h, _ := ResolveTargetSpec("t", TargetSpecDraft{HorizonBars: 6, UpperATRMultiple: 1.5, LowerATRMultiple: 1.0, ATRPeriod: 14}, "labels:v1")
	hid, _ := h.Identity()
	if a.Digest == hid.Digest {
		t.Fatal("Horizon must change TargetDigest")
	}
	m, _ := ResolveTargetSpec("t", TargetSpecDraft{HorizonBars: 5, UpperATRMultiple: 2, LowerATRMultiple: 1.0, ATRPeriod: 14}, "labels:v1")
	mid, _ := m.Identity()
	if a.Digest == mid.Digest {
		t.Fatal("ATR multiple must change TargetDigest")
	}
	p, _ := ResolveTargetSpec("t", TargetSpecDraft{HorizonBars: 5, UpperATRMultiple: 1.5, LowerATRMultiple: 1.0, ATRPeriod: 21}, "labels:v1")
	pid, _ := p.Identity()
	if a.Digest == pid.Digest {
		t.Fatal("ATRSpec period must change TargetDigest")
	}
	err := GenerateLabelSet(filepath.Join(t.TempDir(), "x.labelset"), filepath.Join(t.TempDir(), "no.tape"), TargetSpec{
		Family:           TargetFamilyATRFirstPassage,
		HorizonBars:      5,
		UpperATRMultiple: 1,
		LowerATRMultiple: 1,
		ATR:              indicators.CanonicalATRSpec(),
		DualHit:          DualHitResolveFinerHistory,
		Logic:            "labels:v1",
	}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "DualHitPolicy") {
		t.Fatalf("1A must refuse finer dual-hit policy, got %v", err)
	}
}

func TestLabelSet_SourceDigestConsumedRange(t *testing.T) {
	spec := testLabelSpec(t, 3, 2, 2, 2)
	primary := stampBars(t, boundBars(16))
	tape := primary[4:5]
	_, _, a := genLabels(t, spec, tape, primary, nil)
	trim := primary[:consumedEndIndex(4, spec.HorizonBars, len(primary))+1]
	want := LabelSourceRangeDigest(testTapeHeader().Market, trim)
	if a.LabelSourceRangeDigest != want {
		t.Fatal("LabelSourceRangeDigest must cover prefix+candidates+needed H tail only")
	}
	extra := append(append([]CanonicalClosedBar{}, primary...), ohlc(99, 1, 1, 1, 1))
	extra[len(extra)-1].OpenTime = labelAt(t, 99)
	_, _, b := genLabels(t, spec, tape, extra, nil)
	if a.LabelSourceRangeDigest != b.LabelSourceRangeDigest {
		t.Fatal("unused caller bars after used end must not change LabelSourceRangeDigest")
	}
	mut := append([]CanonicalClosedBar{}, primary...)
	mut[0].Close += 0.25
	_, _, c := genLabels(t, spec, tape, mut, nil)
	if c.LabelSourceRangeDigest == a.LabelSourceRangeDigest {
		t.Fatal("changing a consumed OHLCV field must change LabelSourceRangeDigest")
	}
}

func TestLabelSet_ContentTamperRefused(t *testing.T) {
	spec := testLabelSpec(t, 4, 2, 2, 2)
	primary := stampBars(t, boundBars(10))
	tape := primary[4:5]
	dir := t.TempDir()
	tapePath, _ := writeLabelTape(t, dir, tape, nil)
	path := filepath.Join(dir, "t.labelset")
	if err := GenerateLabelSet(path, tapePath, spec, primary, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(raw), `"TIMEOUT"`, `"UP_FIRST"`, 1)
	if tampered == string(raw) {
		t.Fatal("expected TIMEOUT in fixture")
	}
	if err := os.WriteFile(path, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ReadLabelSet(path, nil); err == nil {
		t.Fatal("tampered outcome must refuse")
	}
}

func TestLabelSet_RoundTrip(t *testing.T) {
	spec := testLabelSpec(t, 5, 1, 1, 2)
	primary := stampBars(t, boundBars(14))
	c := 4
	atr := atrThrough(t, spec, primary, c)
	primary[c+1].High = primary[c].Close + spec.UpperATRMultiple*atr
	tape := primary[3:7]
	ready := map[int64]bool{primary[5].OpenTime: false}
	dir := t.TempDir()
	tapePath, _ := writeLabelTape(t, dir, tape, ready)
	path := filepath.Join(dir, "t.labelset")
	hdr, rows, src, err := BuildLabelSet(tapePath, spec, primary, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := GenerateLabelSet(path, tapePath, spec, primary, nil); err != nil {
		t.Fatal(err)
	}
	gh, grows, gf, err := ReadLabelSet(path, &LabelExpect{
		Market:      &hdr.Market,
		Target:      &hdr.TargetDigest,
		TapePlan:    &hdr.FeatureTapePlanDigest,
		TapeSource:  &hdr.FeatureTapeSourceRangeDigest,
		TapeContent: &hdr.FeatureTapeContentDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gh != hdr {
		t.Fatalf("header %+v vs %+v", gh, hdr)
	}
	if len(grows) != len(rows) {
		t.Fatal("row count")
	}
	for i := range rows {
		if grows[i] != rows[i] {
			t.Fatalf("row %d %+v vs %+v", i, grows[i], rows[i])
		}
	}
	if gf.LabelSourceRangeDigest != src || gf.RowCount != len(rows) {
		t.Fatalf("footer %+v", gf)
	}
}

func TestLabelSet_RefuseExistingPath(t *testing.T) {
	spec := testLabelSpec(t, 3, 2, 2, 2)
	primary := stampBars(t, boundBars(8))
	tape := primary[3:4]
	dir := t.TempDir()
	tapePath, _ := writeLabelTape(t, dir, tape, nil)
	path := filepath.Join(dir, "t.labelset")
	if err := GenerateLabelSet(path, tapePath, spec, primary, nil); err != nil {
		t.Fatal(err)
	}
	if err := GenerateLabelSet(path, tapePath, spec, primary, nil); err == nil {
		t.Fatal("existing path must refuse")
	}
}

func TestLabelSet_JSONReasonPresent(t *testing.T) {
	spec := testLabelSpec(t, 4, 2, 2, 2)
	primary := stampBars(t, boundBars(10))
	tape := primary[4:5]
	dir := t.TempDir()
	tapePath, _ := writeLabelTape(t, dir, tape, nil)
	path := filepath.Join(dir, "t.labelset")
	if err := GenerateLabelSet(path, tapePath, spec, primary, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var row struct {
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		if row.Kind == "row" {
			if row.Reason != string(ReasonNone) {
				t.Fatalf("reason %q", row.Reason)
			}
			return
		}
	}
	t.Fatal("no row")
}
