package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestFindFractalATRStop_Long(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, 15)
	for i := range klines {
		klines[i] = exchange.Kline{
			High: 110,
			Low:  100 + float64(i),
			Close: 105,
		}
	}
	klines[7].Low = 90

	got := findFractalATRStop(klines, 14, true, DefaultFractalStopBars, 2.0, 1.5)
	want := 90 - 3.0
	if got != want {
		t.Fatalf("long fractal ATR SL = %v, want %v", got, want)
	}
}

func TestComputePositionStop_FallsBackWithoutATR(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, 15)
	for i := range klines {
		klines[i] = exchange.Kline{High: 110, Low: 100 + float64(i), Close: 105}
	}
	klines[7].Low = 90

	risk := GetRiskSettings()
	risk.StopLossType = "fractal_atr"
	got := computePositionStop(klines, 14, true, 0, risk)
	want := findPositionalFractal(klines, 14, true)
	if got != want {
		t.Fatalf("fallback stop = %v, want %v", got, want)
	}
}
