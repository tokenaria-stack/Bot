package server

import (
	"context"

	"trading_bot/exchange"
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

func chartSeriesFromReplayResult(result *strategy.StreamingReplayResult, compact bool) ([]ChartCandle, []ChartOscillator, []strategy.ChartAnnotation) {
	if result == nil || len(result.ChartPoints) == 0 {
		return nil, nil, nil
	}
	candles := make([]ChartCandle, 0, len(result.ChartPoints))
	oscillators := make([]ChartOscillator, 0, len(result.ChartPoints))
	for _, pt := range result.ChartPoints {
		if !validOHLC(pt.Open, pt.High, pt.Low, pt.Close) {
			continue
		}
		candles = append(candles, ChartCandle{
			Time:   exchange.ChartTimeSec(pt.Time),
			Open:   roundWirePrice(pt.Open),
			High:   roundWirePrice(pt.High),
			Low:    roundWirePrice(pt.Low),
			Close:  roundWirePrice(pt.Close),
			Volume: roundWireVolume(pt.Volume),
		})
		osc := chartOscillatorFromReplayPoint(pt)
		if compact {
			osc = compactChartOscillator(osc)
		}
		oscillators = append(oscillators, osc)
	}
	return candles, oscillators, result.Annotations
}

func chartOscillatorFromReplayPoint(pt strategy.BacktestChartPoint) ChartOscillator {
	return ChartOscillator{
		Time:            exchange.ChartTimeSec(pt.Time),
		Jurik:           pt.Jurik,
		RSX:             pt.RSX,
		RSXSignal:       pt.RSXSignal,
		Red:             pt.RsiHl2,
		Green:           pt.EmaRsi,
		RedLine:         pt.RsiHl2,
		GreenLine:       pt.EmaRsi,
		Blue:            pt.RsiVolFast,
		RsiPrice:        pt.RsiPrice,
		EmaRsi:          pt.EmaRsi,
		RsiRsi:          pt.RsiRsi,
		RsiHl2:          pt.RsiHl2,
		RsiVolFast:      pt.RsiVolFast,
		RsiVolSlow:      pt.RsiVolSlow,
		MacdRsi:         pt.MacdRsi,
		RsiAd:           pt.RsiAd,
		RsiHl2Vol:       pt.RsiHl2Vol,
		VolCrossMarker:  pt.VolCrossMarker,
		VolChanMid:      pt.VolChanMid,
		VolChanUp:       pt.VolChanUp,
		VolChanDn:       pt.VolChanDn,
		PriceChanMid:    pt.PriceChanMid,
		PriceChanUp:     pt.PriceChanUp,
		PriceChanDn:     pt.PriceChanDn,
		Color:           pt.Color,
		Marker:          pt.Marker,
		VolumeSpikeUp:   pt.VolumeSpikeUp,
		VolumeSpikeDown: pt.VolumeSpikeDown,
	}
}

// buildHistoryChartSeriesTrimmed replays a SQLite history chunk without touching live Marker RAM.
// Legacy JSON /api/history path only — live charts use columnar + DAG Projector (Shot 9H).
func (d *DashboardServer) buildHistoryChartSeriesTrimmed(
	ctx context.Context,
	klines []exchange.Kline,
	trim int,
	interval string,
	settings strategy.RSXSettings,
) ([]ChartCandle, []ChartOscillator, []strategy.ChartAnnotation) {
	if len(klines) == 0 {
		return nil, nil, nil
	}
	if err := requestCtxErr(ctx); err != nil {
		return nil, nil, nil
	}
	cfg := strategy.ChartStreamingReplayConfig(settings, interval)
	acc := strategy.NewStreamingReplayAccumulatorCtx(ctx, klines, cfg)
	if err := acc.LastReplayErr(); err != nil {
		return nil, nil, nil
	}
	result := acc.Result()
	candles, oscillators, annotations := chartSeriesFromReplayResult(result, true)
	if trim <= 0 || len(candles) <= trim {
		return candles, oscillators, trimAnnotations(annotations, trim, klines)
	}
	return candles[trim:], oscillators[trim:], trimAnnotations(annotations, trim, klines)
}
