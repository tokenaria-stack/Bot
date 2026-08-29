package market

import (
	"testing"

	"trading_bot/exchange"
)

func TestSecondsHotAuditCountsFormingFanout(t *testing.T) {
	secondsHot.aggTrades.Store(0)
	secondsHot.sec1Forming.Store(0)
	secondsHot.sec1Closed.Store(0)
	secondsHot.sparseForming.Store(0)
	secondsHot.sparseClosed.Store(0)
	secondsHot.onKlineBarCall.Store(0)

	base := noon5s()
	frames := map[string]*Frame{
		"1m": NewFrame(synthOHLCV(40), "1m", testChaos()),
		"1s": NewFrame(nil, "1s", testChaos()),
	}
	for _, e := range exchange.SparseSecondChildren() {
		frames[e.Name] = NewFrame(nil, e.Name, testChaos())
	}
	rt := NewRuntime(frames, nil, nil, true, false, "BTCUSDT", "1m")
	var cbs int
	rt.SetOnKlineBar(func(string, exchange.Kline, bool) { cbs++ })

	rt.applyAggTrade(exchange.AggTrade{TimeMs: base + 10, Price: 100, Qty: 1})
	if secondsHot.aggTrades.Load() != 1 {
		t.Fatalf("agg=%d", secondsHot.aggTrades.Load())
	}
	if secondsHot.sec1Forming.Load() != 1 {
		t.Fatalf("1s forming=%d", secondsHot.sec1Forming.Load())
	}
	wantSparse := uint64(len(exchange.SparseSecondChildren()))
	if secondsHot.sparseForming.Load() != wantSparse {
		t.Fatalf("sparse forming=%d want %d", secondsHot.sparseForming.Load(), wantSparse)
	}
	if secondsHot.onKlineBarCall.Load() == 0 || cbs == 0 {
		t.Fatal("kline bar callback must run for forming fanout")
	}
}
