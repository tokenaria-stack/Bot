package forecast

import "testing"

func TestLatestCausalPriceSwing_NoLookaheadK1(t *testing.T) {
	t.Parallel()
	bars := make([]CanonicalClosedBar, 10)
	for i := range bars {
		bars[i] = CanonicalClosedBar{OpenTime: int64(i+1) * 1000, High: 10, Low: 9, Close: 9.5}
	}
	bars[4].Low = 5 // unique low at t=4, confirms at t=5 for k=1
	bars[4].High = 8
	// Before confirm (entry at t=4): must not use the low.
	got, err := LatestCausalPriceSwing(bars, bars[4].OpenTime, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Found {
		t.Fatalf("k=1 must not use unconfirmed low at entry: %+v", got)
	}
	got, err = LatestCausalPriceSwing(bars, bars[5].OpenTime, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || got.AnchorAt != bars[4].OpenTime || got.Wick != 5 || got.ConfirmedAt != bars[5].OpenTime {
		t.Fatalf("k=1 after right bar: %+v", got)
	}
	bars[7].Low = 4
	got, err = LatestCausalPriceSwing(bars, bars[8].OpenTime, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.AnchorAt != bars[7].OpenTime || got.Wick != 4 {
		t.Fatalf("must prefer later swing: %+v", got)
	}
}

func TestLatestCausalPriceSwing_K2NeedsTwoRightBars(t *testing.T) {
	t.Parallel()
	bars := make([]CanonicalClosedBar, 12)
	for i := range bars {
		bars[i] = CanonicalClosedBar{OpenTime: int64(i+1) * 1000, High: 10, Low: 9, Close: 9.5}
	}
	bars[5].Low = 5
	got, err := LatestCausalPriceSwing(bars, bars[6].OpenTime, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Found {
		t.Fatal("k=2 must wait for two right bars")
	}
	got, err = LatestCausalPriceSwing(bars, bars[7].OpenTime, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || got.AnchorAt != bars[5].OpenTime || got.ConfirmedAt != bars[7].OpenTime {
		t.Fatalf("%+v", got)
	}
}

func TestLatestCausalPriceSwing_ShortMirrors(t *testing.T) {
	t.Parallel()
	bars := make([]CanonicalClosedBar, 8)
	for i := range bars {
		bars[i] = CanonicalClosedBar{OpenTime: int64(i+1) * 1000, High: 10, Low: 9, Close: 9.5}
	}
	bars[3].High = 15
	got, err := LatestCausalPriceSwing(bars, bars[4].OpenTime, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || got.Wick != 15 || got.AnchorAt != bars[3].OpenTime {
		t.Fatalf("%+v", got)
	}
}
