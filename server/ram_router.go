package server

import (
	"context"

	"trading_bot/exchange"
	"trading_bot/strategy"
)

// analystForSpec resolves the live Marker for a dashboard timeframe spec.
func (d *DashboardServer) analystForSpec(spec TimeframeSpec) *strategy.Marker {
	if d == nil {
		return nil
	}
	if analyst, ok := d.analysts[spec.ID]; ok {
		return analyst
	}
	if analyst, ok := d.analysts[spec.BinanceInterval]; ok && spec.Kind == TFBinanceREST {
		return analyst
	}
	return nil
}

type ramChartExport struct {
	candles     []ChartCandle
	oscillators []ChartOscillator
	annotations []strategy.ChartAnnotation
	klines      []exchange.Kline
	hasMore     bool
}

// buildRAMChartExport serves GET /api/state from Marker RAM (zero replay, ~1ms slice export).
func (d *DashboardServer) buildRAMChartExport(
	ctx context.Context,
	spec TimeframeSpec,
	candleLimit int,
	settings strategy.RSXSettings,
) (*ramChartExport, error) {
	if err := requestCtxErr(ctx); err != nil {
		return nil, err
	}
	if candleLimit <= 0 {
		candleLimit = defaultStateCandleLimit
	}

	klines := d.loadLiveKlinesFromRAM(spec, candleLimit)
	windowSize := candleLimit + strategy.IndicatorWarmupBars
	trimBars := strategy.IndicatorWarmupBars
	if IsOrderFlowTimeframe(spec) {
		windowSize = candleLimit
		trimBars = orderFlowWarmupBars
	}
	if len(klines) > windowSize {
		klines = klines[len(klines)-windowSize:]
	}
	if len(klines) == 0 {
		return nil, errWarmingUp
	}

	analyst := d.analystForSpec(spec)
	var candles []ChartCandle
	var oscillators []ChartOscillator
	var annotations []strategy.ChartAnnotation
	var ok bool

	if analyst != nil {
		candles, oscillators, annotations, ok = d.buildLiveChartFromRAM(analyst, klines, settings)
	}
	if !ok {
		if IsOrderFlowTimeframe(spec) {
			candles, oscillators, annotations = buildOHLCChartFromKlines(klines)
		} else {
			return nil, errWarmingUp
		}
	}

	trimBars = historyWarmupTrim(len(klines), candleLimit, trimBars)
	if trimBars > 0 && len(candles) > trimBars {
		candles = candles[trimBars:]
		oscillators = oscillators[trimBars:]
		annotations = trimAnnotations(annotations, trimBars, klines)
	}
	if candleLimit > 0 && len(candles) > candleLimit {
		candles = candles[len(candles)-candleLimit:]
		oscillators = oscillators[len(oscillators)-candleLimit:]
		if len(candles) > 0 {
			annotations = annotationsInWindow(annotations, candles[0].Time, candles[len(candles)-1].Time)
		}
	}

	hasMore := false
	if spec.Kind == TFBinanceREST && spec.BinanceInterval != "" && len(candles) > 0 {
		hasMore = d.sqliteHasBarsBefore(spec.BinanceInterval, candles[0].Time*1000)
	}

	return &ramChartExport{
		candles:     candles,
		oscillators: oscillators,
		annotations: annotations,
		klines:      klines,
		hasMore:     hasMore,
	}, nil
}

func annotationsInWindow(annotations []strategy.ChartAnnotation, fromSec, toSec int64) []strategy.ChartAnnotation {
	if len(annotations) == 0 {
		return nil
	}
	out := make([]strategy.ChartAnnotation, 0, len(annotations))
	for _, ann := range annotations {
		if ann.Time >= fromSec && ann.Time <= toSec {
			out = append(out, ann)
		}
	}
	return out
}
