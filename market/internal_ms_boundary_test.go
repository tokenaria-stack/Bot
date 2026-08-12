package market

import (
	"path/filepath"
	"testing"

	"trading_bot/data"
	"trading_bot/exchange"
)

// Debt #83 C2: internal Frame/Ingress paths must not re-guess Unix-ms units.

func TestFrameUpdateKlineTick_PreservesPre2001MillisecondOpen(t *testing.T) {
	const pre2001Ms int64 = 730_944_000_000
	f := NewFrame(nil, "1M", ChaosConfig{})
	f.UpdateKlineTick(exchange.Kline{
		OpenTime: pre2001Ms, CloseTime: pre2001Ms + 1,
		Open: 1, High: 1, Low: 1, Close: 1, Volume: 1,
	}, true)
	got := f.GetKlines()
	if len(got) != 1 || got[0].OpenTime != pre2001Ms {
		t.Fatalf("Frame open=%v want %d (no ×1000 at ingest)", got, pre2001Ms)
	}
}

func TestMergeKlineSeries_PreservesPre2001MillisecondOpen(t *testing.T) {
	const pre2001Ms int64 = 730_944_000_000
	a := []exchange.Kline{{
		OpenTime: pre2001Ms, CloseTime: pre2001Ms + 1,
		Open: 1, High: 2, Low: 1, Close: 1.5, Volume: 1,
	}}
	b := []exchange.Kline{{
		OpenTime: 1_700_000_040_000, CloseTime: 1_700_000_099_999,
		Open: 2, High: 3, Low: 2, Close: 2.5, Volume: 2,
	}}
	out := exchange.MergeKlineSeries(a, b, exchange.AuthoritySettled, exchange.AuthoritySettled)
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if out[0].OpenTime != pre2001Ms {
		t.Fatalf("first open=%d want %d", out[0].OpenTime, pre2001Ms)
	}
}

func TestCandlesToKlinesStyle_SQLiteMsUnchanged(t *testing.T) {
	// Mirrors main/server candlesToKlines assign-only after C2 (no NormalizeKline).
	data.ResetDBForTest(filepath.Join(t.TempDir(), "c2_candles.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}
	const pre2001Ms int64 = 730_944_000_000
	row := data.Candle{
		OpenTime: pre2001Ms, CloseTime: pre2001Ms + 1,
		Open: 1, High: 1, Low: 1, Close: 1, Volume: 1,
	}
	if err := data.SaveKlines("BTCUSDT", "1M", []data.Candle{row}); err != nil {
		t.Fatal(err)
	}
	loaded, err := data.LoadKlines("BTCUSDT", "1M", pre2001Ms, pre2001Ms, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].OpenTime != pre2001Ms {
		t.Fatalf("load=%v", loaded)
	}
	k := exchange.Kline{
		OpenTime: loaded[0].OpenTime, CloseTime: loaded[0].CloseTime,
		Open: loaded[0].Open, High: loaded[0].High, Low: loaded[0].Low,
		Close: loaded[0].Close, Volume: loaded[0].Volume,
	}
	f := NewFrame(nil, "1M", ChaosConfig{})
	f.UpdateKlineTick(k, true)
	got := f.GetKlines()
	if len(got) != 1 || got[0].OpenTime != pre2001Ms {
		t.Fatalf("SQLite→Frame open=%v want %d", got, pre2001Ms)
	}
}
