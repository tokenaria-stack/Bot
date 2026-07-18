package strategy

import (
	"context"

	"trading_bot/exchange"
)

// StreamingReplayConfig drives the unified walk-forward replay loop (chart-only).
// The trade FSM was purged in Core 5.0 Phase F; strategies plug back in later.
type StreamingReplayConfig struct {
	Symbol      string
	Interval    string
	RSXSettings RSXSettings
	ChaosCfg    ChaosConfig

	HTF        *exchange.HTFProvider
	Navigator  NavigatorUISettings
	Navigators map[string]NavigatorUISettings
	// LightweightMode runs the chart-only fast path.
	LightweightMode bool
	// SimOnly writes BacktestSimPoint slices directly (no BacktestChartPoint allocation).
	SimOnly bool
	// SkipNavigators omits histRSX/histWozduh accumulation (navigator assembly runs elsewhere).
	SkipNavigators bool
}

// StreamingReplayResult is the output of one full streaming replay pass.
type StreamingReplayResult struct {
	ChartPoints []BacktestChartPoint
	SimPoints   []BacktestSimPoint
	Annotations []ChartAnnotation
	HistKlines  []exchange.Kline
	HistRSX     []float64
	HistWozduh  []float64
	Cancelled   bool
}

// ChartStreamingReplayConfig builds a chart-only replay config.
func ChartStreamingReplayConfig(settings RSXSettings, interval string) StreamingReplayConfig {
	return StreamingReplayConfig{
		Interval:        interval,
		RSXSettings:     NormalizeRSXSettings(settings),
		ChaosCfg:        ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34},
		LightweightMode: true,
	}
}

// StreamingReplayConfigFromBacktest maps backtest engine settings to replay config.
func StreamingReplayConfigFromBacktest(cfg BacktestConfig) StreamingReplayConfig {
	rsxCfg := GetRSXSettings()
	if cfg.RSXSettings != nil {
		rsxCfg = NormalizeRSXSettings(*cfg.RSXSettings)
	}
	return StreamingReplayConfig{
		Symbol:         cfg.Symbol,
		Interval:       cfg.Interval,
		RSXSettings:    rsxCfg,
		ChaosCfg:       ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34},
		HTF:            cfg.HTF,
		Navigator:      cfg.Navigator,
		Navigators:     cfg.Navigators,
		SimOnly:        cfg.SimOnly,
		SkipNavigators: cfg.SkipNavigators,
	}
}

// RunStreamingReplay walks klines once through Marker indicators (chart-only replay).
func RunStreamingReplay(ctx context.Context, klines []exchange.Kline, cfg StreamingReplayConfig) *StreamingReplayResult {
	if len(klines) == 0 {
		return &StreamingReplayResult{}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cfg.RSXSettings = NormalizeRSXSettings(cfg.RSXSettings)
	if cfg.LightweightMode {
		return NewStreamingReplayAccumulatorCtx(ctx, klines, cfg).Result()
	}
	return runStreamingReplayFull(ctx, klines, cfg)
}

func runStreamingReplayFull(ctx context.Context, klines []exchange.Kline, cfg StreamingReplayConfig) *StreamingReplayResult {
	result := &StreamingReplayResult{}
	marker := NewMarker(nil, nil, cfg.Interval, "", cfg.ChaosCfg)
	marker.ApplyBacktestRSXConfig(cfg.RSXSettings)

	var prevBlue float64
	var prevBlueReady bool
	var prevRSX float64
	var prevRSXReady bool

	histKlines := make([]exchange.Kline, 0, len(klines))
	var histRSX, histWozduh []float64
	if !cfg.SkipNavigators {
		histRSX = make([]float64, 0, len(klines))
		histWozduh = make([]float64, 0, len(klines))
	}
	var chartData []BacktestChartPoint
	var simData []BacktestSimPoint
	if cfg.SimOnly {
		simData = make([]BacktestSimPoint, 0, len(klines))
	} else {
		chartData = make([]BacktestChartPoint, 0, len(klines))
	}

	cancelled := false

	for _, kline := range klines {
		select {
		case <-ctx.Done():
			cancelled = true
		default:
		}
		if cancelled {
			break
		}

		kline = exchange.NormalizeKline(kline)
		barTimeSec := kline.OpenTime / 1000

		marker.UpdateKlineTick(kline, true)
		histKlines = append(histKlines, kline)
		falcon := marker.FalconSnapshot()
		if !cfg.SkipNavigators {
			histRSX = append(histRSX, falcon.JurikRSX)
			histWozduh = append(histWozduh, falcon.RsiVolSlow)
		}

		if cfg.SimOnly {
			simPt := BacktestSimPoint{
				Time:           barTimeSec,
				Jurik:          falcon.JurikRSX,
				RSX:            falcon.JurikRSX,
				RSXSignal:      falcon.JurikRSXSignal,
				RsiVolFast:     falcon.RsiVolFast,
				RsiVolSlow:     falcon.RsiVolSlow,
				VolCrossMarker: falcon.VolCrossMarker,
			}
			if prevRSXReady {
				simPt.Color = RSXColor(simPt.RSX, prevRSX)
			}
			if prevBlueReady {
				simPt.VolumeSpikeUp = DetectWozduxVolumeSpikeUp(prevBlue, falcon.BlueLine, falcon.RedLine)
				simPt.VolumeSpikeDown = DetectWozduxVolumeSpikeDown(prevBlue, falcon.BlueLine, falcon.RedLine)
			}
			simData = append(simData, simPt)
		} else {
			pt := BacktestChartPoint{
				Time:   barTimeSec,
				Open:   kline.Open,
				High:   kline.High,
				Low:    kline.Low,
				Close:  kline.Close,
				Volume: kline.Volume,
			}
			populateBacktestPointFromMarker(&pt, marker, prevBlue, prevBlueReady)
			if prevRSXReady {
				pt.Color = RSXColor(pt.RSX, prevRSX)
			}
			chartData = append(chartData, pt)
		}

		prevRSX = falcon.JurikRSX
		prevRSXReady = true
		prevBlue = falcon.BlueLine
		prevBlueReady = true
	}

	result.ChartPoints = chartData
	result.SimPoints = simData
	result.HistKlines = histKlines
	result.HistRSX = histRSX
	result.HistWozduh = histWozduh
	result.Cancelled = cancelled
	return result
}
