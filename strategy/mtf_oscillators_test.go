package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestEvaluateHTFOscillators(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, 80)
	for i := range klines {
		base := float64(100 + i)
		klines[i] = exchange.Kline{
			Open:   base,
			High:   base + 2,
			Low:    base - 1,
			Close:  base + 1,
			Volume: 10,
		}
	}

	rsx, color, up, down := evaluateHTFOscillators(klines)
	if rsx <= 0 {
		t.Fatalf("RSXValue = %v, want > 0", rsx)
	}
	if color != "green" && color != "red" && color != "neutral" {
		t.Fatalf("RSXColor = %q, want green/red/neutral", color)
	}
	if up <= 0 || down <= 0 {
		t.Fatalf("Wozduh lines = %v / %v, want positive", up, down)
	}
}

func TestRsxColorLabel(t *testing.T) {
	t.Parallel()

	if got := rsxColorLabel(RSXColorGreen); got != "green" {
		t.Fatalf("got %q, want green", got)
	}
	if got := rsxColorLabel(RSXColorRed); got != "red" {
		t.Fatalf("got %q, want red", got)
	}
	if got := rsxColorLabel(RSXColorNeutral); got != "neutral" {
		t.Fatalf("got %q, want neutral", got)
	}
}
