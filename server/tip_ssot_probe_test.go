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

	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/market"
)

func TestCompareTipSSOT_Match(t *testing.T) {
	t.Parallel()
	step := int64(60_000)
	base := int64(1_700_000_000_000)
	mk := func(i int, close float64) exchange.Kline {
		ot := base + int64(i)*step
		return exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + step - 1,
			Open: close, High: close + 1, Low: close - 1, Close: close, Volume: float64(i + 1),
		})
	}
	hist := make([]exchange.Kline, 50)
	for i := range hist {
		hist[i] = mk(i, 100+float64(i))
	}
	frameK := hist[len(hist)-20:]
	rsx := market.NormalizeRSXSettings(market.RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	frame := market.NewFrame(frameK, "1m", market.ChaosConfig{})
	frame.SetRSXSettings(rsx)
	frame.ReapplyRSXSettings()

	now := base + 49*step + step // after last close
	res := compareTipSSOT(hist, frame.GetKlines(), frame, rsx, now, "1m")
	if !res.OK || res.Verdict != "DATA_PLANE_MATCH" {
		t.Fatalf("want DATA_PLANE_MATCH, got %+v", res)
	}
	if !res.OHLCMatch || !res.ReplayRSXMatch {
		t.Fatalf("expected full match: %+v", res)
	}
}

func TestCompareTipSSOT_OHLCMismatch(t *testing.T) {
	t.Parallel()
	step := int64(60_000)
	base := int64(1_700_000_000_000)
	mk := func(i int, close float64) exchange.Kline {
		ot := base + int64(i)*step
		return exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + step - 1,
			Open: close, High: close + 1, Low: close - 1, Close: close, Volume: 1,
		})
	}
	hist := []exchange.Kline{mk(0, 100), mk(1, 101), mk(2, 102)}
	frameK := []exchange.Kline{mk(0, 100), mk(1, 101), mk(2, 999)} // close mutated
	rsx := market.NormalizeRSXSettings(market.RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	frame := market.NewFrame(frameK, "1m", market.ChaosConfig{})
	now := base + 3*step
	res := compareTipSSOT(hist, frameK, frame, rsx, now, "1m")
	if res.Verdict != "DATA_PLANE_OHLC_MISMATCH" || res.OK {
		t.Fatalf("want OHLC mismatch, got %+v", res)
	}
	if math.Abs(res.DeltaC- (102-999)) > tipSSOTProbeEps {
		t.Fatalf("deltaC=%.6f", res.DeltaC)
	}
}

// TestTipSSOT_RealDataPlane_GetWindowVsFrame — real SQLite∪Frame seam on history.db.
func TestTipSSOT_RealDataPlane_GetWindowVsFrame(t *testing.T) {
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
	endMs, err := data.CapKlineEndToLastClosed(time.Now().UnixMilli(), interval)
	if err != nil {
		t.Fatalf("CapKlineEndToLastClosed: %v", err)
	}
	stepMs, err := data.IntervalDurationMs(interval)
	if err != nil {
		t.Fatalf("IntervalDurationMs: %v", err)
	}
	bootBars := market.FrameBootKlineLimit
	bootStart := endMs - int64(bootBars)*stepMs
	bootCandles, err := exchange.LoadContinuousContractFromDB(symbol, interval, bootStart, endMs, bootBars)
	if err != nil {
		t.Fatalf("boot LoadContinuousContractFromDB: %v", err)
	}
	if len(bootCandles) < bootBars/2 {
		t.Fatalf("boot bars too short: %d", len(bootCandles))
	}
	bootK := candlesToKlines(bootCandles)

	rsx := market.NormalizeRSXSettings(market.RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	market.ApplyRSXSettings(rsx)
	frame := market.NewFrame(bootK, interval, market.ChaosConfig{})
	frame.SetRSXSettings(rsx)
	frame.ReapplyRSXSettings()

	frames := map[string]*market.Frame{interval: frame}
	d := NewDashboardServer(frames, nil, symbol, nil, false, false, interval)
	ctx := context.Background()
	spec := mustSpec(t, interval)

	// Re-Cap immediately before probe so Frame tip and GetWindow share one boundary.
	endMs, err = data.CapKlineEndToLastClosed(time.Now().UnixMilli(), interval)
	if err != nil {
		t.Fatalf("re-Cap: %v", err)
	}
	if tip := frame.GetKlines(); len(tip) == 0 || tip[len(tip)-1].OpenTime != endMs {
		bootStart = endMs - int64(bootBars)*stepMs
		bootCandles, err = exchange.LoadContinuousContractFromDB(symbol, interval, bootStart, endMs, bootBars)
		if err != nil {
			t.Fatalf("re-boot Load: %v", err)
		}
		frame = market.NewFrame(candlesToKlines(bootCandles), interval, market.ChaosConfig{})
		frame.SetRSXSettings(rsx)
		frame.ReapplyRSXSettings()
		frames[interval] = frame
		d = NewDashboardServer(frames, nil, symbol, nil, false, false, interval)
	}

	// Production path: EndTimeMs=Now must Cap internally (ADR-009) → ≡ Frame tip.
	resNow := d.ProbeTipSSOT(ctx, interval, display)
	winNow, okNow := d.GetWindow(ctx, HistoryWindowQuery{
		Spec:        spec,
		EndTimeMs:   time.Now().UnixMilli(),
		CandleLimit: display,
	})
	if !okNow || len(winNow.Klines) == 0 {
		t.Fatal("GetWindow(Now) empty after Cap fix")
	}

	fmt.Printf("\n=== #67 CLOSED-BAR BOUNDARY FIX VERIFY: GetWindow vs Frame (%s %s) ===\n", symbol, interval)
	fmt.Printf("verdict=%s ok=%v histOT=%d frameOT=%d Cap=%d ohlcMatch=%v replayRSXMatch=%v\n",
		resNow.Verdict, resNow.OK, resNow.HistOpenTime, resNow.FrameOpenTime, endMs,
		resNow.OHLCMatch, resNow.ReplayRSXMatch)
	fmt.Printf("ΔO=%.6g ΔH=%.6g ΔL=%.6g ΔC=%.6g ΔV=%.6g |ΔRSX|=%.6f\n",
		resNow.DeltaO, resNow.DeltaH, resNow.DeltaL, resNow.DeltaC, resNow.DeltaV,
		math.Abs(resNow.HistReplayRSX-resNow.FrameReplayRSX))
	fmt.Printf("histBars=%d frameBars=%d KlineSettleGraceMs=%d\n",
		resNow.HistoryBars, resNow.FrameBars, data.KlineSettleGraceMs)

	if resNow.HistOpenTime != endMs {
		t.Fatalf("GetWindow(Now) tip OT=%d want Cap=%d (Closed-bar Boundary SSOT broken)", resNow.HistOpenTime, endMs)
	}
	if resNow.FrameOpenTime != endMs {
		t.Fatalf("Frame tip OT=%d want Cap=%d", resNow.FrameOpenTime, endMs)
	}

	histClosed := dropFormingTip(winNow.Klines, endMs+stepMs)
	frameClosed := dropFormingTip(frame.GetKlines(), endMs+stepMs)
	n := 20
	if len(histClosed) < n || len(frameClosed) < n {
		t.Fatalf("need ≥%d closed bars hist=%d frame=%d", n, len(histClosed), len(frameClosed))
	}
	mismatches := 0
	for i := 0; i < n; i++ {
		h := histClosed[len(histClosed)-n+i]
		f := frameClosed[len(frameClosed)-n+i]
		if h.OpenTime != f.OpenTime ||
			math.Abs(h.Open-f.Open) > tipSSOTProbeEps ||
			math.Abs(h.High-f.High) > tipSSOTProbeEps ||
			math.Abs(h.Low-f.Low) > tipSSOTProbeEps ||
			math.Abs(h.Close-f.Close) > tipSSOTProbeEps ||
			math.Abs(h.Volume-f.Volume) > tipSSOTProbeEps {
			mismatches++
		}
	}
	fmt.Printf("last-%d closed OHLCV mismatches: %d\n", n, mismatches)

	if !resNow.OK || !resNow.OHLCMatch || !resNow.ReplayRSXMatch {
		t.Fatalf("after Cap fix expected DATA_PLANE_MATCH, got %s: %+v", resNow.Verdict, resNow)
	}
	if mismatches > 0 {
		t.Fatalf("last-%d closed: %d OHLCV mismatches", n, mismatches)
	}
	fmt.Printf("VERDICT: Closed-bar Boundary SSOT HOLD — GetWindow(Now) ≡ Cap ≡ Frame (OHLCV+RSX).\n")
}

func mustSpec(t *testing.T, tf string) TimeframeSpec {
	t.Helper()
	spec, err := ResolveTimeframe(tf)
	if err != nil {
		t.Fatal(err)
	}
	return spec
}
