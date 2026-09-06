package forecast

import (
	"path/filepath"
	"strings"
	"testing"
)

func testResearchExpect(h TapeHeader, target Digest) ResearchDatasetExpect {
	return ResearchDatasetExpect{Market: h.Market, Plan: h.PlanDigest, Target: target, MinFirstAt: 0}
}

func writeResearchTape(t *testing.T, dir string, hdr TapeHeader, rows []TapeRow) (string, TapeFooter) {
	t.Helper()
	path := filepath.Join(dir, "t.featuretape")
	w, err := CreateTapeWriter(path, hdr)
	if err != nil {
		t.Fatal(err)
	}
	bars := make([]CanonicalClosedBar, len(rows))
	for i, r := range rows {
		if r.Ready {
			if err := w.WriteRow(r.At, IsReady, r.Values); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := w.WriteRow(r.At, NotReady, nil); err != nil {
				t.Fatal(err)
			}
		}
		bars[i] = CanonicalClosedBar{OpenTime: r.At, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1}
	}
	src := SourceRangeDigest(hdr.Market, bars)
	if err := w.Finish(src); err != nil {
		t.Fatal(err)
	}
	_, _, ft, err := ReadTape(path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return path, ft
}

func writeResearchLabels(t *testing.T, dir string, tapeHdr TapeHeader, tf TapeFooter, target Digest, rows []LabelRow) string {
	t.Helper()
	path := filepath.Join(dir, "t.labelset")
	lh := LabelHeader{
		FormatVersion:                LabelSetFormatV1,
		Market:                       tapeHdr.Market,
		TargetDigest:                 target,
		LabelLogicVersion:            LabelLogicFirstPassagePrimaryV1,
		FeatureTapePlanDigest:        tapeHdr.PlanDigest,
		FeatureTapeSourceRangeDigest: tf.SourceRangeDigest,
		FeatureTapeContentDigest:     tf.ContentDigest,
	}
	w, err := CreateLabelWriter(path, lh)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if err := w.WriteRow(r); err != nil {
			t.Fatal(err)
		}
	}
	var src Digest
	src[0] = 0x11
	if err := w.Finish(src); err != nil {
		t.Fatal(err)
	}
	return path
}

func fixtureAt(t *testing.T, i int) int64 { return labelAt(t, i) }

func TestBuildResearchDataset_EligibilityAndCopy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hdr := testTapeHeader()
	target := Digest{0x42}
	at0, at1, at2, at3, at4 := fixtureAt(t, 0), fixtureAt(t, 1), fixtureAt(t, 2), fixtureAt(t, 3), fixtureAt(t, 4)
	vals := []float64{10, 20, 0, 3}
	tapePath, tf := writeResearchTape(t, dir, hdr, []TapeRow{
		{At: at0, Ready: NotReady},
		{At: at1, Ready: IsReady, Values: vals},
		{At: at2, Ready: IsReady, Values: []float64{1, 2, 0, 0}},
		{At: at3, Ready: IsReady, Values: []float64{3, 4, 0, 0}},
		{At: at4, Ready: IsReady, Values: []float64{5, 6, 0, 0}},
	})
	labelPath := writeResearchLabels(t, dir, hdr, tf, target, []LabelRow{
		{At: at0, Outcome: OutcomeAmbiguous, Reason: ReasonATRZero},
		{At: at1, Outcome: OutcomeUpFirst, Reason: ReasonNone, HitAt: at1 + 1},
		{At: at2, Outcome: OutcomeDownFirst, Reason: ReasonNone, HitAt: at2 + 1},
		{At: at3, Outcome: OutcomeTimeout, Reason: ReasonNone},
		{At: at4, Outcome: OutcomeAmbiguous, Reason: ReasonTruncatedHorizon},
	})
	vals[0] = 999
	rows, acc, err := BuildResearchDataset(tapePath, labelPath, testResearchExpect(hdr, target))
	if err != nil {
		t.Fatal(err)
	}
	if acc.TotalCandidates != 5 || acc.FeatureNotReady != 1 || acc.TrainableUP != 1 || acc.TrainableDOWN != 1 || acc.TrainableTIMEOUT != 1 {
		t.Fatalf("accounting %+v", acc)
	}
	if acc.ExcludedByReason[ReasonTruncatedHorizon] != 1 || acc.ExcludedByReason[ReasonATRZero] != 0 {
		t.Fatalf("Ready=false ATR_ZERO must not also exclude, got %+v", acc.ExcludedByReason)
	}
	if acc.TrainableTotal != 3 || len(rows) != 3 {
		t.Fatalf("trainable %d rows %d", acc.TrainableTotal, len(rows))
	}
	if rows[0].Features[0] != 10 {
		t.Fatalf("feature copy broken: got %v want 10 (src mutated to 999 before read is independent; tape file has 10)", rows[0].Features)
	}
	rows[0].Features[0] = -1
	rows2, _, err := BuildResearchDataset(tapePath, labelPath, testResearchExpect(hdr, target))
	if err != nil {
		t.Fatal(err)
	}
	if rows2[0].Features[0] != 10 {
		t.Fatal("rebuild must not see mutated ResearchRow alias")
	}
}

func TestCloneFeatureVector_NoAlias(t *testing.T) {
	t.Parallel()
	src := []float64{1, 2, 3, 4}
	dst := cloneFeatureVector(src)
	src[0] = 99
	if dst[0] != 1 {
		t.Fatal("cloneFeatureVector aliased src")
	}
}

func TestBuildResearchDataset_ProvenanceRefuse(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hdr := testTapeHeader()
	target := Digest{0x42}
	at := fixtureAt(t, 0)
	tapePath, tf := writeResearchTape(t, dir, hdr, []TapeRow{
		{At: at, Ready: IsReady, Values: []float64{1, 2, 0, 0}},
	})
	okLabels := []LabelRow{{At: at, Outcome: OutcomeTimeout, Reason: ReasonNone}}

	t.Run("plan", func(t *testing.T) {
		bad := hdr
		bad.PlanDigest[1] ^= 0xff
		lh := writeResearchLabels(t, t.TempDir(), bad, tf, target, okLabels)
		_, _, err := BuildResearchDataset(tapePath, lh, testResearchExpect(hdr, target))
		if err == nil || !strings.Contains(err.Error(), "PlanDigest") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("source", func(t *testing.T) {
		tf2 := tf
		tf2.SourceRangeDigest[1] ^= 0xff
		lh := writeResearchLabels(t, t.TempDir(), hdr, tf2, target, okLabels)
		_, _, err := BuildResearchDataset(tapePath, lh, testResearchExpect(hdr, target))
		if err == nil || !strings.Contains(err.Error(), "SourceRangeDigest") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("content", func(t *testing.T) {
		tf2 := tf
		tf2.ContentDigest[1] ^= 0xff
		lh := writeResearchLabels(t, t.TempDir(), hdr, tf2, target, okLabels)
		_, _, err := BuildResearchDataset(tapePath, lh, testResearchExpect(hdr, target))
		if err == nil || !strings.Contains(err.Error(), "ContentDigest") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("market", func(t *testing.T) {
		bad := hdr
		bad.Market.Contract = "SPOT"
		lh := writeResearchLabels(t, t.TempDir(), bad, tf, target, okLabels)
		_, _, err := BuildResearchDataset(tapePath, lh, testResearchExpect(hdr, target))
		if err == nil || !strings.Contains(err.Error(), "PrimaryMarketKey") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("target", func(t *testing.T) {
		other := Digest{0x99}
		lh := writeResearchLabels(t, t.TempDir(), hdr, tf, other, okLabels)
		_, _, err := BuildResearchDataset(tapePath, lh, testResearchExpect(hdr, target))
		if err == nil || !strings.Contains(err.Error(), "TargetDigest") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestBuildResearchDataset_LockstepRefuse(t *testing.T) {
	t.Parallel()
	hdr := testTapeHeader()
	target := Digest{0x42}
	at0, at1 := fixtureAt(t, 0), fixtureAt(t, 1)
	tapePath, tf := writeResearchTape(t, t.TempDir(), hdr, []TapeRow{
		{At: at0, Ready: IsReady, Values: []float64{1, 2, 0, 0}},
		{At: at1, Ready: IsReady, Values: []float64{1, 2, 0, 0}},
	})
	t.Run("length", func(t *testing.T) {
		lh := writeResearchLabels(t, t.TempDir(), hdr, tf, target, []LabelRow{
			{At: at0, Outcome: OutcomeTimeout, Reason: ReasonNone},
		})
		_, _, err := BuildResearchDataset(tapePath, lh, testResearchExpect(hdr, target))
		if err == nil || !strings.Contains(err.Error(), "row count") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("at", func(t *testing.T) {
		lh := writeResearchLabels(t, t.TempDir(), hdr, tf, target, []LabelRow{
			{At: at0, Outcome: OutcomeTimeout, Reason: ReasonNone},
			{At: fixtureAt(t, 9), Outcome: OutcomeTimeout, Reason: ReasonNone},
		})
		_, _, err := BuildResearchDataset(tapePath, lh, testResearchExpect(hdr, target))
		if err == nil || !strings.Contains(err.Error(), "At mismatch") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestBuildResearchDataset_GenesisFloor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hdr := testTapeHeader()
	target := Digest{0x42}
	at := fixtureAt(t, 0)
	tapePath, tf := writeResearchTape(t, dir, hdr, []TapeRow{
		{At: at, Ready: IsReady, Values: []float64{1, 2, 0, 0}},
	})
	labelPath := writeResearchLabels(t, dir, hdr, tf, target, []LabelRow{
		{At: at, Outcome: OutcomeTimeout, Reason: ReasonNone},
	})
	exp := testResearchExpect(hdr, target)
	exp.MinFirstAt = at + 1
	_, _, err := BuildResearchDataset(tapePath, labelPath, exp)
	if err == nil || !strings.Contains(err.Error(), "research floor") {
		t.Fatalf("got %v", err)
	}
}
