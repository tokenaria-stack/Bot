package server

import (
	"context"
	"testing"
	"time"

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
		ot := base + int64(i)*60_000
		klines[i] = exchange.Kline{
			OpenTime:  ot,
			CloseTime: ot + 59_999, // closed relative to wall clock in 2023
			Open:      price,
			High:      price + 10,
			Low:       price - 10,
			Close:     price + 5,
			Volume:    100,
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

func TestDropFormingTip(t *testing.T) {
	t.Parallel()

	base := int64(1_700_000_000_000)
	step := int64(60_000)
	mk := func(i int, closeOffset int64) exchange.Kline {
		ot := base + int64(i)*step
		return exchange.Kline{
			OpenTime:  ot,
			CloseTime: ot + closeOffset,
			Open:      1,
			High:      2,
			Low:       1,
			Close:     1.5,
			Volume:    10,
		}
	}

	closed := []exchange.Kline{mk(0, 59_999), mk(1, 59_999), mk(2, 59_999)}
	nowPastTip := closed[2].CloseTime + 1
	got := dropFormingTip(closed, nowPastTip)
	if len(got) != 3 {
		t.Fatalf("closed tip: len=%d want 3", len(got))
	}

	forming := []exchange.Kline{mk(0, 59_999), mk(1, 59_999), mk(2, 59_999)}
	nowInsideTip := forming[2].OpenTime + 30_000 // before CloseTime
	got = dropFormingTip(forming, nowInsideTip)
	if len(got) != 2 {
		t.Fatalf("forming tip: len=%d want 2", len(got))
	}
	if got[1].OpenTime != forming[1].OpenTime {
		t.Fatalf("expected last closed bar kept, got OpenTime=%d", got[1].OpenTime)
	}

	if dropFormingTip(nil, nowInsideTip) != nil {
		t.Fatal("nil input should stay nil")
	}
	solo := []exchange.Kline{mk(0, 59_999)}
	if dropFormingTip(solo, solo[0].OpenTime) != nil {
		t.Fatal("sole forming bar must yield nil/empty")
	}

	// CloseTime==0 → cannot assert forming; do not drop (safe for incomplete records).
	unknown := []exchange.Kline{{OpenTime: base, CloseTime: 0, Close: 1}}
	if len(dropFormingTip(unknown, base+1)) != 1 {
		t.Fatal("CloseTime=0 must not drop")
	}
}

func TestBuildColumnarHistoryPayload_stripsFormingTip(t *testing.T) {
	t.Parallel()

	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	d := &DashboardServer{projector: wire.NewProjector(reg)}

	nBars := strategy.IndicatorWarmupBars + 10
	klines := make([]exchange.Kline, nBars)
	// Place the series so the last bar is still open relative to wall clock.
	nowMs := time.Now().UnixMilli()
	step := int64(60_000)
	firstOpen := nowMs - int64(nBars-1)*step
	for i := range klines {
		ot := firstOpen + int64(i)*step
		klines[i] = exchange.Kline{
			OpenTime:  ot,
			CloseTime: ot + step - 1,
			Open:      100 + float64(i),
			High:      110 + float64(i),
			Low:       90 + float64(i),
			Close:     105 + float64(i),
			Volume:    10,
		}
	}
	formingOpen := klines[len(klines)-1].OpenTime

	resp, ok := d.buildColumnarHistoryPayload(
		context.Background(),
		klines,
		10,
		strategy.IndicatorWarmupBars,
		strategy.GetRSXSettings(),
		[]string{"line_rsx"},
		false,
		"1m",
		"1m",
	)
	if !ok {
		t.Fatal("expected payload ok after tip strip")
	}
	if resp.Added != 10 {
		t.Fatalf("added = %d want 10 (display after warmup+strip)", resp.Added)
	}
	lastOpen := resp.Times[len(resp.Times)-1] * 1000
	if lastOpen >= formingOpen {
		t.Fatalf("history tip OpenTime ms=%d must be before forming OpenTime=%d", lastOpen, formingOpen)
	}
}

func TestFilterAnnotationsByDisplayTimes(t *testing.T) {
	t.Parallel()
	times := []int64{100, 200, 300}
	anns := []wire.Annotation{
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
	if len(filterAnnotationsByDisplayTimes([]wire.Annotation{{Time: 1}}, nil)) != 0 {
		t.Fatal("expected empty for nil times")
	}
}
