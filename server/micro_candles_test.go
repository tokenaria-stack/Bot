package server

import (
	"testing"

	"trading_bot/domain"
)

func TestSynthesizeMicroCandles_seconds(t *testing.T) {
	trades := []domain.AggTrade{
		{Price: 100, Time: 1000},
		{Price: 102, Time: 1500},
		{Price: 101, Time: 2000},
		{Price: 105, Time: 3500},
	}
	candles := SynthesizeMicroCandles(trades, "1s")
	if len(candles) != 3 {
		t.Fatalf("len = %d, want 3", len(candles))
	}
	if candles[0].Open != 100 || candles[0].Close != 102 {
		t.Fatalf("bar0: %+v", candles[0])
	}
	if candles[1].Close != 101 {
		t.Fatalf("bar1: %+v", candles[1])
	}
}

func TestSynthesizeMicroCandles_ticks(t *testing.T) {
	trades := []domain.AggTrade{
		{Price: 10, Time: 1000},
		{Price: 11, Time: 1100},
		{Price: 12, Time: 1200},
		{Price: 13, Time: 1300},
	}
	candles := SynthesizeMicroCandles(trades, "2ticks")
	if len(candles) != 2 {
		t.Fatalf("len = %d, want 2", len(candles))
	}
	if candles[0].Close != 11 || candles[1].Close != 13 {
		t.Fatalf("candles: %+v", candles)
	}
}

func TestLatestMicroCandle(t *testing.T) {
	trades := []domain.AggTrade{{Price: 42, Time: 5000}}
	bar, ok := LatestMicroCandle(trades, "1s")
	if !ok {
		t.Fatal("expected candle")
	}
	if bar.Time != 5 {
		t.Fatalf("Time = %d, want 5 (seconds)", bar.Time)
	}
}
func TestIsOrderFlowTimeframe(t *testing.T) {
	if !IsOrderFlowTimeframe(TimeframeSpec{ID: "15s"}) {
		t.Fatal("15s should be order flow")
	}
	if !IsOrderFlowTimeframe(TimeframeSpec{ID: "100ticks"}) {
		t.Fatal("100ticks should be order flow")
	}
	if IsOrderFlowTimeframe(TimeframeSpec{ID: "10m", Kind: TFRAMOnly}) {
		t.Fatal("10m should not be order flow")
	}
}
