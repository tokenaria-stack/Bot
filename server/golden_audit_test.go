package server

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"trading_bot/core"
	"trading_bot/data"
	"trading_bot/domain"
	"trading_bot/exchange"
	"trading_bot/server/wire"
	"trading_bot/strategy"
	"trading_bot/ui_config"
)

// TestGoldenAudit compares SQLite (History) vs Binance REST (Live boot) OHLCV + RSX + tip JSON
// for one shared closed 1m bar. Diagnostic only — does not mutate production code.
func TestGoldenAudit(t *testing.T) {
	const (
		symbol   = "BTCUSDT"
		interval = "1m"
		nBars    = 400
	)

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), ".."))
	dbPath := filepath.Join(repoRoot, "history.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("history.db not found at %s: %v", dbPath, err)
	}

	data.ResetDBForTest(dbPath)
	if err := data.InitDB(); err != nil {
		t.Fatalf("InitDB: %v", err)
	}

	endMs, err := data.CapKlineEndToLastClosed(time.Now().Add(-5*time.Minute).UnixMilli(), interval)
	if err != nil {
		t.Fatalf("CapKlineEndToLastClosed: %v", err)
	}
	stepMs, err := data.IntervalDurationMs(interval)
	if err != nil {
		t.Fatalf("IntervalDurationMs: %v", err)
	}
	startMs := endMs - int64(nBars)*stepMs

	// ── Path A: History / SQLite ────────────────────────────────────────────
	sqliteCandles, err := exchange.LoadContinuousContractFromDB(symbol, interval, startMs, endMs, nBars)
	if err != nil {
		t.Fatalf("LoadContinuousContractFromDB: %v", err)
	}
	if len(sqliteCandles) == 0 {
		t.Fatal("SQLite returned 0 candles — archive empty for window?")
	}
	sqliteK := candlesToNormKlines(sqliteCandles)
	sqliteTip := sqliteK[len(sqliteK)-1]

	rsxCfg := strategy.NormalizeRSXSettings(strategy.RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	histA := strategy.ReplayDAGKlines(sqliteK, rsxCfg)
	if histA == nil || histA.Count() < 1 {
		t.Fatal("ReplayDAGKlines(SQLite) produced empty history")
	}
	rsxA := histA.Get(core.SlotJurikRSX, 1)

	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatalf("BuildUIRegistry: %v", err)
	}
	proj := wire.NewProjector(reg)
	timesA := []int64{exchange.ChartTimeSec(sqliteTip.OpenTime)}
	plotsA, _ := proj.BuildHistoryColumnsFiltered(histA, timesA, []string{"line_rsx"})
	histJSON := map[string]any{
		"source":    "history/columnar_tip",
		"timeframe": interval,
		"time":      timesA[0],
		"open":      sqliteTip.Open,
		"high":      sqliteTip.High,
		"low":       sqliteTip.Low,
		"close":     sqliteTip.Close,
		"volume":    sqliteTip.Volume,
		"plots":     map[string]float64{"line_rsx": plotsA["line_rsx"][0]},
	}

	// ── Path B: Live / Binance REST (Marker boot emulation) ─────────────────
	rest, err := exchange.NewBinanceExchange("", "", false)
	if err != nil {
		t.Fatalf("NewBinanceExchange: %v", err)
	}
	binanceCandles, err := rest.FetchClosedRange(symbol, interval, startMs, endMs)
	if err != nil {
		t.Fatalf("FetchClosedRange Binance: %v", err)
	}
	if len(binanceCandles) == 0 {
		t.Fatal("Binance returned 0 candles")
	}
	if len(binanceCandles) > nBars {
		binanceCandles = binanceCandles[len(binanceCandles)-nBars:]
	}
	binanceK := candlesToNormKlines(binanceCandles)
	binanceTip := binanceK[len(binanceK)-1]

	histB := strategy.ReplayDAGKlines(binanceK, rsxCfg)
	if histB == nil || histB.Count() < 1 {
		t.Fatal("ReplayDAGKlines(Binance) produced empty history")
	}
	rsxB := histB.Get(core.SlotJurikRSX, 1)

	// Reconstruct tip TickFrame from committed hist (same scalars RouteChartTick would project).
	liveFrame := tipFrameFromHist(histB)
	livePlots := proj.BuildTickJSON(liveFrame)
	chart, ok := ChartCandleFromDomain(domain.CandleFromKline(binanceTip))
	if !ok {
		t.Fatal("ChartCandleFromDomain(binance tip) rejected")
	}
	wsJSON := map[string]any{
		"source":    "live/RouteChartTick",
		"timeframe": interval,
		"time":      chart.Time,
		"open":      chart.Open,
		"high":      chart.High,
		"low":       chart.Low,
		"close":     chart.Close,
		"volume":    chart.Volume,
		"plots":     livePlots,
	}

	histRaw, _ := json.Marshal(histJSON)
	wsRaw, _ := json.Marshal(wsJSON)

	fmt.Printf("\n========== GOLDEN AUDIT (CORE 4.3) ==========\n")
	fmt.Printf("window endMs=%d barsWant=%d sqliteN=%d binanceN=%d\n", endMs, nBars, len(sqliteK), len(binanceK))
	fmt.Printf("--- OHLCV tip ---\n")
	fmt.Printf("SQLite  OHLCV: openTime=%d O=%.8f H=%.8f L=%.8f C=%.8f V=%.8f\n",
		sqliteTip.OpenTime, sqliteTip.Open, sqliteTip.High, sqliteTip.Low, sqliteTip.Close, sqliteTip.Volume)
	fmt.Printf("Binance OHLCV: openTime=%d O=%.8f H=%.8f L=%.8f C=%.8f V=%.8f\n",
		binanceTip.OpenTime, binanceTip.Open, binanceTip.High, binanceTip.Low, binanceTip.Close, binanceTip.Volume)
	fmt.Printf("--- RSX tip (SlotJurikRSX lookback=1) ---\n")
	fmt.Printf("History RSX: %.12f\n", rsxA)
	fmt.Printf("Live    RSX: %.12f\n", rsxB)
	fmt.Printf("--- JSON tip ---\n")
	fmt.Printf("History JSON: %s\n", string(histRaw))
	fmt.Printf("WS      JSON: %s\n", string(wsRaw))

	ohlcMatch := sqliteTip.OpenTime == binanceTip.OpenTime &&
		almostEq(sqliteTip.Open, binanceTip.Open) &&
		almostEq(sqliteTip.High, binanceTip.High) &&
		almostEq(sqliteTip.Low, binanceTip.Low) &&
		almostEq(sqliteTip.Close, binanceTip.Close) &&
		almostEq(sqliteTip.Volume, binanceTip.Volume)

	rsxMatch := almostEq(rsxA, rsxB)

	// Structural tip parity for wire handoff: same time + OHLC + line_rsx (ignore extra live plot keys).
	histRSXPlot := plotsA["line_rsx"][0]
	liveRSXPlot := math.NaN()
	if livePlots != nil {
		liveRSXPlot = livePlots["line_rsx"]
	}
	jsonMatch := timesA[0] == chart.Time &&
		almostEq(sqliteTip.Open, chart.Open) &&
		almostEq(sqliteTip.High, chart.High) &&
		almostEq(sqliteTip.Low, chart.Low) &&
		almostEq(sqliteTip.Close, chart.Close) &&
		almostEq(histRSXPlot, liveRSXPlot)

	fmt.Printf("--- BINARY VERDICT ---\n")
	fmt.Printf("OHLCV match: %v\n", ohlcMatch)
	fmt.Printf("RSX   match: %v\n", rsxMatch)
	fmt.Printf("JSON  match: %v  (time+OHLC+line_rsx; live may carry extra plot keys)\n", jsonMatch)
	fmt.Printf("============================================\n\n")

	t.Logf("OHLCV=%v RSX=%v JSON=%v", ohlcMatch, rsxMatch, jsonMatch)
}

func candlesToNormKlines(candles []exchange.Candle) []exchange.Kline {
	out := make([]exchange.Kline, len(candles))
	for i, c := range candles {
		out[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime:  c.OpenTime,
			CloseTime: c.CloseTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
		})
	}
	return out
}

func tipFrameFromHist(hist *core.HistoryBus) *core.TickFrame {
	f := &core.TickFrame{}
	if hist == nil || hist.Count() < 1 {
		return f
	}
	for s := core.Slot(0); s < core.SlotCount; s++ {
		f.Set(s, hist.Get(s, 1))
	}
	return f
}

func almostEq(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	if math.IsNaN(a) || math.IsNaN(b) {
		return false
	}
	const eps = 1e-9
	return math.Abs(a-b) <= eps
}
