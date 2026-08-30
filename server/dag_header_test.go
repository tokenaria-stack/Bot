package server

import (
	"testing"

	"trading_bot/core"
	"trading_bot/decision"
	"trading_bot/exchange"
	"trading_bot/market"
	"trading_bot/server/wire"
	"trading_bot/ui_config"
)

func TestDagHeaderFromFrame(t *testing.T) {
	t.Parallel()
	frame := &core.TickFrame{}
	frame.Set(core.SlotJurikRSX, 55.5)
	frame.Set(core.SlotJurikSignal, 50.1)
	frame.Set(core.SlotWozduhFast, 40.0)

	h := dagHeaderFromFrame(frame)
	if h.Jurik != 55.5 || h.RSX != 55.5 {
		t.Fatalf("jurik %+v", h)
	}
	if h.RSXSignal != 50.1 {
		t.Fatalf("signal %+v", h)
	}

	state := &MarketState{Factors: map[string]decision.ScoreFactor{"x": {}}}
	applyDAGHeaderToMarketState(state, h)
	if state.Jurik != 55.5 {
		t.Fatalf("state.Jurik=%v", state.Jurik)
	}
}

func TestEnrichFromDAG_ChartOnlyZerosScore(t *testing.T) {
	prev := market.GetEngineMode()
	t.Cleanup(func() { market.SetEngineMode(prev) })
	market.SetEngineMode(market.EngineModeChartOnly)

	marker := newTestDAGMarker(80)
	state := &MarketState{Factors: map[string]decision.ScoreFactor{"legacy": {}}}
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	d := &DashboardServer{projector: wire.NewProjector(reg)}
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
	if len(state.Plots) == 0 {
		t.Fatal("expected tip plots from projector")
	}
	if _, ok := state.Plots["line_rsx"]; !ok {
		t.Fatalf("expected line_rsx in plots, got %v", state.Plots)
	}
}

func TestEnrichFromDAG_LiveLongScoreStaysZero(t *testing.T) {
	prev := market.GetEngineMode()
	t.Cleanup(func() { market.SetEngineMode(prev) })
	market.SetEngineMode(market.EngineModeLive)

	marker := newTestDAGMarker(80)
	state := &MarketState{}
	d := &DashboardServer{}
	d.enrichFromDAG(state, marker)
	if state.LongScore != 0 {
		t.Fatalf("LongScore=%d want 0", state.LongScore)
	}
	if state.ShortScore != 0 {
		t.Fatal("ShortScore must stay 0")
	}
}

func newTestDAGMarker(n int) *market.Frame {
	klines := make([]exchange.Kline, n)
	base := int64(1_700_000_000_000)
	for i := range klines {
		ot := base + int64(i)*60_000
		px := 100.0 + float64(i)*0.1
		klines[i] = exchange.Kline{
			OpenTime: ot, CloseTime: ot + 59_999,
			Open: px, High: px + 1, Low: px - 1, Close: px + 0.5, Volume: 10,
		}
	}
	return market.NewFrame(klines, "1m", market.ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
}
