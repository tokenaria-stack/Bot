package server

import (
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/exchange"
	"trading_bot/strategy"
)

func TestDagHeaderFromFrame(t *testing.T) {
	t.Parallel()
	frame := &core.TickFrame{}
	frame.Set(core.SlotJurikRSX, 55.5)
	frame.Set(core.SlotJurikSignal, 50.1)
	frame.Set(core.SlotWozduhFast, 40.0)
	frame.Set(core.SlotWozduhSlow, 35.0)

	h := dagHeaderFromFrame(frame)
	if h.Jurik != 55.5 || h.RSX != 55.5 {
		t.Fatalf("jurik %+v", h)
	}
	if h.RSXSignal != 50.1 || h.VolFast != 40 || h.VolSlow != 35 {
		t.Fatalf("woz/signal %+v", h)
	}

	state := &MarketState{Factors: map[string]strategy.ScoreFactor{"x": {}}}
	applyDAGHeaderToMarketState(state, h)
	if state.Jurik != 55.5 {
		t.Fatalf("state.Jurik=%v", state.Jurik)
	}
	if state.RedLine != 0 || state.GreenLine != 0 {
		t.Fatalf("Red/Green must stay empty without DAG price-RSI slots")
	}
}

func TestEnrichFromDAG_ChartOnlyZerosScore(t *testing.T) {
	prev := strategy.GetEngineMode()
	t.Cleanup(func() { strategy.SetEngineMode(prev) })
	strategy.SetEngineMode(strategy.EngineModeChartOnly)

	marker := newTestDAGMarker(80)
	state := &MarketState{Factors: map[string]strategy.ScoreFactor{"legacy": {}}}
	d := &DashboardServer{}
	d.enrichFromDAG(state, marker)

	frame := marker.DAGTickFrame()
	if frame == nil {
		t.Fatal("expected DAG frame")
	}
	wantJurik := frame.Get(core.SlotJurikRSX)
	if state.Jurik != wantJurik {
		t.Fatalf("Jurik=%v want DAG SlotJurikRSX=%v", state.Jurik, wantJurik)
	}
	if state.LongScore != 0 || state.ShortScore != 0 {
		t.Fatalf("ChartOnly scores must be zero, got L=%d S=%d", state.LongScore, state.ShortScore)
	}
	if state.RawAction != "" || state.FinalAction != "" || state.IsVetoed {
		t.Fatal("ChartOnly must not emit action/veto telemetry")
	}
	if state.FibZones != nil {
		t.Fatal("FibZones must be nil")
	}
	if len(state.Factors) != 0 {
		t.Fatalf("Factors must be empty, got %v", state.Factors)
	}
}

func TestEnrichFromDAG_LiveUsesSlotTotalScore(t *testing.T) {
	prev := strategy.GetEngineMode()
	t.Cleanup(func() { strategy.SetEngineMode(prev) })
	strategy.SetEngineMode(strategy.EngineModeLive)

	marker := newTestDAGMarker(80)
	state := &MarketState{}
	d := &DashboardServer{}
	d.enrichFromDAG(state, marker)

	frame := marker.DAGTickFrame()
	total := frame.Get(core.SlotTotalScore)
	want := 0
	if jsonSafeDivState(total) {
		want = int(math.Round(total))
	}
	if state.LongScore != want {
		t.Fatalf("LongScore=%d want SlotTotalScore round=%d (raw=%v)", state.LongScore, want, total)
	}
	if state.ShortScore != 0 || len(state.Factors) != 0 {
		t.Fatal("no ScoreEngine factors/short on Live UI path")
	}
}

func newTestDAGMarker(n int) *strategy.Marker {
	klines := make([]exchange.Kline, n)
	base := int64(1_700_000_000_000)
	for i := range klines {
		ot := base + int64(i)*60_000
		px := 100.0 + float64(i)*0.1
		klines[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + 59_999,
			Open: px, High: px + 1, Low: px - 1, Close: px + 0.5, Volume: 10,
		})
	}
	return strategy.NewMarker(klines, nil, "1m", "test", strategy.ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
}
