package exchange

import (
	"testing"
	"trading_bot/data"
)

func kOHLC(openMs int64, o, h, l, c, v float64) Kline {
	return Kline{OpenTime: openMs, CloseTime: openMs + 59_999, Open: o, High: h, Low: l, Close: c, Volume: v}
}

func TestRequiredParentCountMappings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		child string
		want  int
	}{
		{"2m", 2},
		{"10m", 2},
		{"45m", 3},
		{"3h", 3},
	}
	for _, tc := range cases {
		got, err := RequiredParentCount(tc.child)
		if err != nil {
			t.Fatalf("%s: %v", tc.child, err)
		}
		if got != tc.want {
			t.Fatalf("%s ratio=%d want %d", tc.child, got, tc.want)
		}
		e, _ := TimeframeByName(tc.child)
		if e.Parent == "" || e.Persist {
			t.Fatalf("%s catalog persist/parent", tc.child)
		}
	}
}

func TestParentBarsNeededIncludesAlignment(t *testing.T) {
	t.Parallel()
	n, err := ParentBarsNeeded(3000, "2m")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3000*2+2 {
		t.Fatalf("got %d want 6002", n)
	}
	n45, err := ParentBarsNeeded(100, "45m")
	if err != nil {
		t.Fatal(err)
	}
	if n45 != 100*3+3 {
		t.Fatalf("45m got %d", n45)
	}
}

func TestFoldClosedChildrenOHLCVAndBuckets(t *testing.T) {
	t.Parallel()
	// Two complete 2m buckets from 1m.
	parents := []Kline{
		kOHLC(0, 10, 12, 9, 11, 1),
		kOHLC(60_000, 11, 15, 10, 14, 2),
		kOHLC(120_000, 14, 16, 13, 15, 3),
		kOHLC(180_000, 15, 17, 14, 16, 4),
	}
	closed, forming, err := FoldClosedChildren(parents, "2m", false)
	if err != nil {
		t.Fatal(err)
	}
	if forming != nil {
		t.Fatal("history fold must not return forming")
	}
	if len(closed) != 2 {
		t.Fatalf("closed=%d want 2", len(closed))
	}
	if closed[0].OpenTime != 0 || closed[0].Open != 10 || closed[0].High != 15 || closed[0].Low != 9 || closed[0].Close != 14 || closed[0].Volume != 3 {
		t.Fatalf("first child %+v", closed[0])
	}
	ct, _ := data.BarCloseTimeMs(0, "2m")
	if closed[0].CloseTime != ct {
		t.Fatalf("CloseTime=%d want %d", closed[0].CloseTime, ct)
	}
	if closed[1].OpenTime != 120_000 || closed[1].Volume != 7 {
		t.Fatalf("second %+v", closed[1])
	}
}

func TestFoldDropsPartialLeftAndHoles(t *testing.T) {
	t.Parallel()
	// Window starts at 09:01 equivalent: only second 1m of first 2m bucket.
	parents := []Kline{
		kOHLC(60_000, 1, 1, 1, 1, 1),
		kOHLC(120_000, 2, 3, 2, 2, 1),
		kOHLC(180_000, 2, 4, 1, 3, 1),
	}
	closed, _, err := FoldClosedChildren(parents, "2m", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(closed) != 1 || closed[0].OpenTime != 120_000 {
		t.Fatalf("want only 120000 bucket, got %+v", closed)
	}

	hole := []Kline{
		kOHLC(0, 1, 1, 1, 1, 1),
		// 60000 missing
		kOHLC(120_000, 2, 2, 2, 2, 1),
		kOHLC(180_000, 2, 2, 2, 2, 1),
	}
	closed, _, err = FoldClosedChildren(hole, "2m", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(closed) != 1 || closed[0].OpenTime != 120_000 {
		t.Fatalf("hole must not emit shortened 0-bucket: %+v", closed)
	}
}

func TestAccumulatorReplaceSameParentBucket(t *testing.T) {
	t.Parallel()
	a, err := NewDerivedAccumulator("2m")
	if err != nil {
		t.Fatal(err)
	}
	p := kOHLC(0, 10, 11, 9, 10.5, 1)
	_, closed, ok := a.OnParent(p, false)
	if !ok || closed || a.ClosedParentCount() != 0 {
		t.Fatalf("forming: closed=%v count=%d", closed, a.ClosedParentCount())
	}
	p.High = 20
	p.Close = 19
	child, closed, ok := a.OnParent(p, false)
	if !ok || closed || a.ClosedParentCount() != 0 {
		t.Fatal("repeat forming must not count as a new parent")
	}
	if child.High != 20 || child.Close != 19 {
		t.Fatalf("replace not applied %+v", child)
	}
	if len(a.slots) != 1 {
		t.Fatalf("slots=%d want 1", len(a.slots))
	}
}

func TestAccumulatorClosesOnceOnDistinctParents(t *testing.T) {
	t.Parallel()
	a, err := NewDerivedAccumulator("2m")
	if err != nil {
		t.Fatal(err)
	}
	a.OnParent(kOHLC(0, 10, 12, 9, 11, 1), true)
	if a.ClosedParentCount() != 1 {
		t.Fatalf("count=%d", a.ClosedParentCount())
	}
	child, closed, ok := a.OnParent(kOHLC(60_000, 11, 15, 10, 14, 2), true)
	if !ok || !closed {
		t.Fatal("second distinct closed parent must close child")
	}
	if child.Volume != 3 || child.Open != 10 || child.Close != 14 {
		t.Fatalf("%+v", child)
	}
	// Another forming tick on the second parent after close still one slot for that open.
	_, closed2, _ := a.OnParent(kOHLC(60_000, 11, 15, 10, 14, 2), true)
	if !closed2 {
		t.Fatal("same closed parent still complete")
	}
	if a.ClosedParentCount() != 2 {
		t.Fatalf("count=%d want 2 distinct", a.ClosedParentCount())
	}
}

func TestLiveCloseMatchesHistoryFold(t *testing.T) {
	t.Parallel()
	parents := []Kline{
		kOHLC(0, 10, 12, 9, 11, 1),
		kOHLC(60_000, 11, 15, 10, 14, 2),
	}
	folded, _, err := FoldClosedChildren(parents, "2m", false)
	if err != nil || len(folded) != 1 {
		t.Fatalf("fold %v %v", folded, err)
	}
	a, _ := NewDerivedAccumulator("2m")
	a.OnParent(parents[0], true)
	live, closed, ok := a.OnParent(parents[1], true)
	if !ok || !closed {
		t.Fatal("live close")
	}
	f := folded[0]
	if live.OpenTime != f.OpenTime || live.Open != f.Open || live.High != f.High ||
		live.Low != f.Low || live.Close != f.Close || live.Volume != f.Volume ||
		live.CloseTime != f.CloseTime {
		t.Fatalf("live %+v fold %+v", live, f)
	}
}

func TestTenFortyFiveThreeHourBuckets(t *testing.T) {
	t.Parallel()
	// 10m from two 5m: 0 and 5m.
	p10 := []Kline{kOHLC(0, 1, 2, 1, 1.5, 1), kOHLC(5*60_000, 1.5, 3, 1, 2, 1)}
	c, _, err := FoldClosedChildren(p10, "10m", false)
	if err != nil || len(c) != 1 || c[0].OpenTime != 0 {
		t.Fatalf("10m %+v %v", c, err)
	}
	// 45m from three 15m.
	p45 := []Kline{
		kOHLC(0, 1, 1, 1, 1, 1),
		kOHLC(15*60_000, 1, 2, 1, 1, 1),
		kOHLC(30*60_000, 1, 3, 1, 1, 1),
	}
	c, _, err = FoldClosedChildren(p45, "45m", false)
	if err != nil || len(c) != 1 || c[0].High != 3 {
		t.Fatalf("45m %+v %v", c, err)
	}
	// 3h from three 1h.
	hour := int64(60 * 60_000)
	p3h := []Kline{kOHLC(0, 1, 1, 1, 1, 1), kOHLC(hour, 1, 2, 1, 1, 1), kOHLC(2*hour, 1, 4, 0.5, 3, 2)}
	c, _, err = FoldClosedChildren(p3h, "3h", false)
	if err != nil || len(c) != 1 || c[0].High != 4 || c[0].Low != 0.5 || c[0].Volume != 4 {
		t.Fatalf("3h %+v %v", c, err)
	}
}

func TestDerivedNotNativeWS(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"2m", "10m", "45m", "3h"} {
		if IsNativeBinance(id) || ShouldPersist(id) {
			t.Fatalf("%s must not persist/native", id)
		}
		if !IsLiveChartTF(id) || !IsDerivedTime(id) {
			t.Fatalf("%s must be live derived", id)
		}
	}
	joined := CombinedKlineStreamNames("BTCUSDT")
	for _, s := range joined {
		if s == "btcusdt@kline_2m" {
			t.Fatal("2m must not be on WS")
		}
	}
}
