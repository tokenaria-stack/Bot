package forecast

import (
	"testing"

	"trading_bot/data"
)

func TestCompileValidationPlan_SparseH72IsMarketTime(t *testing.T) {
	t.Parallel()
	start := int64(1_699_999_200_000)
	dense := seq15m(t, start, 400)
	sparse := make([]int64, 0, len(dense)/2)
	for i, at := range dense {
		if i%2 == 0 {
			sparse = append(sparse, at)
		}
	}
	holdout := dense[360]
	draft := ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: holdout, ValidationSpanBars: 20,
		FoldCount: 1, ExtraGapBars: 0, MinTrainRows: 5,
	}
	p := mustPlan(t, draft, 72)
	if p.TargetH != 72 {
		t.Fatal(p.TargetH)
	}
	causal, err := p.TotalCausalBars()
	if err != nil || causal != 72 {
		t.Fatalf("causal %d %v", causal, err)
	}
	got, err := CompileValidationPlan(sparse, "15m", p)
	if err != nil {
		t.Fatal(err)
	}
	fold := got.Folds[0]
	if fold.ValEnd-fold.ValBegin == 20 {
		t.Fatal("realized ValN must not be forced to ValidationSpanBars over a sparse At[]")
	}
	if fold.ValEnd-fold.ValBegin >= 20 {
		t.Fatalf("sparse ValN=%d want < 20 market bars of observations", fold.ValEnd-fold.ValBegin)
	}
	hops := 0
	at := fold.ValBoundaryStartAt
	for at < fold.ValBoundaryEndAt {
		next, err := data.NextBarOpen(at, "15m")
		if err != nil {
			t.Fatal(err)
		}
		hops++
		at = next
	}
	if hops != 20 {
		t.Fatalf("validation market-clock hops=%d want 20", hops)
	}
	if fold.TrainEnd == fold.ValBegin-72 {
		t.Fatal("TrainEnd must not be ValBegin-72 sparse rows")
	}
	he, err := HorizonEnd(fold.TrainLastAt, "15m", 72)
	if err != nil {
		t.Fatal(err)
	}
	if he >= fold.ValBoundaryStartAt {
		t.Fatalf("HorizonEnd(TrainLast)=%d not < valStart %d", he, fold.ValBoundaryStartAt)
	}
}

func TestCompileValidationPlan_H72HoldoutSeam(t *testing.T) {
	t.Parallel()
	start := int64(1_699_999_200_000)
	ats := seq15m(t, start, 250)
	holdout := ats[200]
	draft := ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: holdout, ValidationSpanBars: 30,
		FoldCount: 1, ExtraGapBars: 0, MinTrainRows: 5,
	}
	p := mustPlan(t, draft, 72)
	got, err := CompileValidationPlan(ats, "15m", p)
	if err != nil {
		t.Fatal(err)
	}
	causal := 72
	seam := 0
	for i := got.DevelopmentEndIndex; i < got.HoldoutBeginIndex; i++ {
		h, err := HorizonEnd(ats[i], "15m", causal)
		if err != nil {
			t.Fatal(err)
		}
		if h < holdout {
			t.Fatalf("seam row %d HorizonEnd=%d still < wall", i, h)
		}
		seam++
	}
	if seam == 0 {
		t.Fatal("H72 seam must be nonempty vs H=24")
	}
	if got.Folds[0].ValEnd > got.HoldoutBeginIndex {
		t.Fatal("validation reached holdout")
	}
	for i := got.Folds[0].ValBegin; i < got.Folds[0].ValEnd; i++ {
		h, err := HorizonEnd(ats[i], "15m", causal)
		if err != nil {
			t.Fatal(err)
		}
		if h >= holdout {
			t.Fatalf("val row crosses wall At=%d", ats[i])
		}
	}
}

func TestCompileValidationPlan_H72DigestDiffersFromH24(t *testing.T) {
	t.Parallel()
	d := ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: 1_699_999_200_000, ValidationSpanBars: 17568,
		FoldCount: 4, ExtraGapBars: 0, MinTrainRows: 35040,
	}
	a := mustPlan(t, d, 24)
	c := mustPlan(t, d, 72)
	ida, _ := a.Identity()
	idc, _ := c.Identity()
	if ida.Digest == idc.Digest {
		t.Fatal("TargetH must change ValidationPlan digest")
	}
	if c.TargetH != 72 || a.TargetH != 24 {
		t.Fatal(a.TargetH, c.TargetH)
	}
	if c.ValidationSpanBars == 8640 || d.ValidationSpanBars == 8640 {
		t.Fatal("DecisionResearch span leak")
	}
}
