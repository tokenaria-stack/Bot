package strategy

import (
	"context"
	"testing"

	"trading_bot/exchange"
)

func TestComputeBacktestMetrics(t *testing.T) {
	t.Parallel()

	trades := []backtestClosedTrade{
		{dollarPnL: 200, pnlPct: 2},
		{dollarPnL: -100, pnlPct: -1},
		{dollarPnL: 150, pnlPct: 1.5},
	}

	m := computeBacktestMetrics(10000, 10250, trades, 2.5, 250)
	if m.totalTrades != 3 {
		t.Fatalf("totalTrades = %d, want 3", m.totalTrades)
	}
	if m.winRate < 66.66 || m.winRate > 66.67 {
		t.Fatalf("winRate = %v, want ~66.67", m.winRate)
	}
	if m.netProfit != 2.5 {
		t.Fatalf("netProfit = %v, want 2.5", m.netProfit)
	}
	if m.profitFactor != 3.5 {
		t.Fatalf("profitFactor = %v, want 3.5", m.profitFactor)
	}
	if m.maxDrawdown != 2.5 {
		t.Fatalf("maxDrawdown = %v, want 2.5", m.maxDrawdown)
	}
	if m.recoveryFactor != 1.0 {
		t.Fatalf("recoveryFactor = %v, want 1.0", m.recoveryFactor)
	}
}

func TestBacktestPadStartMs(t *testing.T) {
	t.Parallel()

	start := int64(90 * 24 * 60 * 60 * 1000) // 90 days epoch ms
	end := start + 30*24*60*60*1000

	padded, ok := PadBacktestStartMs("1d", start, end, 31)
	if !ok {
		t.Fatal("expected padding for 1d with 31 candles")
	}
	if padded >= start {
		t.Fatalf("padded start %d should be before %d", padded, start)
	}

	_, ok = PadBacktestStartMs("15m", start, end, 31)
	if ok {
		t.Fatal("15m should not pad")
	}
}

func TestBacktestPadStartDays(t *testing.T) {
	t.Parallel()

	if got := BacktestPadStartDays("1d", 31, 50); got < 60 {
		t.Fatalf("1d pad days = %d, want at least 60", got)
	}
	if got := BacktestPadStartDays("1w", 10, 50); got < 40*7 {
		t.Fatalf("1w pad days = %d, want enough weeks", got)
	}
}

func TestParseBacktestDateRange(t *testing.T) {
	t.Parallel()

	start, end, err := ParseBacktestDateRange("2025-01-01", "2025-01-02")
	if err != nil {
		t.Fatal(err)
	}
	if end <= start {
		t.Fatalf("end %d should be after start %d", end, start)
	}
}

func TestCheckFractalStop(t *testing.T) {
	t.Parallel()

	longPos := &btPosition{side: "BUY", stopPrice: 100}
	if price, hit := checkFractalStop(longPos, exchange.Candle{Low: 99, High: 105}); !hit || price != 100 {
		t.Fatalf("long stop hit = %v price %v, want hit at 100", hit, price)
	}
	if _, hit := checkFractalStop(longPos, exchange.Candle{Low: 101, High: 105}); hit {
		t.Fatal("long stop should not hit above stop")
	}

	shortPos := &btPosition{side: "SELL", stopPrice: 110}
	if price, hit := checkFractalStop(shortPos, exchange.Candle{Low: 100, High: 111}); !hit || price != 110 {
		t.Fatalf("short stop hit = %v price %v, want hit at 110", hit, price)
	}
}

func TestCalcPnLPct(t *testing.T) {
	t.Parallel()

	if got := calcPnLPct("BUY", 100, 110); got != 10 {
		t.Fatalf("long pnl = %v, want 10", got)
	}
	if got := calcPnLPct("SELL", 100, 90); got != 10 {
		t.Fatalf("short pnl = %v, want 10", got)
	}
}

func TestEvaluateBacktestDecision_UsesRequestMatrix(t *testing.T) {
	origL, origS := LongScoreThreshold(), ShortScoreThreshold()
	t.Cleanup(func() { SetScoreThresholds(origL, origS) })
	SetScoreThresholds(35, 35)

	matrix := ScoringMatrix{UseWozduhCross: true}
	engine := NewBacktestEngine(BacktestConfig{Matrix: &matrix})
	chief := NewChiefAnalyst()

	report := &Report{
		Close: 100,
		Falcon: FalconSignals{VolCrossMarker: "red"},
		Volatility: VolatilityState{
			ATR:    1.0,
			Regime: RegimeExpansion,
		},
	}

	decision := engine.evaluateBacktestDecision(report, chief)
	if decision.Action != SellAction {
		t.Fatalf("Action = %q, want SELL (matrix from config, not global)", decision.Action)
	}
}

func TestEvaluateBacktestDecision_DisabledMatrix(t *testing.T) {
	matrix := ScoringMatrix{}
	engine := NewBacktestEngine(BacktestConfig{Matrix: &matrix})
	chief := NewChiefAnalyst()

	report := &Report{
		Close:      100,
		RSXMarker:  "L",
		Falcon:     FalconSignals{VolCrossMarker: "lime"},
		Volatility: scalpVolatilityOK(),
	}

	decision := engine.evaluateBacktestDecision(report, chief)
	if decision.Action != WaitAction {
		t.Fatalf("Action = %q, want WAIT when matrix disabled", decision.Action)
	}
}

func TestProcessScoreForMatrix_IgnoresGlobalMatrix(t *testing.T) {
	ResetScoringMatrix()
	t.Cleanup(ResetScoringMatrix)

	report := longSignalReport()
	global := ProcessScore(context.Background(), report, DefaultScalpFeeRate, nil)
	if len(global.ActiveSignals) != 0 {
		t.Fatalf("global matrix should be disabled by default, got %d signals", len(global.ActiveSignals))
	}

	enabled := allEnabledScoringMatrix()
	local := ProcessScoreForMatrix(report, enabled)
	if len(local.ActiveSignals) == 0 {
		t.Fatal("expected active signals from explicit matrix")
	}
}
