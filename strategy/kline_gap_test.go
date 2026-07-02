package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestKlineTailNeedsGapFill(t *testing.T) {
	t.Parallel()
	const intervalMs = 60_000
	const endMs = 1_700_000_000_000

	if KlineTailNeedsGapFill(endMs-intervalMs, endMs, intervalMs) {
		t.Fatal("one bar behind should not need gap fill")
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
	continuous := []exchange.Kline{
		{OpenTime: endMs - intervalMs*3},
		{OpenTime: endMs - intervalMs*2},
		{OpenTime: endMs - intervalMs},
	}
	if KlineSeriesNeedsGapFill(continuous, endMs, intervalMs) {
		t.Fatal("continuous series with fresh tail should not need gap fill")
	}
}
