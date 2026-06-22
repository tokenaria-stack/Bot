package indicators_test

import (
	"testing"

	"trading_bot/indicators"
)

func TestTrendline_ValueAt(t *testing.T) {
	t.Parallel()

	p1 := indicators.Peak{Index: 0, Value: 100, Type: indicators.PeakHigh}
	p2 := indicators.Peak{Index: 10, Value: 110, Type: indicators.PeakHigh}
	tl := indicators.NewTrendline(p1, p2, true)

	m, b := tl.Equation()
	if m != 1.0 {
		t.Fatalf("m = %f, want 1.0", m)
	}
	if b != 100 {
		t.Fatalf("b = %f, want 100", b)
	}
	if got := tl.ValueAt(5); got != 105 {
		t.Fatalf("ValueAt(5) = %f, want 105", got)
	}
}

func TestTrendline_UpdateTouches_resistance(t *testing.T) {
	t.Parallel()

	p1 := indicators.Peak{Index: 0, Value: 100, Type: indicators.PeakHigh}
	p2 := indicators.Peak{Index: 10, Value: 100, Type: indicators.PeakHigh}
	tl := indicators.NewTrendline(p1, p2, true)

	tl.UpdateTouches(5, 99.8, 99.0, 1.0)
	if tl.Touches != 1 {
		t.Fatalf("Touches = %d, want 1", tl.Touches)
	}

	tl.UpdateTouches(6, 101.0, 99.5, 1.0)
	if tl.Touches != 1 {
		t.Fatalf("breakout touch should not increment, Touches = %d", tl.Touches)
	}
}

func TestTrendline_CheckBreakout_resistance(t *testing.T) {
	t.Parallel()

	p1 := indicators.Peak{Index: 0, Value: 100, Type: indicators.PeakHigh}
	p2 := indicators.Peak{Index: 10, Value: 100, Type: indicators.PeakHigh}
	tl := indicators.NewTrendline(p1, p2, true)
	tl.Touches = 2

	ok, score := tl.CheckBreakout(11, 101, 100, 200, 100, true)
	if !ok {
		t.Fatal("expected resistance breakout")
	}
	if score != 3 {
		t.Fatalf("score = %d, want 3 (1 + Touches)", score)
	}

	ok, _ = tl.CheckBreakout(11, 101, 100, 100, 100, true)
	if ok {
		t.Fatal("expected no breakout without volume spike")
	}
}

func TestDetectTriangle_symmetrical(t *testing.T) {
	t.Parallel()

	res := indicators.NewTrendline(
		indicators.Peak{Index: 0, Value: 120, Type: indicators.PeakHigh},
		indicators.Peak{Index: 20, Value: 110, Type: indicators.PeakHigh},
		true,
	)
	sup := indicators.NewTrendline(
		indicators.Peak{Index: 0, Value: 90, Type: indicators.PeakLow},
		indicators.Peak{Index: 20, Value: 100, Type: indicators.PeakLow},
		false,
	)

	kind := indicators.DetectTriangle(res, sup)
	if kind != "symmetrical" {
		t.Fatalf("DetectTriangle = %q, want symmetrical", kind)
	}
}

func TestDetectTriangle_ascending(t *testing.T) {
	t.Parallel()

	res := indicators.NewTrendline(
		indicators.Peak{Index: 0, Value: 110, Type: indicators.PeakHigh},
		indicators.Peak{Index: 20, Value: 110, Type: indicators.PeakHigh},
		true,
	)
	sup := indicators.NewTrendline(
		indicators.Peak{Index: 0, Value: 90, Type: indicators.PeakLow},
		indicators.Peak{Index: 20, Value: 100, Type: indicators.PeakLow},
		false,
	)

	kind := indicators.DetectTriangle(res, sup)
	if kind != "ascending" {
		t.Fatalf("DetectTriangle = %q, want ascending", kind)
	}
}
