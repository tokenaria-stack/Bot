package server

import (
	"context"
	"net/http"
	"testing"

	"trading_bot/exchange"
	"trading_bot/strategy"
)

func TestHistoryEndTimeToMs(t *testing.T) {
	t.Parallel()

	if got := historyEndTimeToMs(1700000000); got != 1700000000000 {
		t.Fatalf("seconds: got %d, want %d", got, 1700000000000)
	}
	if got := historyEndTimeToMs(0); got != 0 {
		t.Fatalf("zero: got %d", got)
	}
}

func TestBuildHistoryChartSeriesTrimmed(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, strategy.IndicatorWarmupBars+100)
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

	d := &DashboardServer{}
	candles, oscillators, _ := d.buildHistoryChartSeriesTrimmed(context.Background(), klines, strategy.IndicatorWarmupBars, "1m", strategy.GetRSXSettings())
	if len(candles) != 100 {
		t.Fatalf("candles len = %d, want 100", len(candles))
	}
	if len(oscillators) != 100 {
		t.Fatalf("oscillators len = %d, want 100", len(oscillators))
	}
	if candles[0].Time >= candles[len(candles)-1].Time {
		t.Fatal("candles not in ascending time order")
	}

	shortKlines := klines[:80]
	shortCandles, shortOsc, _ := d.buildHistoryChartSeriesTrimmed(context.Background(), shortKlines, 100, "1m", strategy.GetRSXSettings())
	if len(shortCandles) != 80 {
		t.Fatalf("short candles len = %d, want 80", len(shortCandles))
	}
	if len(shortOsc) != 80 {
		t.Fatalf("short oscillators len = %d, want 80 (history start — no price trim)", len(shortOsc))
	}
}

func TestBuildNavigatorsFromSeries(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, 500)
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

	d := &DashboardServer{}
	candles, oscillators, _ := d.buildHistoryChartSeriesTrimmed(context.Background(), klines, 100, "1m", strategy.GetRSXSettings())
	if len(candles) == 0 {
		t.Fatal("expected trimmed oscillators")
	}

	panes := defaultLiveNavigatorPanes()
	nav := buildNavigatorsFromSeries(context.Background(), "BTCUSDT", klines, oscillators, 100, "15m", panes, nil)
	if nav == nil {
		t.Fatal("expected navigators map")
	}
	priceNav, ok := nav["price"]
	if !ok {
		t.Fatal("expected price navigator key")
	}
	_ = priceNav // line count depends on pivot geometry in synthetic data
}

func TestParseRSXSettingsFromRequest(t *testing.T) {
	t.Parallel()

	base := strategy.GetRSXSettings()
	req, err := http.NewRequest(http.MethodGet, "/api/history/chunk?rsx_length=21&rsx_signal_length=5&rsx_source=hlc3&rsx_method=fractal&rsx_pivot_radius=4&min_price_delta_ratio=0.001&min_osc_delta=1.5&rsx_div_lookback=120", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := parseRSXSettingsFromRequest(req)
	if got.Length != 21 {
		t.Fatalf("length = %d, want 21", got.Length)
	}
	if got.SignalLength != 5 {
		t.Fatalf("signal_length = %d, want 5", got.SignalLength)
	}
	if got.Source != "hlc3" {
		t.Fatalf("source = %q, want hlc3", got.Source)
	}
	if got.DivMethod != "fractal" {
		t.Fatalf("div_method = %q, want fractal", got.DivMethod)
	}
	if got.PivotRadius != 4 {
		t.Fatalf("pivot_radius = %d, want 4", got.PivotRadius)
	}
	if got.DivLookback != 120 {
		t.Fatalf("div_lookback = %d, want 120", got.DivLookback)
	}
	if got.MinPriceDeltaRatio != 0.001 {
		t.Fatalf("min_price_delta_ratio = %v, want 0.001", got.MinPriceDeltaRatio)
	}
	if got.MinOscDelta != 1.5 {
		t.Fatalf("min_osc_delta = %v, want 1.5", got.MinOscDelta)
	}

	noOverride := parseRSXSettingsFromRequest(nil)
	if noOverride.Length != base.Length {
		t.Fatalf("nil request should use base settings")
	}
}

func TestHistoryWarmupTrim(t *testing.T) {
	t.Parallel()

	if got := historyWarmupTrim(149, 50, 100); got != 0 {
		t.Fatalf("history start: got trim %d, want 0", got)
	}
	if got := historyWarmupTrim(150, 50, 100); got != 100 {
		t.Fatalf("exact window: got trim %d, want 100", got)
	}
	if got := historyWarmupTrim(100, 50, 100); got != 0 {
		t.Fatalf("partial window: got trim %d, want 0", got)
	}
}
