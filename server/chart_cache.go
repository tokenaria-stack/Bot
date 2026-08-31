package server

import (
	"context"
	"math"
	"time"

	"trading_bot/core"
	"trading_bot/exchange"
	"trading_bot/market"
)

// buildOHLCChartFromKlines materializes price candles without indicator replay (Order Flow SSOT).
func buildOHLCChartFromKlines(klines []exchange.Kline) ([]ChartCandle, []ChartOscillator, []market.ChartAnnotation) {
	if len(klines) == 0 {
		return nil, nil, nil
	}
	candles := make([]ChartCandle, 0, len(klines))
	for _, k := range klines {
		if c, ok := ChartCandleFromKline(k); ok {
			candles = append(candles, c)
		}
	}
	return candles, nil, nil
}

// buildHistoryChartSeriesTrimmed builds legacy JSON history without Falcon/StreamingReplay.
// Candles from klines; oscillators + annotations from DAG → Projector (Shot 9I).
func (d *DashboardServer) buildHistoryChartSeriesTrimmed(
	ctx context.Context,
	klines []exchange.Kline,
	trim int,
	interval string,
	settings market.RSXSettings,
) ([]ChartCandle, []ChartOscillator, []market.ChartAnnotation) {
	_ = interval
	if len(klines) == 0 {
		return nil, nil, nil
	}
	if err := requestCtxErr(ctx); err != nil {
		return nil, nil, nil
	}

	// Shot 11A: same History Tip Protocol as columnar — closed bars only for Replay.
	klines = dropFormingTip(klines, time.Now().UnixMilli())
	if len(klines) == 0 {
		return nil, nil, nil
	}

	display := klines
	if trim > 0 && len(display) > trim {
		display = display[trim:]
	}
	if len(display) == 0 {
		return nil, nil, nil
	}

	candles := make([]ChartCandle, 0, len(display))
	for _, k := range display {
		if c, ok := ChartCandleFromKline(k); ok {
			candles = append(candles, c)
		}
	}
	if len(candles) == 0 {
		return nil, nil, nil
	}

	times := columnarTimesFromKlines(display)
	hist := market.ReplayDAGKlines(klines, settings)
	oscillators := dagOscillatorsFromHistory(hist, times)
	annotations := dagAnnotationsFromHistory(d, hist, times)
	return candles, oscillators, annotations
}

func dagAnnotationsFromHistory(d *DashboardServer, hist *core.HistoryBus, times []int64) []market.ChartAnnotation {
	_ = d
	_ = hist
	_ = times
	// RSX-SIGNAL-2B: ZigZag DivState is gone. Columnar history packs facts; this JSON path stays empty.
	return nil
}

// dagOscillatorsFromHistory fills legacy ChartOscillator rows from DAG slots (no Falcon).
// finiteOrZero maps warmup NaN to 0 on ReplayClosedBars (compute-all). This is not the
// live Frame Wozduh mask and is not current chart truth (columnar + WS omit non-finite).
func dagOscillatorsFromHistory(hist *core.HistoryBus, times []int64) []ChartOscillator {
	n := len(times)
	out := make([]ChartOscillator, n)
	histCount := 0
	if hist != nil {
		histCount = hist.Count()
	}
	for i := 0; i < n; i++ {
		lookback := n - i
		out[i] = ChartOscillator{Time: times[i]}
		if hist == nil || lookback > histCount {
			continue
		}
		out[i].Jurik = finiteOrZero(hist.Get(core.SlotJurikRSX, lookback))
		out[i].RSX = out[i].Jurik
		out[i].RSXSignal = finiteOrZero(hist.Get(core.SlotJurikSignal, lookback))
		fast := finiteOrZero(hist.Get(core.SlotWozduhFast, lookback))
		slow := finiteOrZero(hist.Get(core.SlotWozduhSlow, lookback))
		out[i].Blue = fast
		out[i].RsiVolFast = fast
		out[i].RsiVolSlow = slow
	}
	return out
}

func finiteOrZero(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}
