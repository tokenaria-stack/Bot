package indicators_test

import (
	"testing"

	"trading_bot/indicators"
)

func TestCheckClassicDivergence_bearishClassA(t *testing.T) {
	t.Parallel()

	pricePeaks := []indicators.Peak{
		{Index: 10, Value: 100, Type: indicators.PeakHigh},
		{Index: 30, Value: 110, Type: indicators.PeakHigh},
	}
	oscPeaks := []indicators.Peak{
		{Index: 10, Value: 5, Type: indicators.PeakHigh},
		{Index: 30, Value: 3, Type: indicators.PeakHigh},
	}

	result := indicators.CheckClassicDivergence(pricePeaks, oscPeaks, 2, 0, 0)
	if result.Direction != indicators.Bearish {
		t.Fatalf("Direction = %q, want %q", result.Direction, indicators.Bearish)
	}
	if result.Class != indicators.ClassA {
		t.Fatalf("Class = %q, want %q", result.Class, indicators.ClassA)
	}
}

func TestCheckClassicDivergence_bullishClassA(t *testing.T) {
	t.Parallel()

	pricePeaks := []indicators.Peak{
		{Index: 10, Value: 90, Type: indicators.PeakLow},
		{Index: 30, Value: 80, Type: indicators.PeakLow},
	}
	oscPeaks := []indicators.Peak{
		{Index: 10, Value: -5, Type: indicators.PeakLow},
		{Index: 30, Value: -2, Type: indicators.PeakLow},
	}

	result := indicators.CheckClassicDivergence(pricePeaks, oscPeaks, 2, 0, 0)
	if result.Direction != indicators.Bullish {
		t.Fatalf("Direction = %q, want %q", result.Direction, indicators.Bullish)
	}
	if result.Class != indicators.ClassA {
		t.Fatalf("Class = %q, want %q", result.Class, indicators.ClassA)
	}
}

func TestCheckClassicDivergence_noMatch(t *testing.T) {
	t.Parallel()

	result := indicators.CheckClassicDivergence(
		[]indicators.Peak{{Index: 1, Value: 1, Type: indicators.PeakHigh}},
		[]indicators.Peak{{Index: 2, Value: 1, Type: indicators.PeakHigh}},
		1,
		0, 0,
	)
	if result.Direction != indicators.NoDiv || result.Class != indicators.None {
		t.Fatalf("expected empty result, got %+v", result)
	}
}

func TestCheckTripleDivergence_bearish(t *testing.T) {
	t.Parallel()

	pricePeaks := []indicators.Peak{
		{Index: 10, Value: 100, Type: indicators.PeakHigh},
		{Index: 20, Value: 105, Type: indicators.PeakHigh},
		{Index: 30, Value: 110, Type: indicators.PeakHigh},
	}
	oscPeaks := []indicators.Peak{
		{Index: 10, Value: 6, Type: indicators.PeakHigh},
		{Index: 20, Value: 4, Type: indicators.PeakHigh},
		{Index: 30, Value: 2, Type: indicators.PeakHigh},
	}

	result := indicators.CheckTripleDivergence(pricePeaks, oscPeaks, 2)
	if result.Direction != indicators.Bearish || result.Class != indicators.ClassA {
		t.Fatalf("got %+v, want bearish ClassA", result)
	}
	if result.PriceP1.Index != 10 || result.PriceP2.Index != 30 {
		t.Fatalf("expected extreme price peaks P1=10 P3=30, got %+v", result)
	}
}

func TestCheckTripleDivergence_bullish(t *testing.T) {
	t.Parallel()

	pricePeaks := []indicators.Peak{
		{Index: 10, Value: 90, Type: indicators.PeakLow},
		{Index: 20, Value: 85, Type: indicators.PeakLow},
		{Index: 30, Value: 80, Type: indicators.PeakLow},
	}
	oscPeaks := []indicators.Peak{
		{Index: 10, Value: -6, Type: indicators.PeakLow},
		{Index: 20, Value: -4, Type: indicators.PeakLow},
		{Index: 30, Value: -2, Type: indicators.PeakLow},
	}

	result := indicators.CheckTripleDivergence(pricePeaks, oscPeaks, 2)
	if result.Direction != indicators.Bullish || result.Class != indicators.ClassA {
		t.Fatalf("got %+v, want bullish ClassA", result)
	}
}
