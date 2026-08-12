package exchange

import (
	"encoding/json"
	"testing"
)

// Debt #83 C1: Binance REST/WS source adapters must preserve known Unix-ms timestamps.

func TestKlineFromBinanceMs_PreservesPre2001AndPost2001(t *testing.T) {
	t.Parallel()
	const (
		pre2001Ms  int64 = 730_944_000_000 // 1993-03-01 — would be *1000 by EnsureUnixMillis
		post2001Ms int64 = 1_700_000_040_000
	)
	for _, ot := range []int64{pre2001Ms, post2001Ms} {
		k := klineFromBinanceMs(ot, ot+60_000-1, 1, 2, 0.5, 1.5, 9)
		if k.OpenTime != ot || k.CloseTime != ot+60_000-1 {
			t.Fatalf("open/close = %d/%d want %d/%d (no unit guess)", k.OpenTime, k.CloseTime, ot, ot+60_000-1)
		}
	}
}

func TestKlinesFromCandles_PreservesRESTMillisecondOpenTimes(t *testing.T) {
	t.Parallel()
	const pre2001Ms int64 = 730_944_000_000
	in := []Candle{
		{OpenTime: pre2001Ms, CloseTime: pre2001Ms + 1, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1},
		{OpenTime: 1_700_000_040_000, CloseTime: 1_700_000_099_999, Open: 2, High: 2, Low: 2, Close: 2, Volume: 2},
	}
	out := KlinesFromCandles(in)
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0].OpenTime != pre2001Ms || out[1].OpenTime != 1_700_000_040_000 {
		t.Fatalf("opens=%d,%d — REST ms must pass through unchanged", out[0].OpenTime, out[1].OpenTime)
	}
	// Contrast: NormalizeKline would corrupt the pre-2001 open.
	if got := NormalizeKline(Kline{OpenTime: pre2001Ms}).OpenTime; got == pre2001Ms {
		t.Fatalf("fixture assumption broken: NormalizeKline unexpectedly left pre-2001 ms unchanged")
	}
}

func TestWSKlinePayload_ToKlinePreservesMillis(t *testing.T) {
	t.Parallel()
	// Pre-2001 open in the JSON t field (ms by Binance WS contract).
	raw := []byte(`{
		"e":"kline","E":730944000000,"s":"BTCUSDT",
		"k":{"t":730944000000,"T":730944059999,"s":"BTCUSDT","i":"1M",
		"o":"100","c":"101","h":"102","l":"99","v":"1","x":true}
	}`)
	var event wsKlinePayload
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	kd := event.Kline
	open, _ := kd.Open.Float64()
	high, _ := kd.High.Float64()
	low, _ := kd.Low.Float64()
	closePrice, _ := kd.Close.Float64()
	volume, _ := kd.Volume.Float64()
	k := klineFromBinanceMs(kd.StartTime, kd.CloseTime, open, high, low, closePrice, volume)
	if k.OpenTime != 730_944_000_000 || k.CloseTime != 730_944_059_999 {
		t.Fatalf("WS→kline times open=%d close=%d (must not ×1000)", k.OpenTime, k.CloseTime)
	}
}
