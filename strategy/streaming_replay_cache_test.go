package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestStreamingReplayAccumulator_WindowSlice(t *testing.T) {
	t.Parallel()

	base := int64(1_700_000_000_000)
	makeKlines := func(n int) []exchange.Kline {
		out := make([]exchange.Kline, n)
		for i := range out {
			price := 50000.0 + float64(i)
			out[i] = exchange.Kline{
				OpenTime: base + int64(i)*60_000,
				Open:     price,
				High:     price + 10,
				Low:      price - 10,
				Close:    price + 5,
				Volume:   100,
			}
		}
		return out
	}

	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "close", DivMethod: "tv"}
	cfg := ChartStreamingReplayConfig(settings, "1m")

	full := makeKlines(150)
	acc := NewStreamingReplayAccumulator(full, cfg)
	if len(acc.Result().ChartPoints) != 150 {
		t.Fatalf("initial points = %d want 150", len(acc.Result().ChartPoints))
	}

	window := full[50:]
	if result, ok := acc.TryServeWindow(window, settings); !ok || len(result.ChartPoints) != 100 {
		t.Fatalf("window slice ok=%v len=%d want 100", ok, len(result.ChartPoints))
	}
}

func TestStreamingReplayAccumulator_IncrementalExtend(t *testing.T) {
	t.Parallel()

	base := int64(1_700_000_000_000)
	makeKlines := func(n int) []exchange.Kline {
		out := make([]exchange.Kline, n)
		for i := range out {
			price := 50000.0 + float64(i)
			out[i] = exchange.Kline{
				OpenTime: base + int64(i)*60_000,
				Open:     price,
				High:     price + 10,
				Low:      price - 10,
				Close:    price + 5,
				Volume:   100,
			}
		}
		return out
	}

	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "close", DivMethod: "tv"}
	cfg := ChartStreamingReplayConfig(settings, "1m")

	first := makeKlines(120)
	acc := NewStreamingReplayAccumulator(first, cfg)
	if len(acc.Result().ChartPoints) != 120 {
		t.Fatalf("initial points = %d want 120", len(acc.Result().ChartPoints))
	}

	extended := makeKlines(125)
	if !acc.CanExtend(settings, extended) {
		t.Fatal("expected cache extend eligibility")
	}
	acc.Extend(extended)
	if len(acc.Result().ChartPoints) != 125 {
		t.Fatalf("extended points = %d want 125", len(acc.Result().ChartPoints))
	}
}

func TestRSXSettingsEqual(t *testing.T) {
	t.Parallel()
	a := RSXSettings{Length: 14, SignalLength: 9, Source: "close"}
	b := RSXSettings{Length: 14, SignalLength: 9, Source: "close"}
	if !RSXSettingsEqual(a, b) {
		t.Fatal("expected equal settings")
	}
	b.SignalLength = 21
	if RSXSettingsEqual(a, b) {
		t.Fatal("expected different settings")
	}
}
