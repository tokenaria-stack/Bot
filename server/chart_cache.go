package server

import (
	"context"
	"math"

	"trading_bot/core"
	"trading_bot/exchange"
	"trading_bot/server/wire"
	"trading_bot/strategy"
)

// buildOHLCChartFromKlines materializes price candles without indicator replay (Order Flow SSOT).
func buildOHLCChartFromKlines(klines []exchange.Kline) ([]ChartCandle, []ChartOscillator, []strategy.ChartAnnotation) {
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
	settings strategy.RSXSettings,
) ([]ChartCandle, []ChartOscillator, []strategy.ChartAnnotation) {
	_ = interval
	if len(klines) == 0 {
		return nil, nil, nil
	}
	if err := requestCtxErr(ctx); err != nil {
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
	hist := strategy.ReplayDAGKlines(klines, settings)
	oscillators := dagOscillatorsFromHistory(hist, times)
	annotations := dagAnnotationsFromHistory(d, hist, times)
	return candles, oscillators, annotations
}

func dagAnnotationsFromHistory(d *DashboardServer, hist *core.HistoryBus, times []int64) []strategy.ChartAnnotation {
	if d == nil || d.projector == nil {
		return nil
	}
	wireAnns := d.projector.BuildHistoryAnnotations(hist, times)
	return strategyAnnotationsFromWire(wireAnns)
}

func strategyAnnotationsFromWire(anns []wire.Annotation) []strategy.ChartAnnotation {
	if len(anns) == 0 {
		return nil
	}
	out := make([]strategy.ChartAnnotation, len(anns))
	for i, a := range anns {
		out[i] = strategy.ChartAnnotation{
			Time:     a.Time,
			Pane:     a.Pane,
			Label:    a.Label,
			Color:    a.Color,
			Position: a.Position,
			Shape:    a.Shape,
		}
	}
	return out
}

// dagOscillatorsFromHistory fills legacy ChartOscillator rows from DAG slots (no Falcon).
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
