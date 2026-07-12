package server

import (
	"context"
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
		context.Background(),
		klines,
		50,
		strategy.IndicatorWarmupBars,
		strategy.GetRSXSettings(),
		[]string{"line_rsx", "woz_fast"},
		false,
		"1m",
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
	if resp.Annotations == nil {
		t.Fatal("annotations must be non-nil slice")
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
	if _, ok := resp.Plots["line_rsx"]; !ok {
		t.Fatal("expected line_rsx plot")
	}
	if _, ok := resp.Plots["woz_fast"]; !ok {
		t.Fatal("expected woz_fast plot")
	}
}

func TestFilterAnnotationsByDisplayTimes(t *testing.T) {
	t.Parallel()
	times := []int64{100, 200, 300}
	anns := []strategy.ChartAnnotation{
		{Time: 50, Label: "LL", Pane: "rsx"},
		{Time: 200, Label: "SS", Pane: "rsx"},
		{Time: 999, Label: "L", Pane: "rsx"},
	}
	got := filterAnnotationsByDisplayTimes(anns, times)
	if len(got) != 1 {
		t.Fatalf("len = %d want 1", len(got))
	}
	if got[0].Time != 200 || got[0].Label != "SS" {
		t.Fatalf("got %+v", got[0])
	}
}

func TestFilterAnnotationsByDisplayTimes_emptyInputs(t *testing.T) {
	t.Parallel()
	if len(filterAnnotationsByDisplayTimes(nil, []int64{1})) != 0 {
		t.Fatal("expected empty for nil annotations")
	}
	if len(filterAnnotationsByDisplayTimes([]strategy.ChartAnnotation{{Time: 1}}, nil)) != 0 {
		t.Fatal("expected empty for nil times")
	}
}
