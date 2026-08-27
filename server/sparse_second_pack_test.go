package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/market"
	"trading_bot/server/wire"
	"trading_bot/ui_config"
)

func TestSparseSecondGoldenPacksFormingPastCloseTime(t *testing.T) {
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC).UnixMilli()
	ct, err := data.BarCloseTimeMs(base, "5s")
	if err != nil {
		t.Fatal(err)
	}
	forming := exchange.Kline{
		OpenTime: base, CloseTime: ct,
		Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 2,
	}
	if time.Now().UnixMilli() <= forming.CloseTime {
		t.Fatal("test requires wall clock after 2024-01-15 12:00:05")
	}
	if isFormingKline(forming, time.Now().UnixMilli()) {
		t.Fatal("calendar predicate must already treat this 5s as closed")
	}
	frame := market.NewFrame(nil, "5s", market.ChaosConfig{})
	frame.UpdateKlineTick(forming, false)
	d := &DashboardServer{
		frames:    map[string]*market.Frame{"5s": frame},
		projector: wire.NewProjector(reg),
		symbol:    "BTCUSDT",
	}
	resp, ok := d.buildColumnarHistoryPayload(
		context.Background(),
		nil,
		30,
		0,
		market.NormalizeRSXSettings(market.RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"}),
		nil,
		false,
		false,
		"5s",
		"",
	)
	if !ok {
		t.Fatal("Closed=[] + Forming must pack")
	}
	if len(resp.Times) != 1 || resp.Times[0] != exchange.ChartTimeSec(base) {
		t.Fatalf("times=%v", resp.Times)
	}
	if resp.Candles.Open[0] != 100 || resp.Candles.Close[0] != 100.5 {
		t.Fatalf("candles %+v", resp.Candles)
	}
}

func TestSparseHistoryPackExcludesLiveForming(t *testing.T) {
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	closedOpen := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC).UnixMilli()
	closedCT, err := data.BarCloseTimeMs(closedOpen, "5s")
	if err != nil {
		t.Fatal(err)
	}
	closed := exchange.Kline{
		OpenTime: closedOpen, CloseTime: closedCT,
		Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 2,
	}
	liveOpen := time.Now().UnixMilli()
	liveOpen -= liveOpen % 5000
	liveCT, err := data.BarCloseTimeMs(liveOpen, "5s")
	if err != nil {
		t.Fatal(err)
	}
	forming := exchange.Kline{
		OpenTime: liveOpen, CloseTime: liveCT,
		Open: 200, High: 201, Low: 199, Close: 200.5, Volume: 3,
	}
	frame := market.NewFrame([]exchange.Kline{closed}, "5s", market.ChaosConfig{})
	frame.UpdateKlineTick(forming, false)
	d := &DashboardServer{
		frames:    map[string]*market.Frame{"5s": frame},
		projector: wire.NewProjector(reg),
		symbol:    "BTCUSDT",
	}
	rsx := market.NormalizeRSXSettings(market.RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	hist, ok := d.buildColumnarHistoryPayloadOpts(
		context.Background(), []exchange.Kline{closed}, 30, 0, rsx, nil,
		true, true, "5s", "", false,
	)
	if !ok {
		t.Fatal("closed-only HISTORY pack")
	}
	if len(hist.Times) != 1 || hist.Times[0] != exchange.ChartTimeSec(closedOpen) {
		t.Fatalf("HISTORY must not include live forming, times=%v", hist.Times)
	}
	live, ok := d.buildColumnarHistoryPayloadOpts(
		context.Background(), []exchange.Kline{closed}, 30, 0, rsx, nil,
		true, true, "5s", "", true,
	)
	if !ok {
		t.Fatal("LIVE overlay pack")
	}
	if len(live.Times) < 2 || live.Times[len(live.Times)-1] != exchange.ChartTimeSec(liveOpen) {
		t.Fatalf("LIVE-tail must keep forming overlay, times=%v", live.Times)
	}
}

func TestParseIncludeFormingQueryDefaultTrue(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/api/history?tf=5s&endTime=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !parseIncludeFormingQuery(req) {
		t.Fatal("omitted includeForming must default true")
	}
	req.URL.RawQuery = "tf=5s&endTime=1&includeForming=false"
	if parseIncludeFormingQuery(req) {
		t.Fatal("includeForming=false")
	}
}
