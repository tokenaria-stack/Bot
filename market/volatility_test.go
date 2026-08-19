package market_test

import (
	"testing"

	"trading_bot/market"
)

func TestVolatilityEngine_warmupSafe(t *testing.T) {
	t.Parallel()

	engine := market.NewVolatilityEngine()
	state := engine.Evaluate(101, 99, 100, 1000, 50)

	if state.Regime != market.RegimeExpansion {
		t.Fatalf("warmup regime = %q, want EXPANSION", state.Regime)
	}
	if state.LotModifier != 1.0 {
		t.Fatalf("LotModifier = %f, want 1.0", state.LotModifier)
	}
}

func TestVolatilityEngine_squeeze(t *testing.T) {
	t.Parallel()

	engine := market.NewVolatilityEngine()

	for i := 0; i < 25; i++ {
		engine.Evaluate(102, 98, 100, 1000, 50)
	}
	var state market.VolatilityState
	for i := 0; i < 50; i++ {
		state = engine.Evaluate(100.01, 99.99, 100, 1000, 50)
	}

	if state.ATR <= 0 {
		t.Fatal("expected positive ATR after warmup")
	}
	if state.Regime != market.RegimeSqueeze {
		t.Fatalf("regime = %q, want SQUEEZE", state.Regime)
	}
	if state.LotModifier != 1.2 {
		t.Fatalf("LotModifier = %f, want 1.2", state.LotModifier)
	}
}

func TestVolatilityEngine_climax(t *testing.T) {
	t.Parallel()

	engine := market.NewVolatilityEngine()

	var state market.VolatilityState
	for i := 0; i < 25; i++ {
		state = engine.Evaluate(100.5, 99.5, 100, 1000, 50)
	}

	state = engine.Evaluate(110, 90, 105, 5000, 85)
	if state.Regime != market.RegimeClimax {
		t.Fatalf("regime = %q, want CLIMAX", state.Regime)
	}
	if state.LotModifier != 0.3 {
		t.Fatalf("LotModifier = %f, want 0.3", state.LotModifier)
	}
	if state.SafeStopDist <= state.ATR {
		t.Fatalf("climax stop should be wider than ATR: stop=%f atr=%f", state.SafeStopDist, state.ATR)
	}
}

func TestVolatilityEngine_expansion(t *testing.T) {
	t.Parallel()

	engine := market.NewVolatilityEngine()

	var state market.VolatilityState
	for i := 0; i < 25; i++ {
		spread := 1.0 + float64(i%3)*0.2
		state = engine.Evaluate(100+spread, 100-spread, 100, 1000+float64(i*10), 55)
	}

	if state.Regime != market.RegimeExpansion {
		t.Fatalf("regime = %q, want EXPANSION", state.Regime)
	}
	if state.SafeStopDist != state.ATR*1.5 {
		t.Fatalf("SafeStopDist = %f, want %f", state.SafeStopDist, state.ATR*1.5)
	}
}
