package forecast

import (
	"testing"

	"trading_bot/data"
)

func TestSplitCausalTail_MarketClockNotRowSubtraction(t *testing.T) {
	t.Parallel()
	start := int64(1_699_999_200_000)
	dense := seq15m(t, start, 400)
	sparse := make([]int64, 0, len(dense)/2)
	for i, at := range dense {
		if i%2 == 0 {
			sparse = append(sparse, at)
		}
	}
	last := sparse[len(sparse)-1]
	excl, err := data.NextBarOpen(last, "15m")
	if err != nil {
		t.Fatal(err)
	}
	got, err := SplitCausalTail(sparse, "15m", excl, 20, 72, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.InnerValEnd-got.InnerValBegin == 20 {
		t.Fatal("realized inner ValN must not be forced to spanBars on sparse At[]")
	}
	if got.InnerValEnd-got.InnerValBegin >= 20 {
		t.Fatalf("sparse ValN=%d want < 20", got.InnerValEnd-got.InnerValBegin)
	}
	if got.TailExclusiveEnd != excl {
		t.Fatal("tailExclusiveEnd")
	}
	if got.TrainLastAt != last {
		t.Fatal("TrainLastAt")
	}
	if got.InnerTrainEnd > got.InnerValBegin {
		t.Fatal("overlap")
	}
	for i := got.InnerTrainBegin; i < got.InnerTrainEnd; i++ {
		h, err := HorizonEnd(sparse[i], "15m", 72)
		if err != nil {
			t.Fatal(err)
		}
		if h >= got.InnerValStartAt {
			t.Fatalf("innerTrain horizon leak At=%d", sparse[i])
		}
	}
}

func TestSplitCausalTail_ExclusiveEndIsNextBarOpen(t *testing.T) {
	t.Parallel()
	ats := seq15m(t, int64(1_699_999_200_000), 250)
	last := ats[len(ats)-1]
	excl, err := data.NextBarOpen(last, "15m")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SplitCausalTail(ats, "15m", last, 30, 72, 0, 5); err == nil {
		t.Fatal("want refuse off-grid or lastAt as exclusive end")
	}
	if _, err := SplitCausalTail(ats, "15m", last+1, 30, 72, 0, 5); err == nil {
		t.Fatal("want refuse lastAt+1ms")
	}
	got, err := SplitCausalTail(ats, "15m", excl, 30, 72, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.TailExclusiveEnd != excl {
		t.Fatal(got.TailExclusiveEnd)
	}
}

func TestSplitCausalTail_ValStartMustNotMatter(t *testing.T) {
	t.Parallel()
	ats := seq15m(t, int64(1_699_999_200_000), 250)
	last := ats[200]
	train := ats[:201]
	excl, err := data.NextBarOpen(last, "15m")
	if err != nil {
		t.Fatal(err)
	}
	a, err := SplitCausalTail(train, "15m", excl, 30, 72, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	b, err := SplitCausalTail(train, "15m", excl, 30, 72, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("identical outer train must match")
	}
	_ = ats[230] // later "ValStart" exists in a denser world but is not an argument
}

func TestSplitCausalTail_MinTrainRows(t *testing.T) {
	t.Parallel()
	ats := seq15m(t, int64(1_699_999_200_000), 80)
	excl, err := data.NextBarOpen(ats[len(ats)-1], "15m")
	if err != nil {
		t.Fatal(err)
	}
	_, err = SplitCausalTail(ats, "15m", excl, 30, 72, 0, 35040)
	if err == nil {
		t.Fatal("want INNER_TRAIN_TOO_SMALL")
	}
}
