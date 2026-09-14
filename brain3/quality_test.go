package brain3

import (
	"math"
	"testing"

	"trading_bot/ml"
)

func TestSoftmax_PTPFirst(t *testing.T) {
	t.Parallel()
	p := ml.Softmax3([3]float64{0, 0, 0})
	if math.Abs(p[0]+p[1]+p[2]-1) > 1e-12 {
		t.Fatal(p)
	}
	if math.Abs(ScorePTP([3]float64{0, 0, 0})-1.0/3.0) > 1e-12 {
		t.Fatal(ScorePTP([3]float64{0, 0, 0}))
	}
	hot := ScorePTP([3]float64{40, 0, 0})
	if hot < 0.999 {
		t.Fatalf("stable softmax got %v", hot)
	}
}

func TestSelectedN_Ceil(t *testing.T) {
	t.Parallel()
	if SelectedN(1, 1083) != 1083 {
		t.Fatal(SelectedN(1, 1083))
	}
	if SelectedN(0.05, 1083) != 55 {
		t.Fatal(SelectedN(0.05, 1083))
	}
	if SelectedN(0.5, 1083) != 542 {
		t.Fatal(SelectedN(0.5, 1083))
	}
}

func TestRankFold_LocalAndTieAt(t *testing.T) {
	t.Parallel()
	rows := []OOFRow{
		{At: 20, Fold: 0, Y: 1, Logit: [3]float64{0, 0, 0}},
		{At: 10, Fold: 0, Y: 0, Logit: [3]float64{0, 0, 0}},
		{At: 30, Fold: 0, Y: 2, Logit: [3]float64{1, 0, 0}},
	}
	got := rankFold(rows)
	if got[0].At != 30 || got[1].At != 10 || got[2].At != 20 {
		t.Fatalf("%v", []int64{got[0].At, got[1].At, got[2].At})
	}
}

func TestPooled_FromSelectedRowsNotMeanPct(t *testing.T) {
	t.Parallel()
	hi := [3]float64{2, 0, 0}
	lo := [3]float64{0, 2, 0}
	f0 := []OOFRow{
		{At: 1, Fold: 0, Y: 0, Logit: hi},
		{At: 2, Fold: 0, Y: 0, Logit: hi},
		{At: 3, Fold: 0, Y: 1, Logit: lo},
		{At: 4, Fold: 0, Y: 1, Logit: lo},
	}
	f1 := []OOFRow{
		{At: 11, Fold: 1, Y: 1, Logit: hi},
		{At: 12, Fold: 1, Y: 1, Logit: hi},
		{At: 13, Fold: 1, Y: 1, Logit: lo},
		{At: 14, Fold: 1, Y: 1, Logit: lo},
	}
	sel, err := pooledAt([][]OOFRow{f0, f1}, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if len(sel) != 4 {
		t.Fatalf("n=%d", len(sel))
	}
	c := countY(ysOf(sel))
	if c.TP != 2 || c.Stop != 2 {
		t.Fatalf("got TP/STOP=%d/%d (must be row-weighted 2/2, not mean of 100%% and 0%%)", c.TP, c.Stop)
	}
	seen := map[int64]struct{}{}
	for _, r := range sel {
		if _, ok := seen[r.At]; ok {
			t.Fatal("dup")
		}
		seen[r.At] = struct{}{}
	}
}

func TestFold100_MatchesCensus(t *testing.T) {
	t.Parallel()
	rows := []OOFRow{
		{At: 1, Y: 0, Logit: [3]float64{0, 0, 0}},
		{At: 2, Y: 1, Logit: [3]float64{1, 0, 0}},
		{At: 3, Y: 1, Logit: [3]float64{0, 1, 0}},
		{At: 4, Y: 2, Logit: [3]float64{0, 0, 1}},
	}
	fc := curveForFold(0, rows)
	if fc.Slices[0].N != 4 || fc.Slices[0].TP != 1 || fc.Slices[0].Stop != 2 || fc.Slices[0].Timeout != 1 {
		t.Fatalf("%+v", fc.Slices[0])
	}
	if fc.Slices[0].TPLiftPP != 0 || fc.Slices[0].StopChgPP != 0 {
		t.Fatal("100% lift")
	}
}
