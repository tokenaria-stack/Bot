package forecast

import (
	"strings"
	"testing"

	"trading_bot/data"
)

func testValSpec(t *testing.T, h int) TargetSpec {
	t.Helper()
	spec, err := ResolveTargetSpec("t", TargetSpecDraft{
		HorizonBars: h, UpperATRMultiple: 1, LowerATRMultiple: 1, ATRPeriod: 14,
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func seq15m(t *testing.T, start int64, n int) []int64 {
	t.Helper()
	out := make([]int64, n)
	at := start
	for i := 0; i < n; i++ {
		open, err := data.CurrentBarOpen(at, "15m")
		if err != nil || open != at {
			t.Fatalf("not on 15m grid %d", at)
		}
		out[i] = at
		next, err := data.NextBarOpen(at, "15m")
		if err != nil {
			t.Fatal(err)
		}
		at = next
	}
	return out
}

func mustPlan(t *testing.T, d ValidationPlanDraft, h int) ValidationPlan {
	t.Helper()
	p, err := ResolveValidationPlan(d, testValSpec(t, h), ValidationLogicWalkForwardV1)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCompileValidationPlan_HorizonOffByOne(t *testing.T) {
	t.Parallel()
	start := int64(1_699_999_200_000)
	ats := seq15m(t, start, 80)
	// Holdout far enough that 1 fold of 10 bars packs inside the series.
	holdout := ats[70]
	draft := ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: holdout, ValidationSpanBars: 10,
		FoldCount: 1, ExtraGapBars: 0, MinTrainRows: 5,
	}
	p := mustPlan(t, draft, 2)
	got, err := CompileValidationPlan(ats, "15m", p)
	if err != nil {
		t.Fatal(err)
	}
	v := got.Folds[0].ValBoundaryStartAt
	causal, _ := p.TotalCausalBars()
	// First illegal calendar open for train is hopPrev(V, H); last train At must have HorizonEnd < V.
	he, err := HorizonEnd(got.Folds[0].TrainLastAt, "15m", causal)
	if err != nil {
		t.Fatal(err)
	}
	if he >= v {
		t.Fatalf("TrainLast HorizonEnd=%d not < V=%d", he, v)
	}
	nextTrain, err := data.NextBarOpen(got.Folds[0].TrainLastAt, "15m")
	if err != nil {
		t.Fatal(err)
	}
	heEq, err := HorizonEnd(nextTrain, "15m", causal)
	if err != nil {
		t.Fatal(err)
	}
	if heEq < v {
		t.Fatalf("expected next calendar after TrainLast to be illegal, HorizonEnd=%d V=%d", heEq, v)
	}
	for i := got.Folds[0].TrainBegin; i < got.Folds[0].TrainEnd; i++ {
		h, err := HorizonEnd(ats[i], "15m", causal)
		if err != nil {
			t.Fatal(err)
		}
		if h >= v {
			t.Fatalf("train idx %d HorizonEnd=%d >= V", i, h)
		}
	}
}

func TestCompileValidationPlan_RefuseMalformed(t *testing.T) {
	t.Parallel()
	start := int64(1_699_999_200_000)
	ats := seq15m(t, start, 40)
	holdout := ats[35]
	ok := ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: holdout, ValidationSpanBars: 5,
		FoldCount: 1, ExtraGapBars: 0, MinTrainRows: 3,
	}
	p := mustPlan(t, ok, 2)

	dup := append([]int64(nil), ats...)
	dup[10] = dup[9]
	if _, err := CompileValidationPlan(dup, "15m", p); err == nil || !strings.Contains(err.Error(), "strictly increasing") {
		t.Fatalf("duplicate: %v", err)
	}
	dec := append([]int64(nil), ats...)
	dec[5], dec[6] = dec[6], dec[5]
	if _, err := CompileValidationPlan(dec, "15m", p); err == nil || !strings.Contains(err.Error(), "strictly increasing") {
		t.Fatalf("unsorted: %v", err)
	}
	off := p
	off.HoldoutStartAt = holdout + 7*60*1000
	if _, err := CompileValidationPlan(ats, "15m", off); err == nil || !strings.Contains(err.Error(), "off the") {
		t.Fatalf("off-grid: %v", err)
	}
	if _, err := CompileValidationPlan(ats, "1h", p); err == nil || !strings.Contains(err.Error(), "timeframe") {
		t.Fatalf("tf mismatch: %v", err)
	}
	after := p
	after.HoldoutStartAt = ats[len(ats)-1]
	next, err := data.NextBarOpen(after.HoldoutStartAt, "15m")
	if err != nil {
		t.Fatal(err)
	}
	after.HoldoutStartAt = next
	if _, err := CompileValidationPlan(ats, "15m", after); err == nil || !strings.Contains(err.Error(), "empty final holdout") {
		t.Fatalf("empty holdout: %v", err)
	}
	tiny := p
	tiny.MinTrainRows = 100000
	if _, err := CompileValidationPlan(ats, "15m", tiny); err == nil || !strings.Contains(err.Error(), "MinTrainRows") {
		t.Fatalf("min train: %v", err)
	}
}

func TestResolveValidationPlan_InvalidFields(t *testing.T) {
	t.Parallel()
	spec := testValSpec(t, 2)
	base := ValidationPlanDraft{Timeframe: "15m", HoldoutStartAt: 1_699_999_200_000, ValidationSpanBars: 5, FoldCount: 1, MinTrainRows: 1}
	if _, err := ResolveValidationPlan(base, spec, ""); err == nil {
		t.Fatal("empty logic")
	}
	bad := base
	bad.ValidationSpanBars = 0
	if _, err := ResolveValidationPlan(bad, spec, ValidationLogicWalkForwardV1); err == nil {
		t.Fatal("span")
	}
	bad = base
	bad.FoldCount = 0
	if _, err := ResolveValidationPlan(bad, spec, ValidationLogicWalkForwardV1); err == nil {
		t.Fatal("folds")
	}
	zeroH, err := ResolveTargetSpec("t", TargetSpecDraft{HorizonBars: 1, UpperATRMultiple: 1, LowerATRMultiple: 1}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	zeroH.HorizonBars = 0
	if _, err := ResolveValidationPlan(base, zeroH, ValidationLogicWalkForwardV1); err == nil {
		t.Fatal("H")
	}
	bad = base
	bad.ExtraGapBars = -1
	if _, err := ResolveValidationPlan(bad, spec, ValidationLogicWalkForwardV1); err == nil {
		t.Fatal("extra")
	}
	bad = base
	bad.MinTrainRows = 0
	if _, err := ResolveValidationPlan(bad, spec, ValidationLogicWalkForwardV1); err == nil {
		t.Fatal("min")
	}
}

func TestCompileValidationPlan_EmptyValidationWindow(t *testing.T) {
	t.Parallel()
	start := int64(1_699_999_200_000)
	early := seq15m(t, start, 25)
	holdout := early[0]
	var err error
	for i := 0; i < 40; i++ {
		holdout, err = data.NextBarOpen(holdout, "15m")
		if err != nil {
			t.Fatal(err)
		}
	}
	late := seq15m(t, holdout, 8)
	ats := append(append([]int64{}, early...), late...)
	p := mustPlan(t, ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: holdout, ValidationSpanBars: 8,
		FoldCount: 1, ExtraGapBars: 0, MinTrainRows: 3,
	}, 2)
	_, err = CompileValidationPlan(ats, "15m", p)
	if err == nil || !strings.Contains(err.Error(), "empty validation") {
		t.Fatalf("got %v", err)
	}
}

func TestValidationPlan_IdentityRulesOnly(t *testing.T) {
	t.Parallel()
	start := int64(1_699_999_200_000)
	a := seq15m(t, start, 50)
	b := seq15m(t, start, 60)
	holdout := a[40]
	d := ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: holdout, ValidationSpanBars: 6,
		FoldCount: 1, ExtraGapBars: 0, MinTrainRows: 4,
	}
	p := mustPlan(t, d, 2)
	id1, err := p.Identity()
	if err != nil {
		t.Fatal(err)
	}
	c1, err := CompileValidationPlan(a, "15m", p)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := CompileValidationPlan(b, "15m", p)
	if err != nil {
		t.Fatal(err)
	}
	if c1.Plan.Digest != id1.Digest || c2.Plan.Digest != id1.Digest {
		t.Fatal("compiled plan identity must equal rules identity")
	}
	shift := seq15m(t, start+15*60*1000, 50)
	c3, err := CompileValidationPlan(shift, "15m", p)
	if err != nil {
		t.Fatal(err)
	}
	if c3.Plan.Digest != id1.Digest {
		t.Fatal("shifted At[] must keep ValidationPlan digest")
	}
	if c3.HoldoutBeginIndex == c1.HoldoutBeginIndex && c3.Folds[0].ValBegin == c1.Folds[0].ValBegin {
		t.Fatal("shifted chronology should move compiled indices")
	}
	mutate := []struct {
		name string
		fn   func(*ValidationPlan)
	}{
		{"tf", func(p *ValidationPlan) { p.Timeframe = "1m" }},
		{"holdout", func(p *ValidationPlan) { p.HoldoutStartAt += 15 * 60 * 1000 }},
		{"span", func(p *ValidationPlan) { p.ValidationSpanBars++ }},
		{"folds", func(p *ValidationPlan) { p.FoldCount++ }},
		{"h", func(p *ValidationPlan) { p.TargetH++ }},
		{"extra", func(p *ValidationPlan) { p.ExtraGapBars++ }},
		{"min", func(p *ValidationPlan) { p.MinTrainRows++ }},
		{"logic", func(p *ValidationPlan) { p.Logic = "validation:walk-forward-v2" }},
	}
	for _, m := range mutate {
		q := p
		m.fn(&q)
		id2, err := q.Identity()
		if err != nil {
			continue
		}
		if id2.Digest == id1.Digest {
			t.Fatalf("%s must change ValidationPlan digest", m.name)
		}
	}
}

func TestCompileValidationPlan_DisjointAndHoldoutIsolation(t *testing.T) {
	t.Parallel()
	start := int64(1_699_999_200_000)
	ats := seq15m(t, start, 200)
	holdout := ats[180]
	p := mustPlan(t, ValidationPlanDraft{
		Timeframe: "15m", HoldoutStartAt: holdout, ValidationSpanBars: 20,
		FoldCount: 3, ExtraGapBars: 0, MinTrainRows: 10,
	}, 2)
	got, err := CompileValidationPlan(ats, "15m", p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Folds[2].ValBoundaryEndAt != got.DevelopmentExclusiveEndAt {
		t.Fatal("last val end != development exclusive end")
	}
	for i := range got.Folds {
		f := got.Folds[i]
		if f.ValBegin >= got.HoldoutBeginIndex || f.ValEnd > got.HoldoutBeginIndex {
			t.Fatalf("fold %d val hits holdout", i)
		}
		if f.TrainEnd > got.HoldoutBeginIndex {
			t.Fatalf("fold %d train hits holdout", i)
		}
		for j := i + 1; j < len(got.Folds); j++ {
			g := got.Folds[j]
			if f.ValEnd > g.ValBegin && g.ValEnd > f.ValBegin {
				t.Fatal("overlap")
			}
		}
	}
}
