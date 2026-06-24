package server

import (
	"testing"

	"trading_bot/exchange"
	"trading_bot/strategy"
)

func TestHistoryEndTimeToMs(t *testing.T) {
	t.Parallel()

	if got := historyEndTimeToMs(1700000000); got != 1700000000000 {
		t.Fatalf("seconds: got %d, want %d", got, 1700000000000)
	}
	if got := historyEndTimeToMs(1700000000000); got != 1700000000000 {
		t.Fatalf("milliseconds passthrough: got %d", got)
	}
	if got := historyEndTimeToMs(0); got != 0 {
		t.Fatalf("zero: got %d", got)
	}
}

func TestBuildChartSeriesTrimmed(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, 150)
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

	candles, oscillators := buildChartSeriesTrimmed(klines, 100, strategy.RSXLookbackDefault)
	if len(candles) != 50 {
		t.Fatalf("candles len = %d, want 50", len(candles))
	}
	if len(oscillators) != 50 {
		t.Fatalf("oscillators len = %d, want 50", len(oscillators))
	}
	if candles[0].Time >= candles[len(candles)-1].Time {
		t.Fatal("candles not in ascending time order")
	}

	shortKlines := klines[:80]
	shortCandles, shortOsc := buildChartSeriesTrimmed(shortKlines, 100, strategy.RSXLookbackDefault)
	if len(shortCandles) != 80 {
		t.Fatalf("short candles len = %d, want 80", len(shortCandles))
	}
	if len(shortOsc) != 80 {
		t.Fatalf("short oscillators len = %d, want 80 (history start — no price trim)", len(shortOsc))
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
