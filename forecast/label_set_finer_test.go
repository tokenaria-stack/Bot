package forecast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"trading_bot/data"
	"trading_bot/indicators"
)

func testResolveSpec(t *testing.T, horizon int, up, down float64, period int, finerTF string) TargetSpec {
	t.Helper()
	spec, err := ResolveTargetSpec("t", TargetSpecDraft{
		HorizonBars:      horizon,
		UpperATRMultiple: up,
		LowerATRMultiple: down,
		ATRPeriod:        period,
		DualHit:          DualHitResolveFinerHistory,
		FinerTimeframe:   finerTF,
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func finerMarketOf(primary MarketKey, tf string) MarketKey {
	k := primary
	k.Timeframe = tf
	return k
}

func minuteBars(t *testing.T, parentOpen int64, n int, close, high, low float64) []CanonicalClosedBar {
	t.Helper()
	step, err := data.IntervalDurationMs("1m")
	if err != nil {
		t.Fatal(err)
	}
	out := make([]CanonicalClosedBar, n)
	for i := 0; i < n; i++ {
		ot := parentOpen + int64(i)*step
		got, err := data.CurrentBarOpen(ot, "1m")
		if err != nil || got != ot {
			t.Fatalf("1m open %d", ot)
		}
		out[i] = CanonicalClosedBar{OpenTime: ot, Open: close, High: high, Low: low, Close: close, Volume: 1}
	}
	return out
}

func genFinerLabels(t *testing.T, spec TargetSpec, tapeBars, primary, finer []CanonicalClosedBar) (LabelHeader, []LabelRow, LabelFooter) {
	t.Helper()
	dir := t.TempDir()
	tapePath, _ := writeLabelTape(t, dir, tapeBars, nil)
	out := filepath.Join(dir, "t.labelset")
	fm := finerMarketOf(testTapeHeader().Market, spec.FinerTimeframe)
	if err := GenerateLabelSetWithFiner(out, tapePath, spec, primary, fm, finer, nil); err != nil {
		t.Fatal(err)
	}
	h, rows, f, err := ReadLabelSet(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	return h, rows, f
}

func dualHitFixture(t *testing.T, spec TargetSpec) (primary []CanonicalClosedBar, c int, parent int, upper, lower float64) {
	t.Helper()
	primary = stampBars(t, boundBars(10))
	c = 4
	atr := atrThrough(t, spec, primary, c)
	upper = primary[c].Close + spec.UpperATRMultiple*atr
	lower = primary[c].Close - spec.LowerATRMultiple*atr
	parent = c + 1
	primary[parent].High = upper
	primary[parent].Low = lower
	return primary, c, parent, upper, lower
}

func TestTargetSpec_ExcludeDigestUnchangedFrom1APayload(t *testing.T) {
	spec, err := ResolveTargetSpec("n1", TargetSpecDraft{HorizonBars: 24, UpperATRMultiple: 1.5, LowerATRMultiple: 1.0}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	got, err := spec.Identity()
	if err != nil {
		t.Fatal(err)
	}
	old, err := computeDigest(struct {
		Family           TargetFamily
		HorizonBars      int
		UpperATRMultiple float64
		LowerATRMultiple float64
		ATR              indicators.ATRSpec
		DualHit          DualHitPolicy
		Logic            LogicVersion
	}{spec.Family, spec.HorizonBars, spec.UpperATRMultiple, spec.LowerATRMultiple, spec.ATR, spec.DualHit, spec.Logic})
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest != old {
		t.Fatal("exclude_ambiguous TargetDigest must match pre-FinerTimeframe payload bytes")
	}
}

func TestTargetSpec_FinerTimeframeIdentityAndValidation(t *testing.T) {
	m, err := ResolveTargetSpec("t", TargetSpecDraft{
		HorizonBars: 5, UpperATRMultiple: 1, LowerATRMultiple: 1, DualHit: DualHitResolveFinerHistory, FinerTimeframe: "1m",
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	s, err := ResolveTargetSpec("t", TargetSpecDraft{
		HorizonBars: 5, UpperATRMultiple: 1, LowerATRMultiple: 1, DualHit: DualHitResolveFinerHistory, FinerTimeframe: "1s",
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	im, _ := m.Identity()
	is, _ := s.Identity()
	if im.Digest == is.Digest {
		t.Fatal("1m vs 1s FinerTimeframe must change TargetDigest")
	}
	if _, err := ResolveTargetSpec("t", TargetSpecDraft{
		HorizonBars: 5, UpperATRMultiple: 1, LowerATRMultiple: 1, DualHit: DualHitResolveFinerHistory,
	}, "labels:v1"); err == nil {
		t.Fatal("resolve without FinerTimeframe must refuse")
	}
	if _, err := ResolveTargetSpec("t", TargetSpecDraft{
		HorizonBars: 5, UpperATRMultiple: 1, LowerATRMultiple: 1, FinerTimeframe: "1m",
	}, "labels:v1"); err == nil {
		t.Fatal("exclude_ambiguous with FinerTimeframe must refuse")
	}
}

func TestLabelSet_FinerTiles15m1mAndRefusesNonTiling(t *testing.T) {
	if err := finerTilesPrimary("15m", "1m", labelAt(t, 0)); err != nil {
		t.Fatal(err)
	}
	if err := finerTilesPrimary("1h", "45m", labelAt(t, 0)); err == nil {
		t.Fatal("1h/45m must not tile")
	}
}

func TestLabelSet_FinerSameFamilyRefuse(t *testing.T) {
	spec := testResolveSpec(t, 3, 1, 1, 2, "1m")
	primary := stampBars(t, boundBars(8))
	tape := primary[3:4]
	dir := t.TempDir()
	tapePath, _ := writeLabelTape(t, dir, tape, nil)
	spot := testTapeHeader().Market
	spot.Contract = "SPOT"
	spot.Timeframe = "1m"
	err := GenerateLabelSetWithFiner(filepath.Join(dir, "t.labelset"), tapePath, spec, primary, spot, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "SameFamily") {
		t.Fatalf("spot vs perp must refuse, got %v", err)
	}
	other := testTapeHeader().Market
	other.Venue = "OTHER"
	other.Timeframe = "1m"
	err = GenerateLabelSetWithFiner(filepath.Join(dir, "u.labelset"), tapePath, spec, primary, other, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "SameFamily") {
		t.Fatalf("wrong venue must refuse, got %v", err)
	}
	inst := testTapeHeader().Market
	inst.Instrument = "ETHUSDT"
	inst.Timeframe = "1m"
	err = GenerateLabelSetWithFiner(filepath.Join(dir, "i.labelset"), tapePath, spec, primary, inst, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "SameFamily") {
		t.Fatalf("wrong instrument must refuse, got %v", err)
	}
	tf := testTapeHeader().Market
	tf.Timeframe = "1s"
	err = GenerateLabelSetWithFiner(filepath.Join(dir, "s.labelset"), tapePath, spec, primary, tf, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "FinerTimeframe") {
		t.Fatalf("header TF must match TargetSpec.FinerTimeframe, got %v", err)
	}
}

func TestLabelSet_FinerUpThenDown(t *testing.T) {
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	primary, c, parent, upper, lower := dualHitFixture(t, spec)
	mins := minuteBars(t, primary[parent].OpenTime, 15, 100, 100.2, 99.8)
	mins[1].High = upper
	mins[1].Low = 100
	mins[2].Low = lower
	mins[2].High = 100
	_, rows, _ := genFinerLabels(t, spec, primary[c:c+1], primary, mins)
	if rows[0].Outcome != OutcomeUpFirst || rows[0].Reason != ReasonNone || rows[0].HitAt != primary[parent].OpenTime {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestLabelSet_FinerDownFirst(t *testing.T) {
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	primary, c, parent, upper, lower := dualHitFixture(t, spec)
	mins := minuteBars(t, primary[parent].OpenTime, 15, 100, 100.2, 99.8)
	mins[1].Low = lower
	mins[1].High = 100
	mins[2].High = upper
	mins[2].Low = 100
	_, rows, _ := genFinerLabels(t, spec, primary[c:c+1], primary, mins)
	if rows[0].Outcome != OutcomeDownFirst || rows[0].HitAt != primary[parent].OpenTime {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestLabelSet_FinerMissingFromParentOpen(t *testing.T) {
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	primary, c, parent, upper, _ := dualHitFixture(t, spec)
	mins := minuteBars(t, primary[parent].OpenTime, 15, 100, 100.2, 99.8)
	mins = mins[1:] // start late
	mins[0].High = upper
	_, rows, _ := genFinerLabels(t, spec, primary[c:c+1], primary, mins)
	if rows[0].Outcome != OutcomeAmbiguous || rows[0].Reason != ReasonFinerMissing || rows[0].HitAt != primary[parent].OpenTime {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestLabelSet_FinerGapBeforeResult(t *testing.T) {
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	primary, c, parent, _, _ := dualHitFixture(t, spec)
	mins := minuteBars(t, primary[parent].OpenTime, 15, 100, 100.2, 99.8)
	gapped := append(append([]CanonicalClosedBar{}, mins[:1]...), mins[2:]...)
	_, rows, _ := genFinerLabels(t, spec, primary[c:c+1], primary, gapped)
	if rows[0].Reason != ReasonFinerGap {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestLabelSet_FinerHitBeforeLaterGap(t *testing.T) {
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	primary, c, parent, upper, _ := dualHitFixture(t, spec)
	mins := minuteBars(t, primary[parent].OpenTime, 15, 100, 100.2, 99.8)
	mins[0].High = upper
	mins[0].Low = 100
	gapped := append(append([]CanonicalClosedBar{}, mins[:2]...), mins[4:]...)
	_, rows, _ := genFinerLabels(t, spec, primary[c:c+1], primary, gapped)
	if rows[0].Outcome != OutcomeUpFirst {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestLabelSet_FinerDualHit(t *testing.T) {
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	primary, c, parent, upper, lower := dualHitFixture(t, spec)
	mins := minuteBars(t, primary[parent].OpenTime, 15, 100, 100.2, 99.8)
	mins[0].High = upper
	mins[0].Low = lower
	_, rows, _ := genFinerLabels(t, spec, primary[c:c+1], primary, mins)
	if rows[0].Reason != ReasonFinerDualHit {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestLabelSet_FinerInconsistent(t *testing.T) {
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	primary, c, parent, _, _ := dualHitFixture(t, spec)
	mins := minuteBars(t, primary[parent].OpenTime, 15, 100, 100.2, 99.8)
	_, rows, _ := genFinerLabels(t, spec, primary[c:c+1], primary, mins)
	if rows[0].Reason != ReasonFinerInconsistent {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestLabelSet_NoPrimaryDualHitZeroFinerWindows(t *testing.T) {
	spec := testResolveSpec(t, 4, 2, 2, 2, "1m")
	primary := stampBars(t, boundBars(12))
	c := 4
	mins := minuteBars(t, primary[c+1].OpenTime, 15, 100, 101, 99)
	hdr, rows, ft := genFinerLabels(t, spec, primary[c:c+1], primary, mins)
	if rows[0].Outcome != OutcomeTimeout {
		t.Fatalf("got %+v", rows[0])
	}
	if ft.FinerWindowCount != 0 {
		t.Fatalf("windows %d", ft.FinerWindowCount)
	}
	if ft.FinerSourceDigest != emptyFinerSourceDigest(hdr.FinerMarket) {
		t.Fatal("zero-window digest")
	}
}

func TestLabelSet_DualHitNoFinerData(t *testing.T) {
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	primary, c, parent, _, _ := dualHitFixture(t, spec)
	_, rows, ft := genFinerLabels(t, spec, primary[c:c+1], primary, nil)
	if rows[0].Reason != ReasonFinerMissing || ft.FinerWindowCount != 1 {
		t.Fatalf("got %+v windows=%d", rows[0], ft.FinerWindowCount)
	}
	zero := emptyFinerSourceDigest(finerMarketOf(testTapeHeader().Market, "1m"))
	if ft.FinerSourceDigest == zero {
		t.Fatal("attempted empty window must differ from zero-window digest")
	}
	_ = parent
}

func TestLabelSet_FinerDigestStopsAtResult(t *testing.T) {
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	primary, c, parent, upper, _ := dualHitFixture(t, spec)
	mins := minuteBars(t, primary[parent].OpenTime, 15, 100, 100.2, 99.8)
	mins[2].High = upper
	mins[2].Low = 100
	_, _, a := genFinerLabels(t, spec, primary[c:c+1], primary, mins)
	mins[10].Close += 1
	_, _, b := genFinerLabels(t, spec, primary[c:c+1], primary, mins)
	if a.FinerSourceDigest != b.FinerSourceDigest {
		t.Fatal("unconsulted later finer bar must not change FinerSourceDigest")
	}
	mins[1].Close += 1
	_, _, cft := genFinerLabels(t, spec, primary[c:c+1], primary, mins)
	if cft.FinerSourceDigest == a.FinerSourceDigest {
		t.Fatal("consulted finer bar must change FinerSourceDigest")
	}
}

func TestLabelSet_FinerNoCacheByPrimaryAt(t *testing.T) {
	specA := testResolveSpec(t, 5, 1, 1, 2, "1m")
	specB := testResolveSpec(t, 5, 3, 3, 2, "1m")
	primary := stampBars(t, boundBars(10))
	c := 4
	atrA := atrThrough(t, specA, primary, c)
	atrB := atrThrough(t, specB, primary, c)
	upperA := primary[c].Close + specA.UpperATRMultiple*atrA
	upperB := primary[c].Close + specB.UpperATRMultiple*atrB
	lowerB := primary[c].Close - specB.LowerATRMultiple*atrB
	parent := c + 1
	primary[parent].High = upperB // wide enough for both
	primary[parent].Low = lowerB
	mins := minuteBars(t, primary[parent].OpenTime, 15, 100, 100.2, 99.8)
	mins[0].High = upperA
	mins[0].Low = 100
	_, rowsA, _ := genFinerLabels(t, specA, primary[c:c+1], primary, mins)
	_, rowsB, _ := genFinerLabels(t, specB, primary[c:c+1], primary, mins)
	if rowsA[0].Outcome != OutcomeUpFirst {
		t.Fatalf("A %+v", rowsA[0])
	}
	if rowsB[0].Outcome == OutcomeUpFirst {
		t.Fatalf("B must not reuse A's UP for the same primary At, got %+v", rowsB[0])
	}
}

func TestLabelSet_V2ContentTamperRefused(t *testing.T) {
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	primary, c, parent, upper, _ := dualHitFixture(t, spec)
	mins := minuteBars(t, primary[parent].OpenTime, 15, 100, 100.2, 99.8)
	mins[0].High = upper
	mins[0].Low = 100
	dir := t.TempDir()
	tapePath, _ := writeLabelTape(t, dir, primary[c:c+1], nil)
	path := filepath.Join(dir, "t.labelset")
	fm := finerMarketOf(testTapeHeader().Market, "1m")
	if err := GenerateLabelSetWithFiner(path, tapePath, spec, primary, fm, mins, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(raw), `"UP_FIRST"`, `"DOWN_FIRST"`, 1)
	if tampered == string(raw) {
		t.Fatal("expected UP_FIRST")
	}
	if err := os.WriteFile(path, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ReadLabelSet(path, nil); err == nil {
		t.Fatal("tamper must refuse")
	}
}

func TestLabelSet_V2FooterTamperRefused(t *testing.T) {
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	primary, c, parent, _, _ := dualHitFixture(t, spec)
	dir := t.TempDir()
	tapePath, _ := writeLabelTape(t, dir, primary[c:c+1], nil)
	path := filepath.Join(dir, "t.labelset")
	fm := finerMarketOf(testTapeHeader().Market, "1m")
	if err := GenerateLabelSetWithFiner(path, tapePath, spec, primary, fm, nil, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `"finer_window_count":1`) {
		t.Fatal(s)
	}
	win := strings.Replace(s, `"finer_window_count":1`, `"finer_window_count":9`, 1)
	if err := os.WriteFile(path, []byte(win), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ReadLabelSet(path, nil); err == nil {
		t.Fatal("FinerWindowCount tamper must refuse")
	}
	src := strings.Replace(s, `"finer_source_digest":"`, `"finer_source_digest":"00`, 1)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ReadLabelSet(path, nil); err == nil {
		t.Fatal("FinerSourceDigest tamper must refuse")
	}
	_ = parent
}

func TestLabelSet_ExcludeAmbiguousUnchangedVs1A(t *testing.T) {
	spec := testLabelSpec(t, 5, 1, 1, 2)
	primary, c, parent, _, _ := dualHitFixture(t, spec)
	hdr, rows, _ := genLabels(t, spec, primary[c:c+1], primary, nil)
	if rows[0].Reason != ReasonDualHit || rows[0].HitAt != primary[parent].OpenTime {
		t.Fatalf("1A dual-hit %+v", rows[0])
	}
	if hdr.FormatVersion != LabelSetFormatV1 {
		t.Fatalf("format %s", hdr.FormatVersion)
	}
}

func TestLabelSet_No1sFallbackWhen1mPinned(t *testing.T) {
	spec := testResolveSpec(t, 5, 1, 1, 2, "1m")
	if spec.FinerTimeframe != "1m" {
		t.Fatal(spec.FinerTimeframe)
	}
	primary, c, _, _, _ := dualHitFixture(t, spec)
	_, rows, _ := genFinerLabels(t, spec, primary[c:c+1], primary, nil)
	if rows[0].Reason != ReasonFinerMissing {
		t.Fatalf("pinned 1m must not invent another TF, got %+v", rows[0])
	}
}

func TestLabelSet_CurrentHistorical15mTo1mResearchTarget(t *testing.T) {
	spec := testResolveSpec(t, 24, 1.5, 1.0, 14, "1m")
	if spec.DualHit != DualHitResolveFinerHistory || spec.FinerTimeframe != "1m" {
		t.Fatalf("research microscope %+v", spec)
	}
	primary := testTapeHeader().Market
	if primary.Timeframe != "15m" {
		t.Fatalf("research primary TF is FeatureTape MarketKey, got %s", primary.Timeframe)
	}
	if err := finerTilesPrimary(primary.Timeframe, spec.FinerTimeframe, labelAt(t, 0)); err != nil {
		t.Fatal(err)
	}
	if spec.FinerTimeframe == "1s" {
		t.Fatal("historical 15m research target must not pin 1s")
	}
}
