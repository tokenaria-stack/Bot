package market

import (
	"testing"

	"trading_bot/exchange"
)

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

func TestBacktestChartPointToSim_omitsOHLC(t *testing.T) {
	t.Parallel()

	sim := backtestChartPointToSim(BacktestChartPoint{
		Time:       1700000000,
		Open:       99,
		High:       101,
		Low:        98,
		Close:      100,
		RSX:        55,
		RsiVolFast: 40,
		RsiVolSlow: 35,
	})
	if sim.Time != 1700000000 || sim.RSX != 55 {
		t.Fatalf("unexpected sim point: %+v", sim)
	}
	if sim.RsiVolFast != 40 || sim.RsiVolSlow != 35 {
		t.Fatalf("rsi vol: fast=%v slow=%v", sim.RsiVolFast, sim.RsiVolSlow)
	}
}

func TestAssembleRunResult_SimOnly(t *testing.T) {
	t.Parallel()

	engine := NewBacktestEngine(BacktestConfig{SimOnly: true})
	run := engine.assembleRunResult(
		nil,
		[]BacktestSimPoint{{Time: 1, RSX: 50}},
		nil, nil, nil,
		false, nil,
	)
	if len(run.ChartData) != 0 {
		t.Fatalf("ChartData len = %d, want 0 when SimOnly", len(run.ChartData))
	}
	if len(run.SimData) != 1 || run.SimData[0].RSX != 50 {
		t.Fatalf("SimData = %+v", run.SimData)
	}
}

func TestAssembleRunResult_SkipNavigators(t *testing.T) {
	t.Parallel()

	engine := NewBacktestEngine(BacktestConfig{SkipNavigators: true})
	run := engine.assembleRunResult(
		nil, nil,
		make([]exchange.Kline, 100),
		nil, nil,
		false, nil,
	)
	if run.Navigators != nil && len(run.Navigators) > 0 {
		t.Fatalf("Navigators = %+v, want nil/empty when SkipNavigators", run.Navigators)
	}
}
