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

func TestMarker_DataBus_SaveRestore_IntraBarNoGrowth(t *testing.T) {
	t.Parallel()

	klines := syntheticRSXKlines(20)
	m := NewMarker(nil, nil, "1m", "", ChaosConfig{})
	for _, k := range klines {
		m.UpdateKlineTick(k, true)
	}
	m.mu.Lock()
	wantLen := len(m.JurikLines)
	wantLast := m.JurikLines[wantLen-1]
	m.saveLayer2StreamingState()
	m.AppendJurikValue(len(m.klines)-1, wantLast+5)
	if len(m.JurikLines) != wantLen {
		t.Fatalf("open-bar update should not grow series: len %d want %d", len(m.JurikLines), wantLen)
	}
	m.restoreLayer2StreamingState()
	if len(m.JurikLines) != wantLen {
		t.Fatalf("restore len = %d, want %d", len(m.JurikLines), wantLen)
	}
	if m.JurikLines[wantLen-1] != wantLast {
		t.Fatalf("restore should rollback open-bar mutation: got %f want %f", m.JurikLines[wantLen-1], wantLast)
	}
	m.mu.Unlock()
}
