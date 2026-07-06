package server

import (
	"context"
	"log"

	"trading_bot/exchange"
	"trading_bot/strategy"
)

func (d *DashboardServer) buildLiveChartFromRAM(
	analyst *strategy.Marker,
	klines []exchange.Kline,
	settings strategy.RSXSettings,
) ([]ChartCandle, []ChartOscillator, []strategy.ChartAnnotation, bool) {
	if analyst == nil || len(klines) == 0 {
		return nil, nil, nil, false
	}
	if !strategy.RSXSettingsEqual(analyst.EffectiveRSXSettings(), settings) {
		return nil, nil, nil, false
	}
	result, ok := strategy.ExportChartSeriesForWindow(analyst, klines, settings)
	if !ok {
		return nil, nil, nil, false
	}
	if len(result.ChartPoints) > stateTailPollLimit+strategy.IndicatorWarmupBars {
		log.Printf("[ChartCache] RAM marker export: %d bars (zero replay)", len(result.ChartPoints))
	}
	candles, oscillators, annotations := chartSeriesFromReplayResult(result, true)
	return candles, oscillators, annotations, true
}

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

// buildTailPollChartFromRAM serves lightweight GET /api/state?poll=1 requests:
// one live candle + one oscillator point from Marker RAM (no chart export walk).
func (d *DashboardServer) buildTailPollChartFromRAM(
	analyst *strategy.Marker,
	klines []exchange.Kline,
) ([]ChartCandle, []ChartOscillator, []strategy.ChartAnnotation) {
	if analyst == nil || len(klines) == 0 {
		return nil, nil, nil
	}
	lastK := klines[len(klines)-1]
	candle, ok := ChartCandleFromKline(lastK)
	if !ok {
		return nil, nil, nil
	}
	osc := tailOscillatorFromSnapshot(analyst.TailPollSnapshot(), lastK)
	return []ChartCandle{candle}, []ChartOscillator{osc}, nil
}

func tailOscillatorFromSnapshot(snap strategy.MarkerTailPollSnapshot, k exchange.Kline) ChartOscillator {
	sig := snap.Falcon
	barTimeSec := exchange.ChartTimeSec(k.OpenTime)
	osc := ChartOscillator{
		Time:           barTimeSec,
		Jurik:          sig.JurikRSX,
		RSX:            sig.JurikRSX,
		RSXSignal:      sig.JurikRSXSignal,
		Red:            sig.RedLine,
		Green:          sig.GreenLine,
		RedLine:        sig.RedLine,
		GreenLine:      sig.GreenLine,
		Blue:           sig.BlueLine,
		RsiPrice:       sig.RsiPrice,
		EmaRsi:         sig.EmaRsi,
		RsiRsi:         sig.RsiRsi,
		RsiHl2:         sig.RsiHl2,
		RsiVolFast:     sig.RsiVolFast,
		RsiVolSlow:     sig.RsiVolSlow,
		MacdRsi:        sig.MacdRsi,
		RsiAd:          sig.RsiAd,
		RsiHl2Vol:      sig.RsiHl2Vol,
		VolCrossMarker: sig.VolCrossMarker,
		VolChanMid:     sig.VolChanMid,
		VolChanUp:      sig.VolChanUp,
		VolChanDn:      sig.VolChanDn,
		PriceChanMid:   sig.PriceChanMid,
		PriceChanUp:    sig.PriceChanUp,
		PriceChanDn:    sig.PriceChanDn,
		Color:          snap.RSXColor,
		Marker:         snap.RSXMarker,
	}
	return compactChartOscillator(osc)
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
