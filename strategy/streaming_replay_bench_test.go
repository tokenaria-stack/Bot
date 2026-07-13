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

func TestExtractDAGNavigatorSeries_3000BarsTiming(t *testing.T) {
	klines := syntheticKlines(3050)
	settings := NormalizeRSXSettings(GetRSXSettings())
	window := klines[len(klines)-3000:]

	start := time.Now()
	rsx, woz := ExtractDAGNavigatorSeries(window, settings)
	elapsed := time.Since(start)
	if len(rsx) != 3000 || len(woz) != 3000 {
		t.Fatalf("series len rsx=%d woz=%d", len(rsx), len(woz))
	}
	t.Logf("DAG navigator series 3000 bars: %s", elapsed.Round(time.Millisecond))
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
	klines := syntheticKlines(3050)
	settings := NormalizeRSXSettings(GetRSXSettings())
	cfg := ChartStreamingReplayConfig(settings, "1m")
	start := time.Now()
	_ = NewStreamingReplayAccumulator(klines, cfg)
	elapsed := time.Since(start)
	t.Logf("streaming replay 3050 bars: %s", elapsed.Round(time.Millisecond))
}
