package indicators_test

import (
	"testing"

	"trading_bot/indicators"
)

func TestJurikRSX_warmupAndRange(t *testing.T) {
	t.Parallel()

	j := indicators.NewJurikRSX(14)
	const n = 60
	prices := make([]float64, n)
	for i := range prices {
		prices[i] = 100 + float64(i)*0.5
	}

	var last float64
	for i, p := range prices {
		v := j.Update(p)
		if v < 0 || v > 100 {
			t.Fatalf("bar %d: RSX %f out of [0,100]", i, v)
		}
		last = v
	}

	if last <= 50 {
		t.Fatalf("uptrend RSX should be > 50 after warmup, got %f", last)
	}
}

func TestJurikRSXValues(t *testing.T) {
	t.Parallel()

	data := make([]float64, 40)
	for i := range data {
		data[i] = 50 + float64(i%5)
	}

	out := indicators.JurikRSXValues(data, 14)
	if len(out) != len(data) {
		t.Fatalf("len = %d, want %d", len(out), len(data))
	}
	for i, v := range out {
		if v < 0 || v > 100 {
			t.Fatalf("index %d: RSX %f out of range", i, v)
		}
	}
}

func TestJurikRSX_composable(t *testing.T) {
	t.Parallel()

	// RSX can consume output of another indicator (pipeline).
	ema := indicators.NewEMA(3)
	rsx := indicators.NewJurikRSX(14)

	for i := 0; i < 30; i++ {
		val := 100 + float64(i)
		smoothed := ema.Update(val)
		v := rsx.Update(smoothed)
		if v < 0 || v > 100 {
			t.Fatalf("composed RSX %f out of range", v)
		}
	}
}
