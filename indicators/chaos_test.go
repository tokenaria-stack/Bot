package indicators_test

import (
	"testing"

	"trading_bot/exchange"
	"trading_bot/indicators"
)

func TestAOValues_defaults(t *testing.T) {
	t.Parallel()

	highs := make([]float64, 40)
	lows := make([]float64, 40)
	for i := range highs {
		highs[i] = 10 + float64(i)
		lows[i] = 8 + float64(i)
	}

	ao, err := indicators.AOValues(highs, lows, 0, 0)
	if err != nil {
		t.Fatalf("AOValues: %v", err)
	}
	if len(ao) != len(highs) {
		t.Fatalf("len(ao) = %d, want %d", len(ao), len(highs))
	}
}

func TestAOValuesFromKlines(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, 40)
	for i := range klines {
		klines[i] = exchange.Kline{High: 100 + float64(i), Low: 90 + float64(i)}
	}

	ao, err := indicators.AOValuesFromKlines(klines, 5, 34)
	if err != nil {
		t.Fatalf("AOValuesFromKlines: %v", err)
	}
	if len(ao) != len(klines) {
		t.Fatalf("len(ao) = %d, want %d", len(ao), len(klines))
	}
}

func TestWilliamsFractals_upFractal(t *testing.T) {
	t.Parallel()

	wf := indicators.NewWilliamsFractals()
	highs := []float64{10, 11, 15, 12, 11, 10, 9}
	lows := []float64{8, 9, 10, 9, 8, 7, 6}

	var found bool
	for i := range highs {
		status := wf.UpdateCandle(highs[i], lows[i])
		if status.UpFractal && status.CenterHigh == 15 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected up fractal at center high 15")
	}
}

func TestATRValues(t *testing.T) {
	t.Parallel()

	high := []float64{12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26}
	low := []float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24}
	close := []float64{11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25}

	atr := indicators.ATRValues(high, low, close, 14)
	if len(atr) != len(close) {
		t.Fatalf("len(atr) = %d, want %d", len(atr), len(close))
	}
	if atr[len(atr)-1] <= 0 {
		t.Fatalf("expected positive ATR, got %f", atr[len(atr)-1])
	}
}

func TestBollingerBandsValues(t *testing.T) {
	t.Parallel()

	close := []float64{10, 11, 12, 11, 10, 11, 12, 13, 12, 11, 10, 11, 12, 13, 14, 15, 14, 13, 12, 11}
	upper, middle, lower := indicators.BollingerBandsValues(close, 20, 2, 2)
	if len(upper) != len(close) || len(middle) != len(close) || len(lower) != len(close) {
		t.Fatalf("band lengths mismatch")
	}
	if upper[len(upper)-1] < middle[len(middle)-1] {
		t.Fatal("upper band should be >= middle band")
	}
	if lower[len(lower)-1] > middle[len(middle)-1] {
		t.Fatal("lower band should be <= middle band")
	}
}
