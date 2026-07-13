package strategy

import (
	"math"

	"trading_bot/core"
	"trading_bot/exchange"
)

// ExtractDAGNavigatorSeries replays klines through the chart DAG and returns
// oldest-first Jurik RSX and Wozduh-slow series aligned 1:1 with klines.
// Used by navigators — never Falcon / chartExportPoints (Shot 9H).
func ExtractDAGNavigatorSeries(klines []exchange.Kline, rsx RSXSettings) (rsxVals, wozSlow []float64) {
	n := len(klines)
	if n == 0 {
		return nil, nil
	}
	hist := ReplayDAGKlines(klines, rsx)
	rsxVals = histSlotSeriesOldestFirst(hist, core.SlotJurikRSX, n)
	wozSlow = histSlotSeriesOldestFirst(hist, core.SlotWozduhSlow, n)
	return rsxVals, wozSlow
}

func histSlotSeriesOldestFirst(hist *core.HistoryBus, slot core.Slot, n int) []float64 {
	out := make([]float64, n)
	if hist == nil || n <= 0 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	count := hist.Count()
	for i := 0; i < n; i++ {
		lookback := n - i
		if lookback > count {
			out[i] = math.NaN()
			continue
		}
		out[i] = hist.Get(slot, lookback)
	}
	return out
}
