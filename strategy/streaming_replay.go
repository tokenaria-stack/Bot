package strategy

import (
	"context"
	"log"
	"strings"

	"trading_bot/exchange"
)

// StreamingReplayConfig drives the unified walk-forward replay loop.
type StreamingReplayConfig struct {
	Symbol         string
	Interval       string
	Matrix         ScoringMatrix
	LongThreshold  int
	ShortThreshold int
	RSXSettings    RSXSettings
	EntryAnalyst   *Analyst
	ChaosCfg       ChaosConfig

	// EnableTrading runs the backtest FSM (entries/exits). Chart-only replay leaves this false.
	EnableTrading    bool
	InitialCapital   float64
	FeeRate          float64
	SlippagePct      float64
	HTF              *exchange.HTFProvider
	Navigator        NavigatorUISettings
	Navigators       map[string]NavigatorUISettings
	// LightweightMode runs the chart-only fast path (no trade FSM, RSX-only annotations).
	LightweightMode bool
	// SimOnly writes BacktestSimPoint slices directly (no BacktestChartPoint allocation).
	SimOnly bool
	// SkipNavigators omits histRSX/histWozduh accumulation (navigator assembly runs elsewhere).
	SkipNavigators bool
}

// StreamingReplayResult is the output of one full streaming replay pass.
type StreamingReplayResult struct {
	ChartPoints    []BacktestChartPoint
	SimPoints      []BacktestSimPoint
	Annotations    []ChartAnnotation
	ClosedTrades   []backtestClosedTrade
	EquityCurve    []BacktestEquityPoint
	HistKlines     []exchange.Kline
	HistRSX        []float64
	HistWozduh     []float64
	FinalBalance   float64
	MaxDrawdownPct float64
	MaxDrawdownUSD float64
	Cancelled      bool
}

// ChartStreamingReplayConfig builds a chart-only replay config (no trade FSM).
func ChartStreamingReplayConfig(settings RSXSettings, interval string) StreamingReplayConfig {
	longTh, shortTh := DefaultScoreThreshold, DefaultScoreThreshold
	return StreamingReplayConfig{
		Interval:        interval,
		Matrix:          chartLightweightMatrix(),
		LongThreshold:   longTh,
		ShortThreshold:  shortTh,
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
	matrix := scoringMatrixSnapshot()
	if cfg.Matrix != nil {
		matrix = *cfg.Matrix
	}
	longTh := cfg.LongThreshold
	if longTh <= 0 {
		longTh = DefaultScoreThreshold
	}
	shortTh := cfg.ShortThreshold
	if shortTh <= 0 {
		shortTh = DefaultScoreThreshold
	}
	fee := cfg.FeeRate
	if fee <= 0 {
		fee = DefaultScalpFeeRate
	}
	slip := cfg.SlippagePct
	if slip <= 0 {
		slip = DefaultBacktestSlippagePct
	}
	return StreamingReplayConfig{
		Symbol:         cfg.Symbol,
		Interval:       cfg.Interval,
		Matrix:         matrix,
		LongThreshold:  longTh,
		ShortThreshold: shortTh,
		RSXSettings:    rsxCfg,
		EntryAnalyst:   cfg.EntryAnalyst,
		ChaosCfg:       ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34},
		EnableTrading:  true,
		InitialCapital: BacktestInitialCapital,
		FeeRate:        fee,
		SlippagePct:    slip,
		HTF:            cfg.HTF,
		Navigator:      cfg.Navigator,
		Navigators:     cfg.Navigators,
		SimOnly:        cfg.SimOnly,
		SkipNavigators: cfg.SkipNavigators,
	}
}

// RunStreamingReplay walks klines once through Marker + scoring (and optional trade FSM).
func RunStreamingReplay(ctx context.Context, klines []exchange.Kline, cfg StreamingReplayConfig) *StreamingReplayResult {
	if len(klines) == 0 {
		return &StreamingReplayResult{}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cfg.RSXSettings = NormalizeRSXSettings(cfg.RSXSettings)
	if cfg.LightweightMode && !cfg.EnableTrading {
		return NewStreamingReplayAccumulator(klines, cfg).Result()
	}
	return runStreamingReplayFull(ctx, klines, cfg)
}

func runStreamingReplayFull(ctx context.Context, klines []exchange.Kline, cfg StreamingReplayConfig) *StreamingReplayResult {
	result := &StreamingReplayResult{}
	marker := NewMarker(nil, nil, cfg.Interval, "", cfg.ChaosCfg)
	marker.ApplyBacktestRSXConfig(cfg.RSXSettings)
	chief := NewChiefAnalyst()

	balance := cfg.InitialCapital
	if balance <= 0 {
		balance = BacktestInitialCapital
	}
	peak := balance

	var pos *btPosition
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
	annotations := make([]ChartAnnotation, 0, 32)
	equity := []BacktestEquityPoint{{Time: klines[0].OpenTime / 1000, Value: balance}}
	var closedTrades []backtestClosedTrade

	var mtfTracker *WalkForwardMTFTracker
	if cfg.EnableTrading {
		if mtfTFs := CollectWalkForwardMTFPeriods(cfg.Navigators, cfg.Interval); cfg.HTF != nil && len(mtfTFs) > 0 {
			priceUI := cfg.Navigators["price"]
			mtfTracker = NewWalkForwardMTFTracker(cfg.HTF, cfg.Symbol, cfg.Interval, priceUI, mtfTFs)
			mtfTracker.SetChartStartMs(klines[0].OpenTime)
			mtfTracker.Prefetch()
		}
	}

	var tradeEngine *BacktestEngine
	if cfg.EnableTrading {
		tradeEngine = NewBacktestEngine(BacktestConfig{
			Symbol:         cfg.Symbol,
			Interval:       cfg.Interval,
			EntryAnalyst:   cfg.EntryAnalyst,
			FeeRate:        cfg.FeeRate,
			SlippagePct:    cfg.SlippagePct,
			Matrix:         &cfg.Matrix,
			Navigator:      cfg.Navigator,
			Navigators:     cfg.Navigators,
			HTF:            cfg.HTF,
			LongThreshold:  cfg.LongThreshold,
			ShortThreshold: cfg.ShortThreshold,
			RSXSettings:    &cfg.RSXSettings,
		})
	}

	lastIdx := len(klines) - 1
	cancelled := false
	lastProcessedIdx := -1

	for i, kline := range klines {
		select {
		case <-ctx.Done():
			cancelled = true
		default:
		}
		if cancelled {
			break
		}

		lastProcessedIdx = i
		kline = exchange.NormalizeKline(kline)
		barTimeSec := kline.OpenTime / 1000

		marker.UpdateKlineTick(kline, true)
		histKlines = append(histKlines, kline)
		falcon := marker.FalconSnapshot()
		if !cfg.SkipNavigators {
			histRSX = append(histRSX, falcon.JurikRSX)
			histWozduh = append(histWozduh, falcon.RsiVolSlow)
		}

		if mtfTracker != nil {
			mtfTracker.Update(barTimeSec, histKlines)
			marker.SetCurrentMTFState(mtfTracker.States())
		}

		scoreReady := marker.HasMinBars(IndicatorWarmupBars)
		var decision ScoreDecision
		var rsxMarker string
		if scoreReady {
			decision = evaluateStreamingDecision(marker, chief, cfg)
			if ann, ok := rsxAnnotationFromMarker(marker, barTimeSec); ok {
				appendStreamingRSXAnnotation(&annotations, ann)
				rsxMarker = ann.Label
			}

			if cfg.EnableTrading && tradeEngine != nil {
				candle := klineToCandle(kline)
				stoppedThisBar := false

				if pos != nil {
					if rawStop, hit := checkFractalStop(pos, candle); hit {
						exitPrice := applyExitSlippage(pos.side, rawStop, cfg.SlippagePct)
						balance, peak, result.MaxDrawdownPct, result.MaxDrawdownUSD = tradeEngine.closeBacktestPosition(
							pos, exitPrice, barTimeSec, "stop", balance, &closedTrades, &equity, peak, result.MaxDrawdownPct, result.MaxDrawdownUSD,
						)
						pos = nil
						stoppedThisBar = true
					}
				}

				if pos != nil {
					switch pos.side {
					case "BUY":
						if decision.FinalAction == SellAction {
							exitPrice := applyExitSlippage(pos.side, candle.Close, cfg.SlippagePct)
							balance, peak, result.MaxDrawdownPct, result.MaxDrawdownUSD = tradeEngine.closeBacktestPosition(
								pos, exitPrice, barTimeSec, "signal", balance, &closedTrades, &equity, peak, result.MaxDrawdownPct, result.MaxDrawdownUSD,
							)
							pos = nil
							logBacktestSignal(i, decision, "reversal")
							tryOpenBacktestPosition(tradeEngine, &pos, histKlines, i, "SELL", candle, decision, "reversal", marker, &balance, cfg.SlippagePct)
						}
					case "SELL":
						if decision.FinalAction == BuyAction {
							exitPrice := applyExitSlippage(pos.side, candle.Close, cfg.SlippagePct)
							balance, peak, result.MaxDrawdownPct, result.MaxDrawdownUSD = tradeEngine.closeBacktestPosition(
								pos, exitPrice, barTimeSec, "signal", balance, &closedTrades, &equity, peak, result.MaxDrawdownPct, result.MaxDrawdownUSD,
							)
							pos = nil
							logBacktestSignal(i, decision, "reversal")
							tryOpenBacktestPosition(tradeEngine, &pos, histKlines, i, "BUY", candle, decision, "reversal", marker, &balance, cfg.SlippagePct)
						}
					}
				} else if decision.FinalAction == BuyAction || decision.FinalAction == SellAction {
					signalKind := "fresh"
					if stoppedThisBar {
						signalKind = "stop-reverse"
					}
					logBacktestSignal(i, decision, signalKind)
					tryOpenBacktestPosition(tradeEngine, &pos, histKlines, i, string(decision.FinalAction), candle, decision, signalKind, marker, &balance, cfg.SlippagePct)
				}

				tradeEngine.logRSXSignalIgnored(i, rsxMarker, decision)

				if i > 0 && i%backtestEquityEvery == 0 {
					recordEquityPoint(&equity, barTimeSec, balance)
					peak, result.MaxDrawdownPct, result.MaxDrawdownUSD = updateDrawdown(balance, peak, result.MaxDrawdownPct, result.MaxDrawdownUSD)
				}
			}
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
			if rsxMarker != "" {
				simPt.Marker = rsxMarker
			} else if ann, ok := rsxAnnotationFromMarker(marker, barTimeSec); ok {
				simPt.Marker = ann.Label
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
			if scoreReady {
				pt.LongScore = decision.LongScore
				pt.ShortScore = decision.ShortScore
				pt.RawAction = string(decision.RawAction)
				pt.FinalAction = string(decision.FinalAction)
				pt.IsVetoed = decision.IsVetoed
				pt.VetoReason = decision.VetoReason
				if decision.FinalAction == BuyAction || decision.FinalAction == SellAction {
					pt.Factors = decision.Factors
				}
				if rsxMarker != "" {
					pt.Marker = rsxMarker
				}
			}
			chartData = append(chartData, pt)
		}

		prevRSX = falcon.JurikRSX
		prevRSXReady = true
		prevBlue = falcon.BlueLine
		prevBlueReady = true
	}

	if cfg.EnableTrading && tradeEngine != nil && !cancelled && pos != nil {
		last := klines[lastIdx]
		barTimeSec := last.OpenTime / 1000
		exitPrice := applyExitSlippage(pos.side, last.Close, cfg.SlippagePct)
		balance, peak, result.MaxDrawdownPct, result.MaxDrawdownUSD = tradeEngine.closeBacktestPosition(
			pos, exitPrice, barTimeSec, "eod", balance, &closedTrades, &equity, peak, result.MaxDrawdownPct, result.MaxDrawdownUSD,
		)
	} else if cfg.EnableTrading && tradeEngine != nil && cancelled && pos != nil && lastProcessedIdx >= 0 {
		last := klines[lastProcessedIdx]
		barTimeSec := last.OpenTime / 1000
		exitPrice := applyExitSlippage(pos.side, last.Close, cfg.SlippagePct)
		balance, peak, result.MaxDrawdownPct, result.MaxDrawdownUSD = tradeEngine.closeBacktestPosition(
			pos, exitPrice, barTimeSec, "stopped", balance, &closedTrades, &equity, peak, result.MaxDrawdownPct, result.MaxDrawdownUSD,
		)
	}

	result.ChartPoints = chartData
	result.SimPoints = simData
	result.Annotations = annotations
	result.ClosedTrades = closedTrades
	result.EquityCurve = equity
	result.HistKlines = histKlines
	result.HistRSX = histRSX
	result.HistWozduh = histWozduh
	result.FinalBalance = balance
	result.Cancelled = cancelled
	return result
}

func evaluateStreamingDecision(marker *Marker, chief *ChiefAnalyst, cfg StreamingReplayConfig) ScoreDecision {
	decision := DefaultScoreEngine.CalculateWithThresholds(marker, cfg.Matrix, cfg.LongThreshold, cfg.ShortThreshold)
	analyst := cfg.EntryAnalyst
	if analyst == nil {
		analyst = NewAnalyst(false)
	}
	return ApplyExecutionVetoes(decision, marker, analyst, chief)
}

func rsxAnnotationFromMarker(marker *Marker, barTimeSec int64) (ChartAnnotation, bool) {
	if marker == nil {
		return ChartAnnotation{}, false
	}
	snap, ok := marker.scoreSnapshot()
	if !ok || snap.rsxMarker == "" {
		return ChartAnnotation{}, false
	}
	return rsxAnnotationFromLabel(snap.rsxMarker, barTimeSec)
}

func rsxAnnotationFromDecision(decision ScoreDecision, barTimeSec int64) (ChartAnnotation, bool) {
	f, ok := decision.Factors["RSX"]
	if !ok || f.Score <= 0 {
		return ChartAnnotation{}, false
	}
	label := rsxLabelFromFactorName(f.Name)
	if label == "" {
		return ChartAnnotation{}, false
	}
	color, position, shape := rsxAnnotationStyle(label)
	return ChartAnnotation{
		Time:     barTimeSec,
		Pane:     normalizeAnnotationPane("rsx"),
		Label:    label,
		Color:    color,
		Position: position,
		Shape:    shape,
	}, true
}

func rsxLabelFromFactorName(name string) string {
	switch strings.TrimSpace(name) {
	case "RSX LL":
		return "LL"
	case "RSX L":
		return "L"
	case "RSX SS":
		return "SS"
	case "RSX S":
		return "S"
	default:
		return ""
	}
}

func appendStreamingRSXAnnotation(annotations *[]ChartAnnotation, ann ChartAnnotation) {
	for i, existing := range *annotations {
		if existing.Time == ann.Time && existing.Pane == ann.Pane {
			if rsxMarkerChartStrength(ann.Label) > rsxMarkerChartStrength(existing.Label) {
				(*annotations)[i] = ann
			}
			return
		}
	}
	*annotations = append(*annotations, ann)
}

func tryOpenBacktestPosition(
	engine *BacktestEngine,
	pos **btPosition,
	histKlines []exchange.Kline,
	barIndex int,
	side string,
	candle exchange.Candle,
	decision ScoreDecision,
	signalKind string,
	marker *Marker,
	balance *float64,
	slippagePct float64,
) bool {
	barTimeSec := candle.OpenTime / 1000
	entryPrice := applyEntrySlippage(side, candle.Close, slippagePct)
	audit := auditFromDecision(decision, side, barIndex, signalKind)
	log.Printf("[Backtest ENTRY] Bar %d: Action=%s, Kind=%s, Score=%f. Reason=%s | Factors: %v",
		barIndex, side, signalKind, audit.score, audit.entryReason, audit.factorsSnapshot)
	posSlot, opened := engine.openPosition(
		histKlines, barIndex, side,
		entryPrice, barTimeSec, marker.LastATR(), *balance, decision.LotMod, audit,
	)
	if opened {
		*pos = &posSlot
		return true
	}
	log.Printf("[Execution Veto] Bar %d: %s signal dropped. Reason: Invalid Stop or Qty", barIndex, side)
	return false
}

func klineToCandle(k exchange.Kline) exchange.Candle {
	return exchange.Candle{
		OpenTime:  k.OpenTime,
		Open:      k.Open,
		High:      k.High,
		Low:       k.Low,
		Close:     k.Close,
		Volume:    k.Volume,
		CloseTime: k.CloseTime,
	}
}
