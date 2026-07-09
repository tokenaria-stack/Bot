package strategy

import (
	"testing"

	"trading_bot/core"
	"trading_bot/exchange"
)

func TestDAGShadowParityWithFalcon(t *testing.T) {
	ApplyRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "close"})

	klines := make([]exchange.Kline, 120)
	price := 100.0
	for i := range klines {
		price += 0.05
		if i%7 == 0 {
			price -= 0.2
		}
		klines[i] = exchange.Kline{
			OpenTime: int64((i + 1) * 60_000),
			Open:     price - 0.1,
			High:     price + 0.2,
			Low:      price - 0.2,
			Close:    price,
			Volume:   1000 + float64(i)*10,
		}
	}

	m := NewMarker(klines, nil, "1m", "", ChaosConfig{})
	if m.dag == nil {
		t.Fatal("expected shadow DAG runner")
	}
	bus := m.dag.Bus()
	if bus == nil || bus.Cur == nil {
		t.Fatal("expected DAG bus")
	}

	cur := bus.Cur
	checks := []struct {
		name string
		got  float64
		want float64
	}{
		{"jurik_rsx", cur.Get(core.SlotJurikRSX), m.falconSignals.JurikRSX},
		{"jurik_signal", cur.Get(core.SlotJurikSignal), m.falconSignals.JurikRSXSignal},
		{"woz_fast", cur.Get(core.SlotWozduhFast), m.falconSignals.RsiVolFast},
		{"woz_slow", cur.Get(core.SlotWozduhSlow), m.falconSignals.RsiVolSlow},
	}
	for _, c := range checks {
		if !shadowValuesMatch(c.got, c.want) {
			t.Fatalf("%s drift: dag=%v falcon=%v", c.name, c.got, c.want)
		}
	}
}
