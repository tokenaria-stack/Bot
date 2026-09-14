package brain3

import (
	"math"
	"testing"

	"trading_bot/ml"
)

func TestAQ_Identities(t *testing.T) {
	t.Parallel()
	z := [3]float64{0.2, -0.1, 0.4}
	p := ml.Softmax3(z)
	if math.Abs(p[0]+p[1]+p[2]-1) > 1e-12 {
		t.Fatal(p)
	}
	aq := RowAQ(z)
	if !aq.Defined {
		t.Fatal("defined")
	}
	if math.Abs(aq.A-(aq.PTP+aq.PStop)) > 1e-15 {
		t.Fatal(aq)
	}
	if math.Abs(aq.A-(1-aq.PTO)) > 1e-12 {
		t.Fatal(aq.A, 1-aq.PTO)
	}
	if math.Abs(aq.Q-aq.PTP/aq.A) > 1e-15 {
		t.Fatal(aq.Q)
	}
}

func TestAQ_UndefinedNonfinite(t *testing.T) {
	t.Parallel()
	aq := RowAQ([3]float64{math.NaN(), 0, 0})
	if aq.Defined {
		t.Fatalf("%+v", aq)
	}
}

func TestQRank_UndefinedLastTieAt(t *testing.T) {
	t.Parallel()
	rows := []OOFRow{
		{At: 30, Y: 0, Logit: [3]float64{0, 0, 0}},
		{At: 10, Y: 0, Logit: [3]float64{1, 0, 0}},
		{At: 20, Y: 1, Logit: [3]float64{0, 1, 0}},
	}
	score := func(r OOFRow) (float64, bool) {
		if r.At == 30 {
			return 0, false
		}
		return scoreQ(r)
	}
	got := rankFoldBy(rows, score)
	if got[0].At != 10 || got[1].At != 20 || got[2].At != 30 {
		t.Fatalf("%v", []int64{got[0].At, got[1].At, got[2].At})
	}
}

func TestResolvedOnly_NoTimeout(t *testing.T) {
	t.Parallel()
	rows := []OOFRow{{Y: 0}, {Y: 2}, {Y: 1}}
	got := resolvedOnly(rows)
	for _, r := range got {
		if r.Y == 2 {
			t.Fatal("timeout")
		}
	}
	if len(got) != 2 {
		t.Fatal(len(got))
	}
}

func TestBinaryTargetAndPriorLoss(t *testing.T) {
	t.Parallel()
	rows := []OOFRow{
		{Y: 0, Logit: [3]float64{2, 0, 0}},
		{Y: 1, Logit: [3]float64{0, 2, 0}},
	}
	if _, _, err := binaryLogLoss(0.5, 2); err == nil {
		t.Fatal("timeout target")
	}
	ll, n, _, undef, err := meanBinaryLoss(rows)
	if err != nil || n != 2 || undef != 0 || ll <= 0 {
		t.Fatalf("%v %d %v", ll, n, err)
	}
	pr, err := priorBinaryLoss(rows, 0.4)
	if err != nil || pr <= 0 {
		t.Fatal(pr, err)
	}
}

func TestSpearman_Monotone(t *testing.T) {
	t.Parallel()
	x := []float64{1, 2, 3, 4}
	y := []float64{2, 4, 6, 8}
	if math.Abs(spearman(x, y)-1) > 1e-12 {
		t.Fatal(spearman(x, y))
	}
}

func TestDecompPooled_FromRows(t *testing.T) {
	t.Parallel()
	hi := [3]float64{2, 0, 0}
	lo := [3]float64{0, 2, 0}
	f0 := []OOFRow{
		{At: 1, Fold: 0, Y: 0, Logit: hi},
		{At: 2, Fold: 0, Y: 1, Logit: lo},
	}
	f1 := []OOFRow{
		{At: 11, Fold: 1, Y: 0, Logit: hi},
		{At: 12, Fold: 1, Y: 1, Logit: lo},
	}
	sel, err := pooledBy([][]OOFRow{f0, f1}, 0.5, scoreQ)
	if err != nil || len(sel) != 2 {
		t.Fatal(len(sel), err)
	}
	c := countY(ysOf(sel))
	if c.TP != 2 || c.Stop != 0 || c.Timeout != 0 {
		t.Fatalf("%+v", c)
	}
}

func TestOuterTrainPrior_UsesTrainYOnly(t *testing.T) {
	t.Parallel()
	ds := Dataset{Y: []int{0, 0, 1, 1, 2, 0}}
	cen := Census{Folds: []FoldRows{
		{TrainBegin: 0, TrainEnd: 4},
		{TrainBegin: 0, TrainEnd: 5},
		{TrainBegin: 0, TrainEnd: 4},
		{TrainBegin: 2, TrainEnd: 6},
	}}
	p, cs, err := OuterTrainResolvedPriors(ds, cen)
	if err != nil {
		t.Fatal(err)
	}
	if p[0] != 0.5 || cs[0].TP != 2 || cs[0].Stop != 2 {
		t.Fatal(p[0], cs[0])
	}
}
