package vector_db_test

import (
	"math"
	"testing"

	"trading_bot/exchange"
	"trading_bot/vector_db"
)

func TestVectorizeCandles(t *testing.T) {
	t.Parallel()

	klines := []exchange.Kline{
		{Close: 100},
		{Close: 110},
		{Close: 95},
	}

	vector, err := vector_db.VectorizeCandles(klines)
	if err != nil {
		t.Fatalf("VectorizeCandles: %v", err)
	}

	if len(vector) != 3 {
		t.Fatalf("vector length = %d, want 3", len(vector))
	}

	want := []float32{0, 10, -5}
	for i, got := range vector {
		if math.Abs(float64(got-want[i])) > 1e-5 {
			t.Errorf("vector[%d] = %v, want %v", i, got, want[i])
		}
	}
}

func TestVectorizeCandles_notEnoughData(t *testing.T) {
	t.Parallel()

	_, err := vector_db.VectorizeCandles([]exchange.Kline{{Close: 100}})
	if err == nil {
		t.Fatal("VectorizeCandles: expected error for single kline, got nil")
	}
}

func TestVectorizeReport(t *testing.T) {
	t.Parallel()

	vector := vector_db.VectorizeReport(vector_db.ReportSnapshot{
		JurikValue:      60,
		DivergenceScore: 50,
		Regime:          "EXPANSION",
		FalconRedLine:   35,
		FalconBlueLine:  45,
		FibActive:       true,
	})

	if len(vector) != vector_db.ReportVectorSize {
		t.Fatalf("vector length = %d, want %d", len(vector), vector_db.ReportVectorSize)
	}

	want := []float32{0.6, 0.5, 1.0, 0.35, 0.45, 1.0}
	for i, got := range vector {
		if math.Abs(float64(got-want[i])) > 1e-5 {
			t.Errorf("vector[%d] = %v, want %v", i, got, want[i])
		}
	}
}
