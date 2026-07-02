package strategy

import (
	"testing"
	"time"

	"trading_bot/exchange"
)

func syntheticKlines(n int) []exchange.Kline {
	out := make([]exchange.Kline, n)
	start := int64(1_700_000_000_000)
	step := int64(60_000)
	for i := 0; i < n; i++ {
		open := start + int64(i)*step
		price := 100.0 + float64(i)*0.01
		out[i] = exchange.Kline{
			OpenTime:  open,
			CloseTime: open + step - 1,
			Open:      price,
			High:      price + 1,
			Low:       price - 1,
			Close:     price + 0.5,
			Volume:    10,
		}
	}
	return out
}

func TestMarkerExport_3000BarsTiming(t *testing.T) {
	klines := syntheticKlines(3050)
	settings := NormalizeRSXSettings(GetRSXSettings())
	cfg := ChartStreamingReplayConfig(settings, "1m")
	m := NewMarker(nil, nil, "1m", "", cfg.ChaosCfg)
	m.ApplyBacktestRSXConfig(settings)
	for _, k := range klines {
		m.UpdateKlineTick(k, true)
	}

	start := time.Now()
	result, ok := ExportChartSeriesForWindow(m, klines[len(klines)-3000:], settings)
	elapsed := time.Since(start)
	if !ok {
		t.Fatal("export failed")
	}
	t.Logf("marker export 3000 bars: %s (%d points)", elapsed.Round(time.Millisecond), len(result.ChartPoints))
}

func BenchmarkStreamingReplayAccumulator_3000Bars(b *testing.B) {
	klines := syntheticKlines(3050)
	settings := NormalizeRSXSettings(GetRSXSettings())
	cfg := ChartStreamingReplayConfig(settings, "1m")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewStreamingReplayAccumulator(klines, cfg)
	}
}

func TestStreamingReplayAccumulator_3000BarsUnderBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("cold replay budget check")
	}
	klines := syntheticKlines(3050)
	settings := NormalizeRSXSettings(GetRSXSettings())
	cfg := ChartStreamingReplayConfig(settings, "1m")

	start := time.Now()
	acc := NewStreamingReplayAccumulator(klines, cfg)
	elapsed := time.Since(start)
	t.Logf("cold replay 3050 bars: %s (%d points)", elapsed.Round(time.Millisecond), len(acc.Result().ChartPoints))
}
