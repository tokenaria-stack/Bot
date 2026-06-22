package data

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoadKlines(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "test_history.db"))

	if err := InitDB(); err != nil {
		t.Fatal(err)
	}

	klines := []Candle{
		{OpenTime: 1_700_000_000_000, Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 10, CloseTime: 1_700_000_059_999},
		{OpenTime: 1_700_000_060_000, Open: 100.5, High: 102, Low: 100, Close: 101, Volume: 12, CloseTime: 1_700_000_119_999},
	}

	if err := SaveKlines("btcusdt", "15m", klines); err != nil {
		t.Fatal(err)
	}
	if err := SaveKlines("btcusdt", "15m", klines); err != nil {
		t.Fatal(err)
	}

	got, err := LoadKlines("BTCUSDT", "15m", 1_700_000_000_000, 1_700_000_120_000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Close != 100.5 || got[1].Close != 101 {
		t.Fatalf("unexpected closes: %+v", got)
	}
}

func TestExpectedKlineCount(t *testing.T) {
	count, err := ExpectedKlineCount("1m", 0, 60_000*100)
	if err != nil {
		t.Fatal(err)
	}
	if count != 100 {
		t.Fatalf("count = %d, want 100", count)
	}
}

func TestCacheCompletenessThreshold(t *testing.T) {
	expected, err := ExpectedKlineCount("15m", 1_700_000_000_000, 1_700_000_000_000+15*60_000*100)
	if err != nil {
		t.Fatal(err)
	}
	have := 98
	if float64(have) < float64(expected)*0.98 {
		t.Fatalf("should be incomplete: have=%d expected=%d", have, expected)
	}
}
