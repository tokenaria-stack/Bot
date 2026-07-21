package server

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"trading_bot/core"
	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/market"
	"trading_bot/server/wire"
	"trading_bot/ui_config"
)

// TestTipHandoff_ProjectionSeam — History columnar tip vs Live Frame Cur after Cap hydrate.
// Answers GPT's remaining #67 question: is the F5 notch overwrite, T→T+1 append, or Cap protocol?
func TestTipHandoff_ProjectionSeam(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), ".."))
	dbPath := filepath.Join(repoRoot, "history.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("history.db missing: %v", err)
	}

	data.ResetDBForTest(dbPath)
	if err := data.InitDB(); err != nil {
		t.Fatalf("InitDB: %v", err)
	}

	const (
		symbol   = "BTCUSDT"
		interval = "1m"
		display  = 3000
	)
	stepMs, err := data.IntervalDurationMs(interval)
	if err != nil {
		t.Fatal(err)
	}
	capEnd, err := data.CapKlineEndToLastClosed(time.Now().UnixMilli(), interval)
	if err != nil {
		t.Fatal(err)
	}

	// Frame boot = Cap tip (closed) — production boot law.
	bootBars := market.FrameBootKlineLimit
	bootStart := capEnd - int64(bootBars)*stepMs
	bootCandles, err := exchange.LoadContinuousContractFromDB(symbol, interval, bootStart, capEnd, bootBars)
	if err != nil || len(bootCandles) < 50 {
		t.Fatalf("boot load: err=%v n=%d", err, len(bootCandles))
	}
	rsx := market.NormalizeRSXSettings(market.RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	market.ApplyRSXSettings(rsx)
	frame := market.NewFrame(candlesToKlines(bootCandles), interval, market.ChaosConfig{})
	frame.SetRSXSettings(rsx)
	frame.ReapplyRSXSettings()

	frames := map[string]*market.Frame{interval: frame}
	d := NewDashboardServer(frames, nil, symbol, nil, false, false, interval)
	ctx := context.Background()
	spec := mustSpec(t, interval)

	win, okWin := d.GetWindow(ctx, HistoryWindowQuery{
		Spec:        spec,
		EndTimeMs:   time.Now().UnixMilli(), // production: Cap inside GetWindow
		CandleLimit: display,
	})
	if !okWin || len(win.Klines) == 0 {
		t.Fatal("GetWindow empty")
	}

	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	proj := wire.NewProjector(reg)
	d.projector = proj

	col, okCol := d.buildColumnarHistoryPayload(
		ctx, win.Klines, display, market.IndicatorWarmupBars, rsx,
		[]string{"line_rsx", "line_rsx_signal", "line_woz"},
		false, interval, interval,
	)
	if !okCol || len(col.Times) == 0 {
		t.Fatal("columnar empty")
	}

	histTipSec := col.Times[len(col.Times)-1]
	histTipRSX := col.Plots["line_rsx"][len(col.Plots["line_rsx"])-1]
	frameClosed := dropFormingTip(frame.GetKlines(), time.Now().UnixMilli())
	if len(frameClosed) == 0 {
		t.Fatal("frame no closed tip")
	}
	frameTipOT := exchange.EnsureUnixMillis(frameClosed[len(frameClosed)-1].OpenTime)
	frameTipSec := exchange.ChartTimeSec(frameTipOT)

	dag := frame.DAGTickFrame()
	liveRSX := math.NaN()
	if dag != nil {
		liveRSX = dag.Get(core.SlotJurikRSX)
	}
	liveTickPlots := proj.BuildTickJSON(dag)
	livePlotRSX := liveTickPlots["line_rsx"]

	fmt.Printf("\n=== #67 TIP HANDOFF / PROJECTION SEAM ===\n")
	fmt.Printf("CapLastClosed=%d histTipSec=%d frameClosedTipSec=%d\n",
		capEnd, histTipSec, frameTipSec)
	fmt.Printf("histTipRSX=%.10f frameLiveCurRSX=%.10f tickJSON.line_rsx=%.10f\n",
		histTipRSX, liveRSX, livePlotRSX)
	fmt.Printf("timeDeltaSec(hist→frameClosed)=%d\n", frameTipSec-histTipSec)

	// Case A: Cap-aligned closed tips must share time + RSX (projection identity on closed bar).
	if histTipSec != frameTipSec {
		t.Fatalf("closed tip time mismatch: hist=%d frame=%d Cap=%d — GetWindow/ Cap / Frame boot drift",
			histTipSec, frameTipSec, exchange.ChartTimeSec(capEnd))
	}
	if math.Abs(histTipRSX-liveRSX) > 1e-9 {
		// Frame Cur after closed-only boot should equal hist tip (no forming bar yet).
		t.Fatalf("closed tip RSX mismatch: hist=%.12f liveCur=%.12f — projection/Replay seam",
			histTipRSX, liveRSX)
	}
	if math.Abs(histTipRSX-livePlotRSX) > 1e-9 {
		t.Fatalf("BuildTickJSON ≠ columnar tip: hist=%.12f tickJSON=%.12f", histTipRSX, livePlotRSX)
	}
	fmt.Printf("CLOSED IDENTITY: hist tip ≡ Frame Cur ≡ tickJSON (time+RSX) OK\n")

	// Case B: Forming tip on Frame → Viewport seeds it (ADR-010 Model 2); first WS = OVERWRITE.
	last := frameClosed[len(frameClosed)-1]
	nextOT := last.OpenTime + stepMs
	forming := exchange.NormalizeKline(exchange.Kline{
		OpenTime:  nextOT,
		CloseTime: time.Now().UnixMilli() + 45_000, // still forming
		Open:      last.Close,
		High:      last.Close + 10,
		Low:       last.Close - 5,
		Close:     last.Close + 3,
		Volume:    50,
	})
	frame.UpdateKlineTick(forming, false)
	dag2 := frame.DAGTickFrame()
	formingRSX := dag2.Get(core.SlotJurikRSX)
	formingSec := exchange.ChartTimeSec(nextOT)

	col2, ok2 := d.buildColumnarHistoryPayload(
		ctx, win.Klines, display, market.IndicatorWarmupBars, rsx,
		[]string{"line_rsx", "line_rsx_signal", "line_woz"},
		false, interval, interval,
	)
	if !ok2 || len(col2.Times) == 0 {
		t.Fatal("columnar after forming empty")
	}
	viewportTip := col2.Times[len(col2.Times)-1]
	viewportRSX := col2.Plots["line_rsx"][len(col2.Plots["line_rsx"])-1]
	deltaSec := viewportTip - histTipSec

	fmt.Printf("VIEWPORT HANDOFF (ADR-010): histClosedTip=%d viewportTip=%d formingSec=%d deltaSec=%d\n",
		histTipSec, viewportTip, formingSec, deltaSec)
	fmt.Printf("viewportRSX=%.10f formingLiveRSX=%.10f |Δ|=%.6e\n",
		viewportRSX, formingRSX, math.Abs(viewportRSX-formingRSX))

	if viewportTip != formingSec {
		t.Fatalf("viewport tip=%d want forming=%d (Model 2 seed)", viewportTip, formingSec)
	}
	if deltaSec != int64(stepMs/1000) {
		t.Fatalf("closed→forming gap=%d want %d", deltaSec, stepMs/1000)
	}
	if math.Abs(viewportRSX-formingRSX) > 1e-9 {
		t.Fatalf("viewport tip RSX ≠ live Cur: %.12f vs %.12f", viewportRSX, formingRSX)
	}
	fmt.Printf("VERDICT: Model 2 — first WS tick at %d OVERWRITES viewport tip (deltaSec=0 vs historyTip).\n", formingSec)
	fmt.Printf("================================================\n\n")
	t.Logf("handoff class=OVERWRITE viewportTip=%d |ΔRSX|=0", viewportTip)
}

// TestTipHandoff_SameOpenTimeOverwriteWouldMutateHistoryTip documents FE contract:
// if live tick.time == history tip time, ColumnarStore overwrites plots (possible kink ON tip).
// Production Tip Protocol should prefer APPEND (next open), not overwrite of Cap-closed tip.
func TestTipHandoff_SameOpenTimeIsOverwriteClass(t *testing.T) {
	t.Parallel()
	// Pure classification helper — mirrors columnar-store appendTick branch.
	classify := func(histTipSec, tickSec int64) string {
		if tickSec < histTipSec {
			return "DROP"
		}
		if tickSec == histTipSec {
			return "OVERWRITE"
		}
		return "APPEND"
	}
	if classify(100, 100) != "OVERWRITE" {
		t.Fatal("same open → overwrite")
	}
	if classify(100, 160) != "APPEND" {
		t.Fatal("next bar → append")
	}
	if classify(100, 90) != "DROP" {
		t.Fatal("past → drop")
	}
}
