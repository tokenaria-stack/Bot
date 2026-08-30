package market

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

	m := NewFrame(klines, "1m", ChaosConfig{})
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

func TestDAGRunner_NoLegacyScoreChain(t *testing.T) {
	t.Parallel()
	r := newDAGRunner(64, NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"}))
	if r.NodeByName("divergence") != nil {
		t.Fatal("DivergenceNode must not be registered")
	}
	if r.NodeByName("micro_pattern") != nil || r.NodeByName("score") != nil {
		t.Fatal("MicroPatternNode / ScoreNode must not be registered")
	}
	if r.NodeByName("rsx") == nil || r.NodeByName("wozduh") == nil || r.NodeByName("zigzag") == nil {
		t.Fatal("rsx/wozduh/zigzag must remain")
	}
}

func TestReplayClosedBars_LegacyScoreSlotsStayZero(t *testing.T) {
	t.Parallel()
	klines := makeSyntheticKlines(80)
	replay := ReplayClosedBars(klines, NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"}))
	if replay.Hist == nil || replay.Hist.Count() == 0 {
		t.Fatal("expected DAG hist")
	}
	n := replay.Hist.Count()
	for lookback := 1; lookback <= n; lookback++ {
		for _, slot := range []core.Slot{core.SlotDivState, core.SlotDivScore, core.SlotMicroDivScore, core.SlotTotalScore} {
			if v := replay.Hist.Get(slot, lookback); v != 0 {
				t.Fatalf("slot %d lookback %d = %v (must stay unused)", slot, lookback, v)
			}
		}
	}
}
