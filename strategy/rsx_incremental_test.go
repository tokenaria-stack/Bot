package strategy

import (
	"math"
	"testing"

	"trading_bot/exchange"
)

func syntheticRSXKlines(n int) []exchange.Kline {
	klines := make([]exchange.Kline, n)
	price := 100.0
	for i := range klines {
		wave := math.Sin(float64(i)*0.25) * 5
		klines[i] = exchange.Kline{
			OpenTime: int64(i) * 60_000,
			Open:     price + wave,
			High:     price + wave + 2,
			Low:      price + wave - 2,
			Close:    price + wave + 0.5,
			Volume:   1000,
		}
		price += 0.05
	}
	return klines
}

func TestRSXMarkerState_SaveRestore_IntraBarNoGrowth(t *testing.T) {
	t.Parallel()

	s := newRSXMarkerState(14)
	for i := 0; i < 20; i++ {
		s.appendBar(100+float64(i), 99, 100, 50+float64(i)*0.1)
		s.SaveState()
	}
	wantLen := len(s.rsx)

	const intraTicks = 10
	for i := 0; i < intraTicks; i++ {
		s.appendBar(120, 115, 118, 55+float64(i)*0.01)
	}
	if len(s.rsx) != wantLen+intraTicks {
		t.Fatalf("poisoned growth: len %d, want %d before restore", len(s.rsx), wantLen+intraTicks)
	}

	s.RestoreState()
	if len(s.rsx) != wantLen {
		t.Fatalf("restore len = %d, want %d", len(s.rsx), wantLen)
	}

	s.appendBar(120, 115, 118, 55)
	if len(s.rsx) != wantLen+1 {
		t.Fatalf("after restore+append len = %d, want %d", len(s.rsx), wantLen+1)
	}
}
