package server

import (
	"encoding/json"
	"testing"

	"trading_bot/core/nodes"
	"trading_bot/market"
)

func TestRSXDemand_WSFactsTriStateAndUnion(t *testing.T) {
	prev := market.GetEngineMode()
	t.Cleanup(func() { market.SetEngineMode(prev) })
	market.SetEngineMode(market.EngineModeChartOnly)

	f1m := demandFrame("1m")
	f15 := demandFrame("15s")
	d := NewDashboardServer(map[string]*market.Frame{"1m": f1m, "15s": f15}, nil, "BTCUSDT", nil, false, false, "1m")

	none := []string{}
	tv := []string{"rsx_tv_div"}
	fr := []string{"rsx_fractal_pivot"}
	a := &WSClient{plotIDs: []string{"line_rsx"}, facts: &tv}
	b := &WSClient{plotIDs: []string{"woz_slow"}, facts: &fr}
	d.clients[a] = true
	d.clients[b] = true
	d.clientTF[a] = "1m"
	d.clientTF[b] = "1m"
	d.recomputeAnalyticalDemand("1m")
	d.recomputeAnalyticalDemand("15s")

	m1, _, _, _, _, _, _, _ := f1m.RSXLiveStats()
	want := nodes.NeedRSXCore | nodes.NeedRSTV | nodes.NeedRSFractal
	if m1 != want {
		t.Fatalf("1m union %#b want %#b", m1, want)
	}
	m15, _, _, _, _, _, _, _ := f15.RSXLiveStats()
	if m15 != 0 {
		t.Fatalf("unused 15s %#b want 0", m15)
	}

	legacy := &WSClient{plotIDs: []string{"woz_fast"}} // facts omitted → all families
	d.clients[legacy] = true
	d.clientTF[legacy] = "1m"
	d.recomputeAnalyticalDemand("1m")
	mAll, _, _, _, _, _, _, _ := f1m.RSXLiveStats()
	if mAll != nodes.RSXWorkAll {
		t.Fatalf("omitted facts must demand all families, got %#b", mAll)
	}
	d.dropWSClient(legacy)

	empty := &WSClient{plotIDs: []string{"line_rsx"}, facts: &none}
	d.clients[empty] = true
	d.clientTF[empty] = "15s"
	d.recomputeAnalyticalDemand("15s")
	mEmpty, _, _, _, _, _, _, _ := f15.RSXLiveStats()
	if mEmpty != nodes.NeedRSXCore {
		t.Fatalf("facts [] must be Core only, got %#b", mEmpty)
	}

	unk := []string{"nope"}
	d.setClientSubscribe(empty, "15s", []string{"woz_fast"}, &unk)
	mUnk, _, _, _, _, _, _, _ := f15.RSXLiveStats()
	if mUnk != 0 {
		t.Fatalf("unknown source must add zero bits, got %#b", mUnk)
	}

	d.dropWSClient(a)
	d.dropWSClient(b)
	d.dropWSClient(empty)
	m1, _, _, _, _, _, _, _ = f1m.RSXLiveStats()
	m15, _, _, _, _, _, _, _ = f15.RSXLiveStats()
	if m1 != 0 || m15 != 0 {
		t.Fatalf("disconnect leak 1m=%#b 15s=%#b", m1, m15)
	}
}

func TestRSXDemand_SubscribeJSONFactsPresence(t *testing.T) {
	type msg struct {
		Facts *[]string `json:"facts"`
	}
	var omitted msg
	if err := json.Unmarshal([]byte(`{"type":"subscribe","tf":"1m"}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.Facts != nil {
		t.Fatal("omitted facts must stay nil")
	}
	var nully msg
	if err := json.Unmarshal([]byte(`{"facts":null}`), &nully); err != nil {
		t.Fatal(err)
	}
	if nully.Facts != nil {
		t.Fatal("JSON null must be nil pointer (legacy ALL)")
	}
	var empty msg
	if err := json.Unmarshal([]byte(`{"facts":[]}`), &empty); err != nil {
		t.Fatal(err)
	}
	if empty.Facts == nil {
		t.Fatal("explicit [] must be non-nil")
	}
	if len(*empty.Facts) != 0 {
		t.Fatal("explicit [] length")
	}
	if nodes.RSXWorkFromClient([]string{"line_rsx"}, omitted.Facts) != nodes.RSXWorkAll {
		t.Fatal("omitted → all")
	}
	if nodes.RSXWorkFromClient([]string{"line_rsx"}, nully.Facts) != nodes.RSXWorkAll {
		t.Fatal("null → all")
	}
	if nodes.RSXWorkFromClient([]string{"line_rsx"}, empty.Facts) != nodes.NeedRSXCore {
		t.Fatal("[] → core only")
	}
}

func TestRSXDemand_TFChangeMovesRSX(t *testing.T) {
	prev := market.GetEngineMode()
	t.Cleanup(func() { market.SetEngineMode(prev) })
	market.SetEngineMode(market.EngineModeChartOnly)

	f1m := demandFrame("1m")
	f15 := demandFrame("15s")
	d := NewDashboardServer(map[string]*market.Frame{"1m": f1m, "15s": f15}, nil, "BTCUSDT", nil, false, false, "1m")
	tv := []string{"rsx_tv_div"}
	c := &WSClient{}
	d.clients[c] = true
	d.setClientSubscribe(c, "1m", []string{"woz_fast"}, &tv)
	m1, _, _, _, _, _, _, _ := f1m.RSXLiveStats()
	if m1 != nodes.NeedRSXCore|nodes.NeedRSTV {
		t.Fatalf("1m %#b", m1)
	}
	d.setClientSubscribe(c, "15s", []string{"woz_fast"}, &tv)
	m1, _, _, _, _, _, _, _ = f1m.RSXLiveStats()
	m15, _, _, _, _, _, _, _ := f15.RSXLiveStats()
	if m1 != 0 {
		t.Fatalf("old TF leak %#b", m1)
	}
	if m15 != nodes.NeedRSXCore|nodes.NeedRSTV {
		t.Fatalf("new TF %#b", m15)
	}
}
