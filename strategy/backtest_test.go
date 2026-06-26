package strategy

import (
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

	start := exchange.BinanceFuturesGenesisMs + 90*24*60*60*1000 // 90 days after futures genesis
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

func TestBacktestPadStartMs_AllowsPreGenesis(t *testing.T) {
	t.Parallel()

	start := exchange.BinanceFuturesGenesisMs + 10*24*60*60*1000
	end := start + 30*24*60*60*1000

	padded, ok := PadBacktestStartMs("1d", start, end, 10)
	if !ok {
		t.Fatal("expected padding")
	}
	if padded >= exchange.BinanceFuturesGenesisMs {
		t.Fatalf("padded start %d should be before futures genesis for continuous contract", padded)
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

func TestApplyEntrySlippage(t *testing.T) {
	t.Parallel()

	const slip = 0.05
	long := applyEntrySlippage("BUY", 100, slip)
	if long <= 100 {
		t.Fatalf("long entry slip = %v, want > 100", long)
	}
	short := applyEntrySlippage("SELL", 100, slip)
	if short >= 100 {
		t.Fatalf("short entry slip = %v, want < 100", short)
	}
}

func TestApplyExitSlippage(t *testing.T) {
	t.Parallel()

	const slip = 0.05
	longExit := applyExitSlippage("BUY", 100, slip)
	if longExit >= 100 {
		t.Fatalf("long exit slip = %v, want < 100", longExit)
	}
	shortExit := applyExitSlippage("SELL", 100, slip)
	if shortExit <= 100 {
		t.Fatalf("short exit slip = %v, want > 100", shortExit)
	}
}

func TestCalcBacktestNetPnL_withSlippagePrices(t *testing.T) {
	t.Parallel()

	entry := applyEntrySlippage("BUY", 100, 0.05)
	exit := applyExitSlippage("BUY", 110, 0.05)
	qty := 1.0
	feeRate := 0.001

	net := calcBacktestNetPnL("BUY", entry, exit, qty, feeRate)
	raw := (exit - entry) * qty
	fees := entry*qty*feeRate + exit*qty*feeRate
	want := raw - fees
	if net != want {
		t.Fatalf("net = %v, want %v", net, want)
	}
	if net >= raw {
		t.Fatal("slipped prices with fees should reduce net vs ideal raw move")
	}
}

func TestResolveBacktestSlippage(t *testing.T) {
	t.Parallel()

	if got := ResolveBacktestSlippage(nil); got != DefaultBacktestSlippagePct {
		t.Fatalf("default slippage = %v, want %v", got, DefaultBacktestSlippagePct)
	}
	custom := 0.1
	if got := ResolveBacktestSlippage(&BacktestRunSettings{SlippagePct: custom}); got != custom {
		t.Fatalf("custom slippage = %v, want %v", got, custom)
	}
}

func TestNewBacktestEngine_defaultSlippage(t *testing.T) {
	t.Parallel()

	engine := NewBacktestEngine(BacktestConfig{})
	if engine.cfg.SlippagePct != DefaultBacktestSlippagePct {
		t.Fatalf("SlippagePct = %v, want %v", engine.cfg.SlippagePct, DefaultBacktestSlippagePct)
	}
}
