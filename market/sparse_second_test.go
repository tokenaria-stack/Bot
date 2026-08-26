package market

import (
	"path/filepath"
	"testing"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

func noon5s() int64 {
	return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC).UnixMilli()
}

func TestHydrateSparseSecondFrom1s(t *testing.T) {
	data.ResetDBForTest(filepath.Join(t.TempDir(), "sparse5s_boot.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}
	base := noon5s()
	rows := []data.Candle{
		{OpenTime: base, Open: 1, High: 2, Low: 1, Close: 1.5, Volume: 1, CloseTime: base + 999},
		{OpenTime: base + 5000, Open: 3, High: 3, Low: 3, Close: 3, Volume: 1, CloseTime: base + 5999},
	}
	if err := data.SaveMicroKlines("BTCUSDT", "1s", rows); err != nil {
		t.Fatal(err)
	}
	frames := map[string]*Frame{}
	AttachLiveSecondFrames(frames, "BTCUSDT", ChaosConfig{})
	HydrateSparseSecondFrames(frames, "BTCUSDT", ChaosConfig{})
	for _, e := range exchange.SparseSecondChildren() {
		if frames[e.Name] == nil {
			t.Fatalf("missing %s Frame", e.Name)
		}
	}
	f5 := frames["5s"]
	if f5 == nil {
		t.Fatal("missing 5s Frame")
	}
	got := f5.GetKlines()
	if len(got) != 2 {
		t.Fatalf("5s bars=%d want 2 (closed + forming)", len(got))
	}
	if got[0].OpenTime != base || got[1].OpenTime != base+5000 {
		t.Fatalf("opens %d %d", got[0].OpenTime, got[1].OpenTime)
	}
	got10 := frames["10s"].GetKlines()
	if len(got10) != 1 || got10[0].OpenTime != base {
		t.Fatalf("10s same 10s-bucket must stay one forming bar, got %v", got10)
	}
}

func TestApplyAggTradeFansOutForming1sTo5s(t *testing.T) {
	base := noon5s()
	f1 := NewFrame(synthOHLCV(40), "1m", testChaos())
	f1s := NewFrame(nil, "1s", testChaos())
	f5s := NewFrame(nil, "5s", testChaos())
	f10s := NewFrame(nil, "10s", testChaos())
	rt := NewRuntime(map[string]*Frame{"1m": f1, "1s": f1s, "5s": f5s, "10s": f10s}, nil, nil, true, false, "BTCUSDT", "1m")
	rt.applyAggTrade(exchange.AggTrade{TimeMs: base + 10, Price: 100, Qty: 1})
	got := f5s.GetKlines()
	if len(got) != 1 || got[0].OpenTime != base {
		t.Fatalf("forming 5s after first 1s: %v", got)
	}
	rt.applyAggTrade(exchange.AggTrade{TimeMs: base + 5010, Price: 110, Qty: 1})
	got = f5s.GetKlines()
	if len(got) != 2 {
		t.Fatalf("later-bucket forming 1s must close prior 5s, bars=%d", len(got))
	}
	if got[0].OpenTime != base || got[1].OpenTime != base+5000 {
		t.Fatalf("opens %v", got)
	}
	got10 := f10s.GetKlines()
	if len(got10) != 1 || got10[0].OpenTime != base {
		t.Fatalf("10s must stay one forming bucket across 12:00:00 and 12:00:05, got %v", got10)
	}
}

func TestSparseSecondNoWSAndNoPersist(t *testing.T) {
	for _, id := range []string{"5s", "10s", "15s", "30s", "45s"} {
		if exchange.ShouldPersist(id) || exchange.IsNativeBinance(id) {
			t.Fatalf("%s must not persist", id)
		}
		want := "btcusdt@kline_" + id
		for _, s := range exchange.CombinedKlineStreamNames("BTCUSDT") {
			if s == want {
				t.Fatalf("%s must not have a kline WS", id)
			}
		}
	}
}
