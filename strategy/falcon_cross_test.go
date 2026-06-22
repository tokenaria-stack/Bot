package strategy_test

import (
	"testing"

	"trading_bot/strategy"
)

func TestDetectVolCross(t *testing.T) {
	t.Parallel()

	if got := strategy.DetectVolCross(40, 50, 55, 48, true); got != "lime" {
		t.Fatalf("bull cross = %q, want lime", got)
	}
	if got := strategy.DetectVolCross(55, 48, 40, 50, true); got != "red" {
		t.Fatalf("bear cross = %q, want red", got)
	}
	if got := strategy.DetectVolCross(40, 50, 45, 48, false); got != "" {
		t.Fatalf("not ready = %q, want empty", got)
	}
}

func TestFalconEngine_VolCross(t *testing.T) {
	t.Parallel()

	engine := strategy.NewFalconEngine()
	high, low, close, volume := 105.0, 95.0, 100.0, 1000.0

	for i := 0; i < 80; i++ {
		if i%20 < 10 {
			close += 1.5
		} else {
			close -= 1.2
		}
		high = close + 2
		low = close - 2
		engine.Evaluate(high, low, close, volume)
	}

	// Warmup complete — further evaluate calls may produce crosses; just ensure no panic.
	sig := engine.Evaluate(high, low, close, volume)
	if sig.RsiVolFast < 0 || sig.RsiVolSlow < 0 {
		t.Fatalf("wt11/wt22 should be non-negative")
	}
}
