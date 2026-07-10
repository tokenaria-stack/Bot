package server

import (
	"testing"

	"trading_bot/exchange"
	"trading_bot/server/wire"
	"trading_bot/strategy"
	"trading_bot/ui_config"
)

func TestParseSlotsParam(t *testing.T) {
	t.Parallel()
	if got := parseSlotsParam(""); got != nil {
		t.Fatalf("empty: got %v", got)
	}
	if got := parseSlotsParam("line_rsx, score_total"); len(got) != 2 || got[0] != "line_rsx" || got[1] != "score_total" {
		t.Fatalf("csv: got %v", got)
	}
}

func TestColumnarLenInvariant(t *testing.T) {
	t.Parallel()
	times := []int64{1, 2, 3}
	candles := columnarCandles{
		Open:   []float64{1, 2, 3},
		High:   []float64{1, 2, 3},
		Low:    []float64{1, 2, 3},
		Close:  []float64{1, 2, 3},
		Volume: []float64{1, 2, 3},
	}
	plots := map[string][]float64{"line_rsx": {1, 2, 3}}
	if !columnarLenInvariant(times, candles, plots) {
		t.Fatal("expected invariant true")
	}
	plots["line_rsx"] = []float64{1, 2}
	if columnarLenInvariant(times, candles, plots) {
		t.Fatal("expected invariant false on plot mismatch")
	}
}

func TestBuildColumnarHistoryPayload_lenInvariant(t *testing.T) {
	t.Parallel()

	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	d := &DashboardServer{projector: wire.NewProjector(reg)}

	klines := make([]exchange.Kline, strategy.IndicatorWarmupBars+50)
	base := int64(1_700_000_000_000)
	for i := range klines {
		price := 50000.0 + float64(i)
		klines[i] = exchange.Kline{
			OpenTime: base + int64(i)*60_000,
			Open:     price,
			High:     price + 10,
			Low:      price - 10,
			Close:    price + 5,
			Volume:   100,
		}
	}

	resp, ok := d.buildColumnarHistoryPayload(
		klines,
		50,
		strategy.IndicatorWarmupBars,
		strategy.GetRSXSettings(),
		[]string{"line_rsx", "score_total"},
		false,
		"1m",
	)
	if !ok {
		t.Fatal("expected payload ok")
	}
	if resp.Format != "columnar" {
		t.Fatalf("format = %q", resp.Format)
	}
	if resp.Added != 50 {
		t.Fatalf("added = %d want 50", resp.Added)
	}
	if resp.WarmupDropped != strategy.IndicatorWarmupBars {
		t.Fatalf("warmupDropped = %d want %d", resp.WarmupDropped, strategy.IndicatorWarmupBars)
	}
	if len(resp.Annotations) != 0 {
		t.Fatalf("annotations should be empty slice, got %d", len(resp.Annotations))
	}
	n := len(resp.Times)
	if n != 50 {
		t.Fatalf("times len = %d", n)
	}
	if len(resp.Candles.Volume) != n || len(resp.Candles.Open) != n {
		t.Fatal("candles column length mismatch")
	}
	for id, col := range resp.Plots {
		if len(col) != n {
			t.Fatalf("plot %s len %d want %d", id, len(col), n)
		}
	}
	if len(resp.Plots) != 2 {
		t.Fatalf("plots count = %d want 2", len(resp.Plots))
	}
}
