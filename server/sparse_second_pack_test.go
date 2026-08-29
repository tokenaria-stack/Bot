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

func sparsePackServer(t *testing.T, frame *market.Frame) *DashboardServer {
	t.Helper()
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	return &DashboardServer{
		frames:    map[string]*market.Frame{"5s": frame},
		projector: wire.NewProjector(reg),
		symbol:    "BTCUSDT",
	}
}

func sparseRSX() market.RSXSettings {
	return market.NormalizeRSXSettings(market.RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
}

func sparseKline(openMs int64, o, h, l, c, v float64) exchange.Kline {
	ct, err := data.BarCloseTimeMs(openMs, "5s")
	if err != nil {
		panic(err)
	}
	return exchange.Kline{
		OpenTime: openMs, CloseTime: ct,
		Open: o, High: h, Low: l, Close: c, Volume: v,
	}
}

func assertClosedPrefixUnchanged(t *testing.T, closed, live columnarHistoryResponse) {
	t.Helper()
	n := len(closed.Times)
	if n == 0 {
		t.Fatal("closed Replay must be non-empty")
	}
	if len(live.Times) < n {
		t.Fatalf("live shorter than closed: closed=%d live=%d", n, len(live.Times))
	}
	for i := 0; i < n; i++ {
		if live.Times[i] != closed.Times[i] {
			t.Fatalf("times prefix [%d]: closed=%d live=%d", i, closed.Times[i], live.Times[i])
		}
		if live.Candles.Open[i] != closed.Candles.Open[i] ||
			live.Candles.High[i] != closed.Candles.High[i] ||
			live.Candles.Low[i] != closed.Candles.Low[i] ||
			live.Candles.Close[i] != closed.Candles.Close[i] ||
			live.Candles.Volume[i] != closed.Candles.Volume[i] {
			t.Fatalf("OHLC prefix mutated at %d", i)
		}
	}
	for id, col := range closed.Plots {
		got, ok := live.Plots[id]
		if !ok {
			t.Fatalf("missing plot %s after overlay", id)
		}
		if len(got) < n {
			t.Fatalf("plot %s shorter than closed n=%d len=%d", id, n, len(got))
		}
		for i := 0; i < n; i++ {
			if got[i] != col[i] {
				t.Fatalf("plot %s prefix mutated at %d: closed=%v live=%v", id, i, col[i], got[i])
			}
		}
	}
}

func TestSparseSecondNoOverwriteClosedReplay(t *testing.T) {
	base := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC).UnixMilli()
	closed := sparseKline(base, 100, 101, 99, 100.5, 2)
	frame := market.NewFrame(nil, "5s", market.ChaosConfig{})
	frame.UpdateKlineTick(closed, false)
	if frame.LastCommittedOpenTime() != 0 {
		t.Fatalf("forming-only Frame committed=%d", frame.LastCommittedOpenTime())
	}
	d := sparsePackServer(t, frame)
	rsx := sparseRSX()
	replay := []exchange.Kline{closed}
	want, ok := d.buildColumnarHistoryPayloadOpts(
		context.Background(), replay, 30, 0, rsx, nil,
		false, false, "5s", "", false,
	)
	if !ok {
		t.Fatal("closed Replay pack")
	}
	got, ok := d.buildColumnarHistoryPayloadOpts(
		context.Background(), replay, 30, 0, rsx, nil,
		false, false, "5s", "", true,
	)
	if !ok {
		t.Fatal("includeForming pack")
	}
	if len(got.Times) != len(want.Times) {
		t.Fatalf("OVERWRITE/APPEND forbidden: closed=%d live=%d", len(want.Times), len(got.Times))
	}
	assertClosedPrefixUnchanged(t, want, got)
}

func TestSparseSecondFrontierMismatchNoOverlay(t *testing.T) {
	base := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC).UnixMilli()
	replayClosed := sparseKline(base, 100, 101, 99, 100.5, 2)
	frameClosed := sparseKline(base+5000, 110, 111, 109, 110.5, 3)
	forming := sparseKline(base+10000, 120, 121, 119, 120.5, 4)
	frame := market.NewFrame([]exchange.Kline{frameClosed}, "5s", market.ChaosConfig{})
	frame.UpdateKlineTick(forming, false)
	if frame.LastCommittedOpenTime() != frameClosed.OpenTime {
		t.Fatalf("committed=%d want=%d", frame.LastCommittedOpenTime(), frameClosed.OpenTime)
	}
	d := sparsePackServer(t, frame)
	rsx := sparseRSX()
	want, ok := d.buildColumnarHistoryPayloadOpts(
		context.Background(), []exchange.Kline{replayClosed}, 30, 0, rsx, nil,
		false, false, "5s", "", false,
	)
	if !ok {
		t.Fatal("closed Replay pack")
	}
	got, ok := d.buildColumnarHistoryPayloadOpts(
		context.Background(), []exchange.Kline{replayClosed}, 30, 0, rsx, nil,
		false, false, "5s", "", true,
	)
	if !ok {
		t.Fatal("includeForming pack")
	}
	if len(got.Times) != len(want.Times) {
		t.Fatalf("mismatch must be NONE: closed=%d live=%d times=%v", len(want.Times), len(got.Times), got.Times)
	}
	assertClosedPrefixUnchanged(t, want, got)
}

func TestSparseSecondAppendFormingAfterMatchingFrontier(t *testing.T) {
	base := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC).UnixMilli()
	closed := sparseKline(base, 100, 101, 99, 100.5, 2)
	forming := sparseKline(base+5000, 120, 121, 119, 120.5, 4)
	frame := market.NewFrame([]exchange.Kline{closed}, "5s", market.ChaosConfig{})
	frame.UpdateKlineTick(forming, false)
	if frame.LastCommittedOpenTime() != closed.OpenTime {
		t.Fatalf("committed=%d want=%d", frame.LastCommittedOpenTime(), closed.OpenTime)
	}
	d := sparsePackServer(t, frame)
	rsx := sparseRSX()
	replay := []exchange.Kline{closed}
	want, ok := d.buildColumnarHistoryPayloadOpts(
		context.Background(), replay, 30, 0, rsx, nil,
		false, false, "5s", "", false,
	)
	if !ok {
		t.Fatal("closed Replay pack")
	}
	n := len(want.Times)
	got, ok := d.buildColumnarHistoryPayloadOpts(
		context.Background(), replay, 30, 0, rsx, nil,
		false, false, "5s", "", true,
	)
	if !ok {
		t.Fatal("LIVE overlay pack")
	}
	if len(got.Times) != n+1 {
		t.Fatalf("want exactly one APPEND, closed=%d live=%d", n, len(got.Times))
	}
	if got.Times[n] != exchange.ChartTimeSec(forming.OpenTime) {
		t.Fatalf("forming time=%d want=%d", got.Times[n], exchange.ChartTimeSec(forming.OpenTime))
	}
	if forming.OpenTime <= closed.OpenTime {
		t.Fatal("forming must be after committed closed")
	}
	assertClosedPrefixUnchanged(t, want, got)
	tick := d.projector.BuildTickJSON(frame.DAGTickFrame())
	if got.Candles.Open[n] != forming.Open || got.Candles.Close[n] != forming.Close {
		t.Fatalf("forming OHLC %+v", got.Candles)
	}
	for id, col := range got.Plots {
		if len(col) != n+1 {
			t.Fatalf("plot %s len=%d want=%d", id, len(col), n+1)
		}
		if v, ok := tick[id]; ok && col[n] != v {
			t.Fatalf("plot %s tip=%v Frame Cur=%v", id, col[n], v)
		}
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
