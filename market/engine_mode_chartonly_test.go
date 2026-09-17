package market

import (
	"testing"

	"trading_bot/exchange"
)

func TestEvaluateTick_BothModesRunDAGWithoutFalcon(t *testing.T) {
	prev := GetEngineMode()
	t.Cleanup(func() { SetEngineMode(prev) })

	klines := make([]exchange.Kline, 80)
	base := int64(1_700_000_000_000)
	for i := range klines {
		ot := base + int64(i)*60_000
		px := 100.0 + float64(i)*0.1
		klines[i] = exchange.Kline{
			OpenTime: ot, CloseTime: ot + 59_999,
			Open: px, High: px + 1, Low: px - 1, Close: px + 0.5, Volume: 10,
		}
	}

	SetEngineMode(EngineModeChartOnly)
	chart := NewFrame(klines, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	if chart.DAGTickFrame() == nil {
		t.Fatal("ChartOnly must still run DAG")
	}

	SetEngineMode(EngineModeLive)
	live := NewFrame(klines, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	if live.DAGTickFrame() == nil {
		t.Fatal("Live must run DAG")
	}
}
