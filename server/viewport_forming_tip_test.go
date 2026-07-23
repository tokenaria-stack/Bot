package server

import (
	"context"
	"math"
	"testing"
	"time"

	"trading_bot/core"
	"trading_bot/exchange"
	"trading_bot/market"
	"trading_bot/server/wire"
	"trading_bot/ui_config"
)

func TestProjectViewportFormingTip_OverwriteSameOpen(t *testing.T) {
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	rsx := market.NormalizeRSXSettings(market.RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	market.ApplyRSXSettings(rsx)

	step := int64(60_000)
	nowMs := time.Now().UnixMilli()
	capEnd := resolveClosedBarBoundary(0, "1m")
	capSec := exchange.ChartTimeSec(capEnd)

	nClosed := 40
	closed := make([]exchange.Kline, nClosed)
	for i := 0; i < nClosed; i++ {
		ot := capEnd - int64(nClosed-1-i)*step
		closed[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + step - 1,
			Open: 100 + float64(i), High: 101 + float64(i), Low: 99 + float64(i),
			Close: 100.5 + float64(i), Volume: 10,
		})
	}

	// Cap tip still "forming" on Frame (CloseTime in future) — same open as Cap last-closed.
	// This is the Cap/Frame race B2.2 OVERWRITE must handle (no append).
	tip := closed[len(closed)-1]
	tip.High = tip.High + 3
	tip.Low = tip.Low - 2
	tip.Close = tip.Close + 1.5
	tip.Volume = tip.Volume + 25
	tip.CloseTime = nowMs + 45_000
	if !isFormingKline(tip, nowMs) {
		t.Fatal("test setup: Cap tip must still be forming on Frame")
	}

	frameKlines := append([]exchange.Kline{}, closed[:len(closed)-1]...)
	frameKlines = append(frameKlines, tip)
	frame := market.NewFrame(frameKlines, "1m", market.ChaosConfig{})
	frame.SetRSXSettings(rsx)
	frame.ReapplyRSXSettings()

	d := &DashboardServer{
		frames:    map[string]*market.Frame{"1m": frame},
		projector: wire.NewProjector(reg),
		symbol:    "BTCUSDT",
	}

	// Cap-closed history uses settled OHLC for last bar (pre-live mutation).
	histClosed := append([]exchange.Kline{}, closed...)
	resp, ok := d.buildColumnarHistoryPayload(
		context.Background(),
		histClosed,
		30,
		0,
		rsx,
		[]string{"line_rsx", "line_rsx_signal"},
		false,
		"1m",
		"1m",
	)
	if !ok {
		t.Fatal("columnar payload failed")
	}

	if len(resp.Times) < 2 {
		t.Fatalf("want Cap tip preserved, got %d bars", len(resp.Times))
	}
	n := len(resp.Times)
	if resp.Times[n-1] != capSec {
		t.Fatalf("OVERWRITE must keep Cap tip open=%d got=%d", capSec, resp.Times[n-1])
	}
	if resp.ProjCont == nil || resp.ProjCont.ProjectionMode != string(viewportProjOverwrite) {
		t.Fatalf("want projectionMode=overwrite, got %+v", resp.ProjCont)
	}
	if resp.Candles.Close[n-1] != tip.Close {
		t.Fatalf("OVERWRITE OHLC Close=%.4f want Frame tip %.4f", resp.Candles.Close[n-1], tip.Close)
	}

	liveRSX := frame.DAGTickFrame().Get(core.SlotJurikRSX)
	plotRSX := resp.Plots["line_rsx"][n-1]
	if math.Abs(plotRSX-liveRSX) > 1e-9 {
		t.Fatalf("OVERWRITE tip RSX=%.12f want Frame Cur=%.12f", plotRSX, liveRSX)
	}
	if math.Abs(resp.ProjCont.LastRSX-liveRSX) > 1e-9 {
		t.Fatalf("projCont lastRSX=%.12f want Frame Cur=%.12f", resp.ProjCont.LastRSX, liveRSX)
	}
	t.Logf("B2.2 OVERWRITE OK: open=%d bars=%d tipRSX=%.6f ≡ Frame Cur mode=%s",
		capSec, n, plotRSX, resp.ProjCont.ProjectionMode)
}

func TestProjectViewportFormingTip_SeedsLiveEdge(t *testing.T) {
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	rsx := market.NormalizeRSXSettings(market.RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	market.ApplyRSXSettings(rsx)

	step := int64(60_000)
	nowMs := time.Now().UnixMilli()
	// Cap last-closed open ≈ currentOpen - step (ignore settle grace for synth by aligning Cap).
	capEnd := resolveClosedBarBoundary(0, "1m")
	capSec := exchange.ChartTimeSec(capEnd)

	// Closed prefix ending at Cap tip.
	nClosed := 40
	closed := make([]exchange.Kline, nClosed)
	for i := 0; i < nClosed; i++ {
		ot := capEnd - int64(nClosed-1-i)*step
		closed[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + step - 1,
			Open: 100 + float64(i), High: 101 + float64(i), Low: 99 + float64(i),
			Close: 100.5 + float64(i), Volume: 10,
		})
	}
	formingOT := capEnd + step
	forming := exchange.NormalizeKline(exchange.Kline{
		OpenTime: formingOT, CloseTime: formingOT + step - 1,
		Open: 140, High: 145, Low: 138, Close: 142, Volume: 50,
	})
	if forming.CloseTime < nowMs {
		// Ensure still forming relative to wall clock.
		forming.CloseTime = nowMs + 30_000
	}

	frame := market.NewFrame(append(append([]exchange.Kline{}, closed...), forming), "1m", market.ChaosConfig{})
	frame.SetRSXSettings(rsx)
	frame.ReapplyRSXSettings()

	d := &DashboardServer{
		frames:    map[string]*market.Frame{"1m": frame},
		projector: wire.NewProjector(reg),
		symbol:    "BTCUSDT",
	}

	resp, ok := d.buildColumnarHistoryPayload(
		context.Background(),
		closed, // GetWindow-style: Cap-closed only
		30,
		0,
		rsx,
		[]string{"line_rsx", "line_rsx_signal"},
		false,
		"1m",
		"1m",
	)
	if !ok {
		t.Fatal("columnar payload failed")
	}
	if len(resp.Times) < 2 {
		t.Fatalf("want closed+forming tip, got %d bars", len(resp.Times))
	}
	tipSec := resp.Times[len(resp.Times)-1]
	prevSec := resp.Times[len(resp.Times)-2]
	formingSec := exchange.ChartTimeSec(formingOT)
	if tipSec != formingSec {
		t.Fatalf("viewport tip=%d want forming=%d (Model 2)", tipSec, formingSec)
	}
	if prevSec != capSec {
		t.Fatalf("closed tip before forming=%d want Cap=%d", prevSec, capSec)
	}

	liveRSX := frame.DAGTickFrame().Get(core.SlotJurikRSX)
	plotRSX := resp.Plots["line_rsx"][len(resp.Plots["line_rsx"])-1]
	if math.Abs(plotRSX-liveRSX) > 1e-9 {
		t.Fatalf("viewport tip RSX=%.12f want live Cur=%.12f", plotRSX, liveRSX)
	}

	// Handoff class: first live tick same open → OVERWRITE (deltaSec=0).
	deltaSec := tipSec - prevSec
	if deltaSec != 60 {
		t.Fatalf("closed→forming gap want 60s got %d", deltaSec)
	}
	t.Logf("ADR-010 OK: viewport tip=%d forming=%d Cap=%d |ΔRSX tip vs Cur|=0 handoff=OVERWRITE@%d",
		tipSec, formingSec, capSec, tipSec)
}

func TestProjectViewportFormingTip_SkipsDeepHistory(t *testing.T) {
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	rsx := market.NormalizeRSXSettings(market.RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	step := int64(60_000)
	base := int64(1_700_000_000_000) // deep past
	closed := make([]exchange.Kline, 20)
	for i := range closed {
		ot := base + int64(i)*step
		closed[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + step - 1,
			Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 1,
		})
	}
	nowMs := time.Now().UnixMilli()
	formingOT := resolveClosedBarBoundary(0, "1m") + step
	forming := exchange.NormalizeKline(exchange.Kline{
		OpenTime: formingOT, CloseTime: nowMs + 60_000,
		Open: 10, High: 11, Low: 9, Close: 10.5, Volume: 2,
	})
	frame := market.NewFrame(append(append([]exchange.Kline{}, closed...), forming), "1m", market.ChaosConfig{})
	frame.SetRSXSettings(rsx)
	frame.ReapplyRSXSettings()

	d := &DashboardServer{
		frames:    map[string]*market.Frame{"1m": frame},
		projector: wire.NewProjector(reg),
	}
	resp, ok := d.buildColumnarHistoryPayload(
		context.Background(), closed, 20, 0, rsx, []string{"line_rsx"}, false, "1m", "1m",
	)
	if !ok {
		t.Fatal("payload failed")
	}
	last := resp.Times[len(resp.Times)-1]
	want := exchange.ChartTimeSec(closed[len(closed)-1].OpenTime)
	if last != want {
		t.Fatalf("deep history mutated tip=%d want=%d (must not attach live forming)", last, want)
	}
}
