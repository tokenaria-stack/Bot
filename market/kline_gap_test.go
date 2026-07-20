package market

import (
	"testing"

	"trading_bot/exchange"
)

func TestKlineTailNeedsGapFill(t *testing.T) {
	t.Parallel()
	const intervalMs = 60_000
	const endMs = 1_700_000_000_000

	// Fresh tip: last open is exactly one interval behind capped end → OK.
	if KlineTailNeedsGapFill(endMs-intervalMs, endMs, intervalMs) {
		t.Fatal("fresh tip (Δ=1×interval) should not need gap fill")
	}
	// P0: one missing closed bar (Δ=2×interval) must need fill (old >2× false-negative).
	if !KlineTailNeedsGapFill(endMs-intervalMs*2, endMs, intervalMs) {
		t.Fatal("one missing bar (Δ=2×interval) should need gap fill")
	}
	if !KlineTailNeedsGapFill(endMs-intervalMs*3, endMs, intervalMs) {
		t.Fatal("three bars behind should need gap fill")
	}
	if KlineTailNeedsGapFill(0, endMs, intervalMs) {
		t.Fatal("zero last open should not trigger")
	}
}

func TestKlineSeriesNeedsGapFill_InternalHole(t *testing.T) {
	t.Parallel()
	const intervalMs = 900_000 // 15m
	const endMs = 1_700_000_000_000
	klines := []exchange.Kline{
		{OpenTime: endMs - intervalMs*400},
		{OpenTime: endMs - intervalMs*100}, // 300-bar hole
	}
	if !KlineSeriesNeedsGapFill(klines, endMs, intervalMs) {
		t.Fatal("expected internal gap detection")
	}

	// P0: exactly one missing bar (ΔOpen = 2×interval) must detect — old >2× missed this.
	oneMissing := []exchange.Kline{
		{OpenTime: endMs - intervalMs*3},
		{OpenTime: endMs - intervalMs}, // skipped endMs-2×interval
	}
	if !KlineSeriesNeedsGapFill(oneMissing, endMs, intervalMs) {
		t.Fatal("one-bar internal hole (Δ=2×interval) must need gap fill")
	}

	continuous := []exchange.Kline{
		{OpenTime: endMs - intervalMs*3},
		{OpenTime: endMs - intervalMs*2},
		{OpenTime: endMs - intervalMs},
	}
	if KlineSeriesNeedsGapFill(continuous, endMs, intervalMs) {
		t.Fatal("continuous series with fresh tip should not need gap fill")
	}
}
