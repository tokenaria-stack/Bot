package market

import (
	"testing"

	"trading_bot/exchange"
)

func TestAttachLiveSecondFrame(t *testing.T) {
	frames := map[string]*Frame{}
	AttachLiveSecondFrames(frames, ChaosConfig{})
	if frames["1s"] == nil {
		t.Fatal("missing 1s Frame")
	}
	if frames["5s"] != nil {
		t.Fatal("5s must not boot")
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
