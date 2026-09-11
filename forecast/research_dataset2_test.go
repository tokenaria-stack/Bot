package forecast

import (
	"path/filepath"
	"strings"
	"testing"
)

func writeResearchLabels2(t *testing.T, dir string, spec2 FeatureSpec2, th Tape2Header, tf Tape2Footer, rows []LabelRow) string {
	t.Helper()
	path := filepath.Join(dir, "t.labelset")
	tid, err := spec2.Target.Identity()
	if err != nil {
		t.Fatal(err)
	}
	fm := spec2.Primary
	fm.Timeframe = spec2.Target.FinerTimeframe
	src := Digest{0x11}
	if err := writeLabelBuild(path, labelBuild{
		Header: LabelHeader{
			FormatVersion:                LabelSetFormatV2,
			Market:                       th.Primary,
			TargetDigest:                 tid.Digest,
			LabelLogicVersion:            LabelLogicFirstPassageFinerV1,
			FeatureTapePlanDigest:        th.PlanDigest,
			FeatureTapeSourceRangeDigest: th.PrimarySource,
			FeatureTapeContentDigest:     tf.ContentDigest,
			FinerMarket:                  fm,
		},
		Rows:        rows,
		Source:      src,
		FinerSource: emptyFinerSourceDigest(fm),
	}); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBuildResearchDatasetFromTape2_RefusesV1Tape(t *testing.T) {
	spec2 := testSpec2(t)
	dir := t.TempDir()
	hdr := testTapeHeader()
	at := fixtureAt(t, 0)
	tapePath, _ := writeResearchTape(t, dir, hdr, []TapeRow{
		{At: at, Ready: IsReady, Values: []float64{1, 2, 0, 0}},
	})
	dummy := tape2HeaderForPrimary(t, spec2, []CanonicalClosedBar{{OpenTime: at, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1}})
	labelPath := writeResearchLabels2(t, dir, spec2, dummy, Tape2Footer{ContentDigest: dummy.PrimarySource}, []LabelRow{
		{At: at, Outcome: OutcomeTimeout, Reason: ReasonNone},
	})
	_, _, err := BuildResearchDatasetFromTape2(tapePath, labelPath, spec2, ResearchDataset2Expect{})
	if err == nil {
		t.Fatal("expected native refuse of feature-tape-v1")
	}
}

func TestBuildResearchDatasetFromTape2_PartitionAndCopy(t *testing.T) {
	spec2 := testSpec2(t)
	dir := t.TempDir()
	ats := []int64{fixtureAt(t, 0), fixtureAt(t, 1), fixtureAt(t, 2), fixtureAt(t, 3), fixtureAt(t, 4)}
	bars := make([]CanonicalClosedBar, len(ats))
	for i, at := range ats {
		bars[i] = CanonicalClosedBar{OpenTime: at, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1}
	}
	readyAt := map[int64]bool{ats[0]: false}
	tapePath, th, tf := writeLabelTape2(t, dir, spec2, bars, readyAt, 1.5)
	labelPath := writeResearchLabels2(t, dir, spec2, th, tf, []LabelRow{
		{At: ats[0], Outcome: OutcomeUpFirst, Reason: ReasonNone, HitAt: ats[0] + 1},
		{At: ats[1], Outcome: OutcomeUpFirst, Reason: ReasonNone, HitAt: ats[1] + 1},
		{At: ats[2], Outcome: OutcomeDownFirst, Reason: ReasonNone, HitAt: ats[2] + 1},
		{At: ats[3], Outcome: OutcomeTimeout, Reason: ReasonNone},
		{At: ats[4], Outcome: OutcomeAmbiguous, Reason: ReasonTruncatedHorizon},
	})
	rows, acc, err := BuildResearchDatasetFromTape2(tapePath, labelPath, spec2, ResearchDataset2Expect{})
	if err != nil {
		t.Fatal(err)
	}
	if acc.TotalCandidates != 5 || acc.FeatureNotReady != 1 || acc.ExcludedTotal != 1 || acc.TrainableTotal != 3 {
		t.Fatalf("accounting %+v", acc)
	}
	if acc.FeatureNotReady+acc.ExcludedTotal+acc.TrainableTotal != acc.TotalCandidates {
		t.Fatal("partition")
	}
	if acc.ExcludedByReason[ReasonTruncatedHorizon] != 1 || acc.ExcludedByReason[ReasonATRZero] != 0 {
		t.Fatalf("NotReady legal UP must not also exclude, got %+v", acc.ExcludedByReason)
	}
	if acc.TrainableUP != 1 || acc.TrainableDOWN != 1 || acc.TrainableTIMEOUT != 1 || len(rows) != 3 {
		t.Fatalf("trainable %+v rows=%d", acc, len(rows))
	}
	if rows[0].Features != dummyVec2AsVec(1.5) {
		t.Fatalf("width/copy %+v", rows[0].Features[0])
	}
	if len(rows[0].Features) != Spec2FeatureWidth {
		t.Fatal("FeatureVector2 width")
	}
	rows[0].Features[0] = -1
	rows2, _, err := BuildResearchDatasetFromTape2(tapePath, labelPath, spec2, ResearchDataset2Expect{})
	if err != nil {
		t.Fatal(err)
	}
	if rows2[0].Features[0] != dummyVec2(1.5)[0] {
		t.Fatal("ResearchRow2 must own FeatureVector2 by value")
	}
}

func dummyVec2AsVec(seed float64) FeatureVector2 {
	var v FeatureVector2
	copy(v[:], dummyVec2(seed))
	return v
}

func TestBuildResearchDatasetFromTape2_ProvenanceRefuse(t *testing.T) {
	spec2 := testSpec2(t)
	dir := t.TempDir()
	bars := []CanonicalClosedBar{{OpenTime: fixtureAt(t, 0), Open: 1, High: 1, Low: 1, Close: 1, Volume: 1}}
	tapePath, th, tf := writeLabelTape2(t, dir, spec2, bars, nil, 1)
	ok := []LabelRow{{At: bars[0].OpenTime, Outcome: OutcomeTimeout, Reason: ReasonNone}}
	labelPath := writeResearchLabels2(t, dir, spec2, th, tf, ok)

	t.Run("spec2", func(t *testing.T) {
		other := spec2
		other.Primary.Instrument = "ETHUSDT"
		var err error
		other, err = ResolveFeatureSpec2(other.Primary, spec2.Target, spec2.Analysis)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = BuildResearchDatasetFromTape2(tapePath, labelPath, other, ResearchDataset2Expect{})
		if err == nil {
			t.Fatal("expected spec2 refuse")
		}
	})
	t.Run("target", func(t *testing.T) {
		v1, err := ResolveTargetSpec("research-15m-1m", TargetSpecDraft{
			HorizonBars: 24, UpperATRMultiple: 1.5, LowerATRMultiple: 1.0, ATRPeriod: 14,
			DualHit: DualHitResolveFinerHistory, FinerTimeframe: "1m",
		}, "labels:v1")
		if err != nil {
			t.Fatal(err)
		}
		badSpec := spec2
		badSpec.Target = v1
		_, _, err = BuildResearchDatasetFromTape2(tapePath, labelPath, badSpec, ResearchDataset2Expect{})
		if err == nil {
			t.Fatal("expected TargetDigest refuse")
		}
	})
	t.Run("content", func(t *testing.T) {
		bad := tf.ContentDigest
		bad[0] ^= 0xff
		_, _, err := BuildResearchDatasetFromTape2(tapePath, labelPath, spec2, ResearchDataset2Expect{TapeContent: &bad})
		if err == nil || !strings.Contains(err.Error(), "ContentDigest") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestBuildResearchDatasetFromTape2_LockstepRefuse(t *testing.T) {
	spec2 := testSpec2(t)
	bars := []CanonicalClosedBar{
		{OpenTime: fixtureAt(t, 0), Open: 1, High: 1, Low: 1, Close: 1, Volume: 1},
		{OpenTime: fixtureAt(t, 1), Open: 1, High: 1, Low: 1, Close: 1, Volume: 1},
	}
	tapePath, th, tf := writeLabelTape2(t, t.TempDir(), spec2, bars, nil, 1)
	t.Run("length", func(t *testing.T) {
		lh := writeResearchLabels2(t, t.TempDir(), spec2, th, tf, []LabelRow{
			{At: bars[0].OpenTime, Outcome: OutcomeTimeout, Reason: ReasonNone},
		})
		_, _, err := BuildResearchDatasetFromTape2(tapePath, lh, spec2, ResearchDataset2Expect{})
		if err == nil || !strings.Contains(err.Error(), "row count") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("at", func(t *testing.T) {
		lh := writeResearchLabels2(t, t.TempDir(), spec2, th, tf, []LabelRow{
			{At: bars[0].OpenTime, Outcome: OutcomeTimeout, Reason: ReasonNone},
			{At: fixtureAt(t, 9), Outcome: OutcomeTimeout, Reason: ReasonNone},
		})
		_, _, err := BuildResearchDatasetFromTape2(tapePath, lh, spec2, ResearchDataset2Expect{})
		if err == nil || !strings.Contains(err.Error(), "At mismatch") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestBuildResearchDatasetFromTape2_WrongLabelContent(t *testing.T) {
	spec2 := testSpec2(t)
	bars := []CanonicalClosedBar{{OpenTime: fixtureAt(t, 0), Open: 1, High: 1, Low: 1, Close: 1, Volume: 1}}
	tapePath, th, tf := writeLabelTape2(t, t.TempDir(), spec2, bars, nil, 1)
	labelPath := writeResearchLabels2(t, t.TempDir(), spec2, th, tf, []LabelRow{
		{At: bars[0].OpenTime, Outcome: OutcomeTimeout, Reason: ReasonNone},
	})
	bad := Digest{0x99}
	_, _, err := BuildResearchDatasetFromTape2(tapePath, labelPath, spec2, ResearchDataset2Expect{LabelContent: &bad})
	if err == nil || !strings.Contains(err.Error(), "LabelSet ContentDigest") {
		t.Fatalf("got %v", err)
	}
}
