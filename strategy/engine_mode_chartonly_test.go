package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestEvaluateTick_ChartOnlySkipsFalconKeepsDAG(t *testing.T) {
	prev := GetEngineMode()
	t.Cleanup(func() { SetEngineMode(prev) })

	klines := make([]exchange.Kline, 80)
	base := int64(1_700_000_000_000)
	for i := range klines {
		ot := base + int64(i)*60_000
		px := 100.0 + float64(i)*0.1
		klines[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + 59_999,
			Open: px, High: px + 1, Low: px - 1, Close: px + 0.5, Volume: 10,
		})
	}

	SetEngineMode(EngineModeChartOnly)
	chart := NewMarker(klines, nil, "1m", "test", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	fs := chart.FalconSnapshot()
	if fs.JurikRSX != 0 || fs.RedLine != 0 || fs.GreenLine != 0 {
		t.Fatalf("ChartOnly must not evaluate Falcon, got %+v", fs)
	}
	if chart.DAGTickFrame() == nil {
		t.Fatal("ChartOnly must still run DAG")
	}

	SetEngineMode(EngineModeLive)
	live := NewMarker(klines, nil, "1m", "test", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	if live.DAGTickFrame() == nil {
		t.Fatal("Live must run DAG")
	}
	lfs := live.FalconSnapshot()
	if lfs.JurikRSX == 0 && lfs.RedLine == 0 {
		t.Fatal("Live warmup should populate Falcon signals")
	}
}
