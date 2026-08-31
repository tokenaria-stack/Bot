package server

import (
	"testing"

	"trading_bot/core/nodes"
	"trading_bot/exchange"
	"trading_bot/market"
)

func demandKlines(n int) []exchange.Kline {
	out := make([]exchange.Kline, n)
	start := int64(1_700_000_000_000)
	step := int64(60_000)
	for i := 0; i < n; i++ {
		open := start + int64(i)*step
		price := 100.0 + float64(i)*0.01
		out[i] = exchange.Kline{
			OpenTime:  open,
			CloseTime: open + step - 1,
			Open:      price,
			High:      price + 1,
			Low:       price - 1,
			Close:     price + 0.5,
			Volume:    10,
		}
	}
	return out
}

func demandFrame(tf string) *market.Frame {
	return market.NewFrame(demandKlines(40), tf, market.ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
}

func TestWozduhDemand_WSUnionDisconnectAndTFChange(t *testing.T) {
	prev := market.GetEngineMode()
	t.Cleanup(func() { market.SetEngineMode(prev) })
	market.SetEngineMode(market.EngineModeChartOnly)

	f1m := demandFrame("1m")
	f15 := demandFrame("15s")
	d := NewDashboardServer(map[string]*market.Frame{"1m": f1m, "15s": f15}, nil, "BTCUSDT", nil, false, false, "1m")

	a := &WSClient{plotIDs: []string{"woz_fast"}}
	b := &WSClient{plotIDs: []string{"woz_rsi_price"}}
	d.clients[a] = true
	d.clients[b] = true
	d.clientTF[a] = "1m"
	d.clientTF[b] = "1m"
	d.recomputeWozduhDemand("1m")
	d.recomputeWozduhDemand("15s")

	want1m := nodes.WozduhMaskForPlots([]string{"woz_fast", "woz_rsi_price"})
	m1, _, _ := f1m.WozduhLiveStats()
	if m1 != want1m {
		t.Fatalf("1m union %#b want %#b", m1, want1m)
	}
	m15, _, _ := f15.WozduhLiveStats()
	if m15 != 0 {
		t.Fatalf("15s unused mask %#b want 0", m15)
	}

	unfiltered := &WSClient{} // nil plotIDs = WIRE-1 all
	d.clients[unfiltered] = true
	d.clientTF[unfiltered] = "1m"
	d.recomputeWozduhDemand("1m")
	mAll, _, _ := f1m.WozduhLiveStats()
	if mAll != nodes.WozduhMaskAll {
		t.Fatalf("nil slots must contribute MaskAll, got %#b", mAll)
	}

	d.dropWSClient(unfiltered)
	m1, _, _ = f1m.WozduhLiveStats()
	if m1 != want1m {
		t.Fatalf("disconnect unfiltered leftover %#b want %#b", m1, want1m)
	}

	d.dropWSClient(a)
	wantB := nodes.WozduhMaskForPlots([]string{"woz_rsi_price"})
	m1, _, _ = f1m.WozduhLiveStats()
	if m1 != wantB {
		t.Fatalf("after A disconnect %#b want %#b", m1, wantB)
	}

	d.setClientSubscribe(b, "15s", []string{"woz_rsi_price"}, nil)
	m1, _, _ = f1m.WozduhLiveStats()
	if m1 != 0 {
		t.Fatalf("old TF must drop B demand, got %#b", m1)
	}
	m15, _, _ = f15.WozduhLiveStats()
	if m15 != wantB {
		t.Fatalf("new TF %#b want %#b", m15, wantB)
	}

	d.dropWSClient(b)
	m15, _, _ = f15.WozduhLiveStats()
	if m15 != 0 {
		t.Fatalf("dead client must not pin demand, got %#b", m15)
	}
}
