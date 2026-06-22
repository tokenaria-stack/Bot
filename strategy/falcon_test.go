package strategy_test

import (
	"testing"

	"trading_bot/strategy"
)

func TestFalconEngine_Evaluate(t *testing.T) {
	t.Parallel()

	engine := strategy.NewFalconEngine()
	high, low, close, volume := 105.0, 95.0, 100.0, 1000.0

	for i := 0; i < 40; i++ {
		close += 0.5
		high = close + 2
		low = close - 2

		sig := engine.Evaluate(high, low, close, volume)
		if sig.JurikRSX < 0 || sig.JurikRSX > 100 {
			t.Fatalf("JurikRSX %f out of [0,100]", sig.JurikRSX)
		}
		if sig.RedLine < 0 || sig.RedLine > 100 {
			t.Fatalf("RedLine %f out of [0,100]", sig.RedLine)
		}
		if sig.GreenLine < 0 || sig.GreenLine > 100 {
			t.Fatalf("GreenLine %f out of [0,100]", sig.GreenLine)
		}
	}
}
