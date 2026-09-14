package brain3

import (
	"strings"
	"testing"

	"trading_bot/data"
)

func TestSpec1_IdentityStable(t *testing.T) {
	t.Parallel()
	s := Spec1()
	if err := s.validate(); err != nil {
		t.Fatal(err)
	}
	ds := Dataset{ContentHex: "a", FeatureNames: []string{"x"}, Width: 1, ClassSchema: SetupClassSchema()}
	val := Validation{PlanHex: "p", SplitHex: "s"}
	a, err := s.DigestHex(ds, val)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.DigestHex(ds, val)
	if err != nil || a != b {
		t.Fatal("digest")
	}
}

func TestPreflightInner_RefusesTooSmall(t *testing.T) {
	t.Parallel()
	start := int64(1_699_999_200_000)
	n := 80
	ats := make([]int64, n)
	at := start
	var err error
	for i := 0; i < n; i++ {
		ats[i] = at
		at, err = data.NextBarOpen(at, "15m")
		if err != nil {
			t.Fatal(err)
		}
	}
	ds := Dataset{At: ats, Y: make([]int, n), X: make([][]float64, n), Width: 1, FeatureNames: []string{"a"}, ClassSchema: SetupClassSchema()}
	for i := range ds.X {
		ds.X[i] = []float64{0}
	}
	cen := Census{Folds: []FoldRows{{
		TrainBegin: 0, TrainEnd: n, ValBegin: n - 10, ValEnd: n,
		TrainFirstAt: ats[0], TrainLastAt: ats[n-1], ValFirstAt: ats[n-10], ValLastAt: ats[n-1],
		TrainN: n, ValN: 10,
	}}}
	_, err = PreflightInnerGeometry(ds, cen, Spec1())
	if err == nil || !strings.Contains(err.Error(), "SPEC1_FAILED_TO_COMPILE") {
		t.Fatalf("got %v", err)
	}
}
