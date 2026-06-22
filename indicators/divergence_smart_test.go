package indicators_test

import (
	"strings"
	"testing"

	"trading_bot/indicators"
)

func TestSmartDivergenceEngine_bearishMacro(t *testing.T) {
	t.Parallel()

	engine := indicators.NewSmartDivergenceEngine()
	engine.UpdateSnapshot(indicators.Snapshot{
		Index: 10, IsHigh: true, Price: 100,
		Jurik: 70, OrangeRSI: 65, BlueVolume: 50, BurgundyAD: 10, AO: 0.5, MACD: 0.3,
	})
	engine.UpdateSnapshot(indicators.Snapshot{
		Index: 30, IsHigh: true, Price: 110,
		Jurik: 60, OrangeRSI: 55, BlueVolume: 40, BurgundyAD: 5, AO: 0.1, MACD: 0.0,
	})

	sig := engine.AnalyzeMacro()
	if sig.Score >= 0 {
		t.Fatalf("expected bearish negative score, got %d", sig.Score)
	}
	if !strings.Contains(sig.Description, "Bearish") {
		t.Fatalf("description = %q", sig.Description)
	}
}

func TestSmartDivergenceEngine_bullishMacro(t *testing.T) {
	t.Parallel()

	engine := indicators.NewSmartDivergenceEngine()
	engine.UpdateSnapshot(indicators.Snapshot{
		Index: 10, IsHigh: false, Price: 90,
		Jurik: 30, OrangeRSI: 35, AO: -0.5, MACD: -0.2,
	})
	engine.UpdateSnapshot(indicators.Snapshot{
		Index: 30, IsHigh: false, Price: 80,
		Jurik: 40, OrangeRSI: 45, AO: -0.1, MACD: 0.1,
	})

	sig := engine.AnalyzeMacro()
	if sig.Score <= 0 {
		t.Fatalf("expected bullish positive score, got %d", sig.Score)
	}
}

func TestSmartDivergenceEngine_hiddenContinuation(t *testing.T) {
	t.Parallel()

	engine := indicators.NewSmartDivergenceEngine()
	engine.UpdateSnapshot(indicators.Snapshot{
		Index: 10, IsHigh: false, Price: 90,
		Jurik: 50, OrangeRSI: 45, RedRSI: 40, MACD: 0.2,
	})
	engine.UpdateSnapshot(indicators.Snapshot{
		Index: 30, IsHigh: false, Price: 95,
		Jurik: 40, OrangeRSI: 35, RedRSI: 30, MACD: 0.0,
	})

	sig := engine.AnalyzeMacro()
	if !strings.Contains(sig.Description, "Hidden Bullish") {
		t.Fatalf("description = %q, want hidden continuation", sig.Description)
	}
}

func TestAnalyzeMicro_saucer(t *testing.T) {
	t.Parallel()

	engine := indicators.NewSmartDivergenceEngine()
	score := engine.AnalyzeMicro(25, 26, 29)
	if score < saucerScore {
		t.Fatalf("expected saucer score >= %d, got %d", saucerScore, score)
	}
}

func TestAnalyzeMicro_vSpike(t *testing.T) {
	t.Parallel()

	engine := indicators.NewSmartDivergenceEngine()
	score := engine.AnalyzeMicro(35, 20, 28)
	if score < vSpikeScore {
		t.Fatalf("expected v-spike score >= %d, got %d", vSpikeScore, score)
	}
}

func TestAnalyzeMicroCombined(t *testing.T) {
	t.Parallel()

	engine := indicators.NewSmartDivergenceEngine()
	engine.UpdateMicroTick(29, 29)
	engine.UpdateMicroTick(26, 26)
	engine.UpdateMicroTick(25, 25)

	score := engine.AnalyzeMicroCombined()
	if score <= 0 {
		t.Fatalf("expected positive micro score, got %d", score)
	}
}

const saucerScore = 15
const vSpikeScore = 20
