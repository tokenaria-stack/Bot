package market

import (
	"testing"

	"trading_bot/data"
	"trading_bot/exchange"
)

func TestKlineTailNeedsGapFill(t *testing.T) {
	t.Parallel()
	const interval = "1m"
	cap, err := data.CurrentBarOpen(1_700_000_000_000, interval)
	if err != nil {
		t.Fatal(err)
	}
	oneBack, err := data.PreviousBarOpen(cap, interval)
	if err != nil {
		t.Fatal(err)
	}
	twoBack, err := data.PreviousBarOpen(oneBack, interval)
	if err != nil {
		t.Fatal(err)
	}
	threeBack, err := data.PreviousBarOpen(twoBack, interval)
	if err != nil {
		t.Fatal(err)
	}

	if KlineTailNeedsGapFill(oneBack, cap, interval) {
		t.Fatal("exactly 1 step behind Cap must not need gap fill")
	}
	if !KlineTailNeedsGapFill(twoBack, cap, interval) {
		t.Fatal("2 steps behind Cap must need gap fill")
	}
	if !KlineTailNeedsGapFill(threeBack, cap, interval) {
		t.Fatal("3 steps behind Cap must need gap fill")
	}
	if KlineTailNeedsGapFill(0, cap, interval) {
		t.Fatal("zero lastOpen must not panic / false-positive")
	}
}

func TestKlineSeriesNeedsGapFill_InternalHole(t *testing.T) {
	t.Parallel()
	const interval = "15m"
	cap, err := data.CurrentBarOpen(1_700_000_000_000, interval)
	if err != nil {
		t.Fatal(err)
	}
	o1, _ := data.RetreatBarOpen(cap, 400, interval)
	o2, _ := data.RetreatBarOpen(cap, 100, interval)
	klines := []exchange.Kline{{OpenTime: o1}, {OpenTime: o2}}
	if !KlineSeriesNeedsGapFill(klines, cap, interval) {
		t.Fatal("expected internal hole to need gap fill")
	}

	a, _ := data.RetreatBarOpen(cap, 3, interval)
	c, _ := data.RetreatBarOpen(cap, 1, interval)
	oneMissing := []exchange.Kline{{OpenTime: a}, {OpenTime: c}}
	if !KlineSeriesNeedsGapFill(oneMissing, cap, interval) {
		t.Fatal("expected one missing bar to need gap fill")
	}

	b, _ := data.RetreatBarOpen(cap, 2, interval)
	continuous := []exchange.Kline{
		{OpenTime: a},
		{OpenTime: b},
		{OpenTime: c},
	}
	if KlineSeriesNeedsGapFill(continuous, cap, interval) {
		t.Fatal("continuous series 1 behind Cap must be healthy")
	}
}

func TestKlineSeriesNeedsGapFill_WeeklyHole(t *testing.T) {
	t.Parallel()
	jun22 := int64(1782086400000) // 2026-06-22
	jul06 := int64(1783296000000) // 2026-07-06
	jul13 := int64(1783900800000) // 2026-07-13 Cap
	gappy := []exchange.Kline{
		{OpenTime: jun22},
		{OpenTime: jul06},
	}
	if !KlineSeriesNeedsGapFill(gappy, jul13, "1w") {
		t.Fatal("skipped week Jun 29 must need gap fill")
	}
	ok := []exchange.Kline{
		{OpenTime: jun22},
		{OpenTime: 1782691200000}, // 2026-06-29
		{OpenTime: jul06},
		{OpenTime: jul13},
	}
	if KlineSeriesNeedsGapFill(ok, jul13, "1w") {
		t.Fatal("contiguous Monday weeks through Cap must be healthy")
	}
}
