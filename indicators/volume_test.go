package indicators_test

import (
	"math"
	"testing"

	"trading_bot/indicators"
)

func TestAD_accumulates(t *testing.T) {
	t.Parallel()

	ad := indicators.NewAD()
	v1 := ad.UpdateCandle(12, 10, 12) // multiplier = 1.0
	v2 := ad.UpdateCandle(11, 10, 10) // multiplier = 0

	if math.Abs(v1-1.0) > 1e-9 {
		t.Fatalf("first AD = %f, want 1.0", v1)
	}
	if math.Abs(v2-0.0) > 1e-9 {
		t.Fatalf("second AD = %f, want 0.0", v2)
	}
}

func TestAD_flatCandle(t *testing.T) {
	t.Parallel()

	ad := indicators.NewAD()
	v := ad.UpdateCandle(10, 10, 10)
	if v != 0 {
		t.Fatalf("flat candle AD = %f, want 0", v)
	}
}

func TestCumSum(t *testing.T) {
	t.Parallel()

	cs := indicators.NewCumSum()
	if cs.Update(1) != 1 || cs.Update(2) != 3 {
		t.Fatal("unexpected cumsum values")
	}
}

func TestVolumeWeightedEMA(t *testing.T) {
	t.Parallel()

	vw := indicators.NewVolumeWeightedEMA(3)
	v := vw.Update(100, 10)
	if v <= 0 {
		t.Fatalf("expected positive VWEMA, got %f", v)
	}
}

func TestCalcRetracements(t *testing.T) {
	t.Parallel()

	engine := indicators.NewFibonacciEngine()
	zones := engine.CalculatePriceZones(100, 200, 150, 0)

	var half, r618 indicators.FibZone
	for _, z := range zones {
		if z.Type == indicators.Retracement && z.Ratio == 0.5 {
			half = z
		}
		if z.Type == indicators.Retracement && z.Ratio == 0.618 {
			r618 = z
		}
	}
	if math.Abs(half.TargetValue-150) > 1e-9 {
		t.Fatalf("0.5 level = %f, want 150", half.TargetValue)
	}
	if math.Abs(r618.TargetValue-(200-61.8)) > 1e-9 {
		t.Fatalf("0.618 level = %f, want %f", r618.TargetValue, 200-61.8)
	}
}

func TestStochValues(t *testing.T) {
	t.Parallel()

	high := []float64{12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26}
	low := []float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24}
	close := []float64{11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25}

	k, d := indicators.StochValues(high, low, close, 5, 3, 3)
	if len(k) != len(close) || len(d) != len(close) {
		t.Fatalf("stoch lengths mismatch")
	}
	if k[len(k)-1] < 0 || k[len(k)-1] > 100 {
		t.Fatalf("%%K out of range: %f", k[len(k)-1])
	}
}

func TestZigZag_confirmsSwing(t *testing.T) {
	t.Parallel()

	zz := indicators.NewZigZag(14)
	zz.SetSensitivity(0.5)

	highs := []float64{10, 11, 15, 12, 11, 10, 9, 8, 7, 6, 5, 6, 8, 10, 12, 14, 16, 18, 20}
	lows := []float64{8, 9, 10, 9, 8, 7, 6, 5, 4, 3, 2, 3, 5, 7, 9, 11, 13, 15, 17}
	closes := highs

	var confirmed bool
	for i := range highs {
		upd := zz.UpdateCandle(highs[i], lows[i], closes[i], 50)
		if upd.Node.Confirmed && upd.Node.Price > 0 {
			confirmed = true
		}
	}
	if !confirmed {
		t.Fatal("expected at least one confirmed zigzag node")
	}
}
