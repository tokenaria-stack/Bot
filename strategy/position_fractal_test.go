package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestFindPositionalFractal_LongUsesLowestLow(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, 15)
	for i := range klines {
		klines[i] = exchange.Kline{
			OpenTime: int64(i) * 60_000,
			High:     110,
			Low:      100 + float64(i),
			Close:    105,
		}
	}
	klines[7].Low = 90

	got := findPositionalFractal(klines, 14, true)
	want := 90 * fractalSLLongBuffer
	if got != want {
		t.Fatalf("long fractal SL = %v, want %v", got, want)
	}
}

func TestFindPositionalFractal_ShortUsesHighestHigh(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, 15)
	for i := range klines {
		klines[i] = exchange.Kline{
			OpenTime: int64(i) * 60_000,
			High:     120 - float64(i),
			Low:      100,
			Close:    110,
		}
	}
	klines[10].High = 130

	got := findPositionalFractal(klines, 14, false)
	want := 130 * fractalSLShortBuffer
	if got != want {
		t.Fatalf("short fractal SL = %v, want %v", got, want)
	}
}
