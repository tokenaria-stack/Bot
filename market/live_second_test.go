package market

import (
	"path/filepath"
	"testing"

	"trading_bot/data"
	"trading_bot/exchange"
)

func TestAttachLiveSecondFrame(t *testing.T) {
	data.ResetDBForTest(filepath.Join(t.TempDir(), "micro_boot.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}
	frames := map[string]*Frame{}
	AttachLiveSecondFrames(frames, "BTCUSDT", ChaosConfig{})
	if frames["1s"] == nil {
		t.Fatal("missing 1s Frame")
	}
	if frames["5s"] != nil {
		t.Fatal("5s must not boot from AttachLiveSecondFrames")
	}
}

func TestMicroRAMCapDropsOldest(t *testing.T) {
	n := MicroKlineRAMCap + 5
	ks := make([]exchange.Kline, n)
	for i := 0; i < n; i++ {
		ot := int64(i) * 1000
		ks[i] = exchange.Kline{OpenTime: ot, CloseTime: ot + 999, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1}
	}
	f := NewFrame(nil, "1s", ChaosConfig{})
	f.ReplaceWorkingKlines(ks)
	got := f.GetKlines()
	if len(got) != MicroKlineRAMCap {
		t.Fatalf("len=%d want %d", len(got), MicroKlineRAMCap)
	}
	if got[0].OpenTime != int64(5)*1000 {
		t.Fatalf("oldest open=%d", got[0].OpenTime)
	}
}

func TestSecondGapDoesNotUnpublishNative(t *testing.T) {
	f1 := NewFrame(synthOHLCV(40), "1m", testChaos())
	f1s := NewFrame(nil, "1s", testChaos())
	rt := NewRuntime(map[string]*Frame{"1m": f1, "1s": f1s}, nil, nil, true, false, "BTCUSDT", "1m")
	if !rt.IsTimelinePublishable() {
		t.Fatal("expected publishable")
	}
	rt.applyAggTrade(exchange.AggTrade{TimeMs: 1_700_000_010_010, Price: 100, Qty: 1})
	rt.applyAggTrade(exchange.AggTrade{TimeMs: 1_700_000_015_010, Price: 101, Qty: 1})
	if !rt.IsTimelinePublishable() {
		t.Fatal("1s hole must not unpublish Master")
	}
	got := f1s.GetKlines()
	if len(got) != 2 {
		t.Fatalf("1s bars=%d want 2 (honest gap, no fill)", len(got))
	}
}

func TestBootHydrateSparseMicroDoesNotUnpublish(t *testing.T) {
	data.ResetDBForTest(filepath.Join(t.TempDir(), "micro_hydrate.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}
	rows := []data.Candle{
		{OpenTime: 1001, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, CloseTime: 2000},
		{OpenTime: 3001, Open: 2, High: 2, Low: 2, Close: 2, Volume: 1, CloseTime: 4000},
	}
	if err := data.SaveMicroKlines("BTCUSDT", "1s", rows); err != nil {
		t.Fatal(err)
	}
	frames := map[string]*Frame{}
	AttachLiveSecondFrames(frames, "BTCUSDT", ChaosConfig{})
	got := frames["1s"].GetKlines()
	if len(got) != 2 || got[0].OpenTime != 1001 || got[1].OpenTime != 3001 {
		t.Fatalf("hydrated=%v", got)
	}
	f1 := NewFrame(synthOHLCV(40), "1m", testChaos())
	rt := NewRuntime(map[string]*Frame{"1m": f1, "1s": frames["1s"]}, nil, nil, true, false, "BTCUSDT", "1m")
	if !rt.IsTimelinePublishable() {
		t.Fatal("sparse 1s hydrate must not unpublish")
	}
	rt.applyAggTrade(exchange.AggTrade{TimeMs: 3001 + 10, Price: 9, Qty: 1})
	if len(frames["1s"].GetKlines()) != 2 {
		t.Fatal("same-second trade after hydrate must not reopen closed bar")
	}
}
