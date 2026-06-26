package strategy

import (
	"context"
	"fmt"
	"log"
	"time"

	"trading_bot/execution"
	"trading_bot/exchange"
)

const (
	BacktestInitialCapital     = 10000.0
	DefaultBacktestSlippagePct = 0.03 // 0.03% per fill
	backtestMinBars            = 50
	backtestEquityEvery        = 100
)

// BacktestMinBars returns the minimum candle count required before trading in backtests.
func BacktestMinBars() int {
	return backtestMinBars
}

const (
	backtestPadMinDailyDays   = 60
	backtestPadExtraDailyDays = 30
	backtestPadMaxDailyDays   = 365
	backtestPadExtraWeekly    = 4
	backtestPadExtraMonthly   = 2
)

// BacktestPadStartDays returns how many calendar days to extend start backward for coarse TFs.
func BacktestPadStartDays(binanceInterval string, have, need int) int {
	if have >= need {
		return 0
	}
	missing := need - have
	switch binanceInterval {
	case "1d":
		pad := missing + backtestPadExtraDailyDays
		if pad < backtestPadMinDailyDays {
			pad = backtestPadMinDailyDays
		}
		if pad > backtestPadMaxDailyDays {
			pad = backtestPadMaxDailyDays
		}
		return pad
	case "1w":
		weeks := missing + backtestPadExtraWeekly
		return weeks * 7
	case "1M":
		months := missing + backtestPadExtraMonthly
		return months * 30
	default:
		return 0
	}
}

// PadBacktestStartMs shifts start backward when coarse intervals lack enough candles.
// Returns new start ms and whether padding was applied.
func PadBacktestStartMs(binanceInterval string, startMs, endMs int64, candleCount int) (int64, bool) {
	padDays := BacktestPadStartDays(binanceInterval, candleCount, backtestMinBars)
	if padDays <= 0 {
		return startMs, false
	}
	newStart := startMs - int64(padDays)*24*time.Hour.Milliseconds()
	if newStart < 0 {
		newStart = 0
	}
	if newStart >= startMs {
		return startMs, false
	}
	return newStart, true
}

// BacktestConfig configures a historical simulation run.
type BacktestConfig struct {
	Symbol       string
	Interval     string
	EntryAnalyst *Analyst
	FeeRate      float64
	SlippagePct  float64
	Matrix       *ScoringMatrix
	Navigator    NavigatorUISettings
	Navigators   map[string]NavigatorUISettings
	HTF          *exchange.HTFProvider
}

func (e *BacktestEngine) activeMatrix() ScoringMatrix {
	if e.cfg.Matrix != nil {
		return *e.cfg.Matrix
	}
	return scoringMatrixSnapshot()
}

// BacktestChartPoint is one candle + full indicator snapshot for the backtest chart.
type BacktestChartPoint struct {
	Time  int64
	Open  float64
	High  float64
	Low   float64
	Close float64
	Volume float64

	Jurik          float64
	RSX            float64
	RSXSignal      float64
	RsiPrice       float64
	EmaRsi         float64
	RsiRsi         float64
	RsiHl2         float64
	RsiVolFast     float64
	RsiVolSlow     float64
	MacdRsi        float64
	RsiAd          float64
	RsiHl2Vol      float64
	VolCrossMarker string
	VolChanMid     float64
	VolChanUp      float64
	VolChanDn      float64
	PriceChanMid   float64
	PriceChanUp    float64
	PriceChanDn    float64
	Color          string
	Marker         string
	VolumeSpikeUp   bool
	VolumeSpikeDown bool

	// Legacy aliases (still populated for compatibility).
	WozduhUp   float64
	WozduhDown float64

	LongScore  int                        `json:"longScore"`
	ShortScore int                        `json:"shortScore"`
	RawAction   string                    `json:"rawAction"`
	FinalAction string                    `json:"finalAction"`
	IsVetoed    bool                      `json:"isVetoed"`
	VetoReason  string                    `json:"vetoReason,omitempty"`
	Factors    map[string]ScoreFactor     `json:"factors,omitempty"`
}

// BacktestTradeResult is one completed round-trip trade.
type BacktestTradeResult struct {
	Time          int64
	EntryTime     int64
	Side          string
	EntryPrice    float64
	ExitPrice     float64
	StopLossPrice float64
	ExitReason    string
	PnL           float64
	Duration      string
}

// BacktestEquityPoint is a balance snapshot for the equity curve.
type BacktestEquityPoint struct {
	Time  int64
	Value float64
}

// BacktestRunResult aggregates simulation output and performance metrics.
type BacktestRunResult struct {
	TotalTrades    int
	WinRate        float64
	NetProfit      float64
	ProfitFactor   float64
	MaxDrawdown    float64
	RecoveryFactor float64
	Cancelled      bool
	Trades         []BacktestTradeResult
	EquityCurve    []BacktestEquityPoint
	ChartData      []BacktestChartPoint
	NavigatorData  NavigatorResultDTO
	Navigators     map[string]NavigatorResultDTO
}

// BacktestEngine replays historical candles through the scoring pipeline.
type BacktestEngine struct {
	cfg BacktestConfig
}

// NewBacktestEngine creates a backtest runner with strategy defaults applied.
func NewBacktestEngine(cfg BacktestConfig) *BacktestEngine {
	if cfg.FeeRate <= 0 {
		cfg.FeeRate = DefaultScalpFeeRate
	}
	if cfg.SlippagePct <= 0 {
		cfg.SlippagePct = DefaultBacktestSlippagePct
	}
	return &BacktestEngine{cfg: cfg}
}

type btPosition struct {
	side       string
	entryPrice float64
	entryTime  int64
	stopPrice  float64
	qty        float64
}

// btPendingEntry is a signal queued on bar N for execution at bar N+1 open.
type btPendingEntry struct {
	side           string
	lotMod         float64
	signalBarIndex int
	atr            float64
}

func (p *btPosition) displaySide() string {
	if p.side == "SELL" {
		return "SHORT"
	}
	return "LONG"
}

// buildBacktestRSXMarkersMap precomputes L/LL/S/SS markers for the full history (same scan as RSX chart).
func buildBacktestRSXMarkersMap(candles []exchange.Candle) map[int64]string {
	n := len(candles)
	if n == 0 {
		return nil
	}

	settings := GetRSXSettings()
	lookback := settings.DivLookback
	if lookback <= 0 {
		lookback = RSXLookbackDefault
	}

	klines := make([]exchange.Kline, n)
	rsxValues := make([]float64, n)
	closes := make([]float64, n)
	falcon := NewFalconEngine()

	for i, c := range candles {
		klines[i] = exchange.Kline{
			OpenTime: c.OpenTime,
			Open:     c.Open,
			High:     c.High,
			Low:      c.Low,
			Close:    c.Close,
			Volume:   c.Volume,
		}
		closes[i] = c.Close
		rsxValues[i] = falcon.Evaluate(c.High, c.Low, c.Close, c.Volume).JurikRSX
	}

	prices := buildRSXPriceSeries(klines, settings.Source)

	var indexMarkers map[int]string
	switch normalizeRSXDivMethod(settings.DivMethod) {
	case "fractal":
		indexMarkers = scanRSXFractalMarkers(prices, rsxValues, lookback, settings.PivotRadius)
	default:
		indexMarkers = scanRSXTVMarkers(closes, rsxValues, lookback)
	}

	out := make(map[int64]string, len(indexMarkers))
	for idx, marker := range indexMarkers {
		if marker == "" || idx < 0 || idx >= n {
			continue
		}
		out[candles[idx].CloseTime] = marker
	}
	return out
}

func (e *BacktestEngine) evaluateBacktestDecision(marker *Marker, chief *ChiefAnalyst) ScoreDecision {
	matrix := e.activeMatrix()
	decision := DefaultScoreEngine.Calculate(marker, matrix)
	analyst := e.cfg.EntryAnalyst
	if analyst == nil {
		analyst = NewAnalyst(false)
	}
	return ApplyExecutionVetoes(decision, marker, analyst, chief)
}

// Run simulates the strategy over historical candles and returns performance stats.
// When ctx is cancelled the loop stops early and returns partial results with Cancelled=true.
func (e *BacktestEngine) Run(ctx context.Context, candles []exchange.Candle) (*BacktestRunResult, error) {
	if len(candles) == 0 {
		return nil, fmt.Errorf("no candles to backtest")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	log.Printf("[Backtest] Processing %d candles", len(candles))

	markersMap := buildBacktestRSXMarkersMap(candles)

	chaosCfg := ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34}
	marker := NewMarker(nil, nil, e.cfg.Interval, "", chaosCfg)
	chief := NewChiefAnalyst()

	balance := BacktestInitialCapital
	peak := balance
	maxDrawdownPct := 0.0
	maxDrawdownUSD := 0.0

	var closedTrades []backtestClosedTrade
	equity := []BacktestEquityPoint{{Time: candles[0].OpenTime / 1000, Value: balance}}
	chartData := make([]BacktestChartPoint, 0, len(candles))

	var pos *btPosition
	var pending *btPendingEntry
	var prevBlue float64
	var prevBlueReady bool
	var prevRSX float64
	var prevRSXReady bool

	var kline exchange.Kline
	var pt BacktestChartPoint
	var scoreReady bool
	var decision ScoreDecision

	histKlines := make([]exchange.Kline, 0, len(candles))
	histRSX := make([]float64, 0, len(candles))
	histWozduh := make([]float64, 0, len(candles))

	var mtfTracker *WalkForwardMTFTracker
	if mtfTFs := CollectWalkForwardMTFPeriods(e.cfg.Navigators, e.cfg.Interval); e.cfg.HTF != nil && len(mtfTFs) > 0 {
		priceUI := e.cfg.Navigators["price"]
		mtfTracker = NewWalkForwardMTFTracker(e.cfg.HTF, e.cfg.Symbol, e.cfg.Interval, priceUI, mtfTFs)
		if len(candles) > 0 {
			mtfTracker.SetChartStartMs(candles[0].OpenTime)
		}
		mtfTracker.Prefetch()
		log.Printf("[Backtest] Walk-forward MTF tracker: %v", mtfTFs)
	}

	lastIdx := len(candles) - 1
	cancelled := false
	lastProcessedIdx := -1
	for i := range candles {
		select {
		case <-ctx.Done():
			cancelled = true
		default:
		}
		if cancelled {
			break
		}

		candle := candles[i]
		lastProcessedIdx = i
		barTimeSec := candle.OpenTime / 1000

		if pending != nil && pos == nil {
			entryPrice := applyEntrySlippage(pending.side, candle.Open, e.cfg.SlippagePct)
			posSlot, opened := e.openPosition(
				histKlines, pending.signalBarIndex, pending.side,
				entryPrice, barTimeSec, pending.atr, balance, pending.lotMod,
			)
			if opened {
				pos = &posSlot
			}
			pending = nil
		}

		kline.OpenTime = candle.OpenTime
		kline.Open = candle.Open
		kline.High = candle.High
		kline.Low = candle.Low
		kline.Close = candle.Close
		kline.Volume = candle.Volume
		kline = exchange.NormalizeKline(kline)

		barMarker := markersMap[candle.CloseTime]

		marker.UpdateKlineTick(kline, false)
		histKlines = append(histKlines, kline)
		histRSX = append(histRSX, marker.falconSignals.JurikRSX)
		histWozduh = append(histWozduh, marker.falconSignals.RsiVolSlow)

		if mtfTracker != nil {
			mtfTracker.Update(barTimeSec, histKlines)
			marker.SetCurrentMTFState(mtfTracker.States())
		}

		scoreReady = marker.HasMinBars(backtestMinBars)

		if !scoreReady {
			continue
		}

		pt = BacktestChartPoint{
			Time:   barTimeSec,
			Open:   candle.Open,
			High:   candle.High,
			Low:    candle.Low,
			Close:  candle.Close,
			Volume: candle.Volume,
		}
		populateBacktestPointFromMarker(&pt, marker, prevBlue, prevBlueReady)
		if prevRSXReady {
			pt.Color = RSXColor(pt.RSX, prevRSX)
		}
		pt.Marker = barMarker
		if pt.Marker == "" {
			pt.Marker = marker.RecentRSXMarker()
		}
		prevRSX = pt.RSX
		prevRSXReady = true
		falcon := marker.FalconSnapshot()
		prevBlue = falcon.BlueLine
		prevBlueReady = true
		decision = e.evaluateBacktestDecision(marker, chief)
		pt.LongScore = decision.LongScore
		pt.ShortScore = decision.ShortScore
		pt.RawAction = string(decision.RawAction)
		pt.FinalAction = string(decision.FinalAction)
		pt.IsVetoed = decision.IsVetoed
		pt.VetoReason = decision.VetoReason
		pt.Factors = decision.Factors
		chartData = append(chartData, pt)

		if pos != nil {
			if rawStop, hit := checkFractalStop(pos, candle); hit {
				exitPrice := applyExitSlippage(pos.side, rawStop, e.cfg.SlippagePct)
				balance, peak, maxDrawdownPct, maxDrawdownUSD = e.closeBacktestPosition(
					pos, exitPrice, barTimeSec, "stop", balance, &closedTrades, &equity, peak, maxDrawdownPct, maxDrawdownUSD,
				)
				pos = nil
			}
		}

		if pos != nil {
			switch pos.side {
			case "BUY":
				switch decision.FinalAction {
				case BuyAction:
					// already long — ignore
				case SellAction:
					exitPrice := applyExitSlippage(pos.side, candle.Close, e.cfg.SlippagePct)
					balance, peak, maxDrawdownPct, maxDrawdownUSD = e.closeBacktestPosition(
						pos, exitPrice, barTimeSec, "signal", balance, &closedTrades, &equity, peak, maxDrawdownPct, maxDrawdownUSD,
					)
					pos = nil
					if i+1 < len(candles) {
						pending = &btPendingEntry{
							side:           "SELL",
							lotMod:         decision.LotMod,
							signalBarIndex: i,
							atr:            marker.LastATR(),
						}
					}
				}
			case "SELL":
				switch decision.FinalAction {
				case SellAction:
					// already short — ignore
				case BuyAction:
					exitPrice := applyExitSlippage(pos.side, candle.Close, e.cfg.SlippagePct)
					balance, peak, maxDrawdownPct, maxDrawdownUSD = e.closeBacktestPosition(
						pos, exitPrice, barTimeSec, "signal", balance, &closedTrades, &equity, peak, maxDrawdownPct, maxDrawdownUSD,
					)
					pos = nil
					if i+1 < len(candles) {
						pending = &btPendingEntry{
							side:           "BUY",
							lotMod:         decision.LotMod,
							signalBarIndex: i,
							atr:            marker.LastATR(),
						}
					}
				}
			}
		} else if decision.FinalAction == BuyAction || decision.FinalAction == SellAction {
			if i+1 < len(candles) {
				pending = &btPendingEntry{
					side:           string(decision.FinalAction),
					lotMod:         decision.LotMod,
					signalBarIndex: i,
					atr:            marker.LastATR(),
				}
			}
		}

		if i > 0 && i%backtestEquityEvery == 0 {
			recordEquityPoint(&equity, barTimeSec, balance)
			peak, maxDrawdownPct, maxDrawdownUSD = updateDrawdown(balance, peak, maxDrawdownPct, maxDrawdownUSD)
		}
	}

	if !cancelled && pos != nil {
		last := candles[lastIdx]
		barTimeSec := last.OpenTime / 1000
		exitPrice := applyExitSlippage(pos.side, last.Close, e.cfg.SlippagePct)
		balance, peak, maxDrawdownPct, maxDrawdownUSD = e.closeBacktestPosition(
			pos, exitPrice, barTimeSec, "eod", balance, &closedTrades, &equity, peak, maxDrawdownPct, maxDrawdownUSD,
		)
	} else if cancelled && pos != nil && lastProcessedIdx >= 0 {
		last := candles[lastProcessedIdx]
		barTimeSec := last.OpenTime / 1000
		exitPrice := applyExitSlippage(pos.side, last.Close, e.cfg.SlippagePct)
		balance, peak, maxDrawdownPct, maxDrawdownUSD = e.closeBacktestPosition(
			pos, exitPrice, barTimeSec, "stopped", balance, &closedTrades, &equity, peak, maxDrawdownPct, maxDrawdownUSD,
		)
	}

	if cancelled {
		log.Printf("[Backtest] Cancelled after %d/%d candles", lastProcessedIdx+1, len(candles))
	}

	return e.assembleRunResult(
		balance, closedTrades, equity, chartData,
		histKlines, histRSX, histWozduh,
		maxDrawdownPct, maxDrawdownUSD, cancelled,
	), nil
}

func (e *BacktestEngine) assembleRunResult(
	balance float64,
	closedTrades []backtestClosedTrade,
	equity []BacktestEquityPoint,
	chartData []BacktestChartPoint,
	histKlines []exchange.Kline,
	histRSX, histWozduh []float64,
	maxDrawdownPct, maxDrawdownUSD float64,
	cancelled bool,
) *BacktestRunResult {
	metrics := computeBacktestMetrics(BacktestInitialCapital, balance, closedTrades, maxDrawdownPct, maxDrawdownUSD)

	trades := make([]BacktestTradeResult, len(closedTrades))
	for i, t := range closedTrades {
		trades[i] = BacktestTradeResult{
			Time:          t.timeSec,
			EntryTime:     t.entryTime,
			Side:          t.side,
			EntryPrice:    t.entryPrice,
			ExitPrice:     t.exitPrice,
			StopLossPrice: t.stopLossPrice,
			ExitReason:    t.exitReason,
			PnL:           t.pnlPct,
			Duration:      t.duration,
		}
	}

	applyBacktestRSXMarkers(chartData, histKlines, histRSX)

	navigators := BuildAllNavigators(e.cfg.Navigators, e.cfg.Symbol, histKlines, histRSX, histWozduh, e.cfg.Interval, e.cfg.HTF)
	navData := NavigatorResultDTO{}
	if len(navigators) > 0 {
		if priceNav, ok := navigators["price"]; ok {
			navData = priceNav
		} else {
			for _, v := range navigators {
				navData = v
				break
			}
		}
	} else if e.cfg.Navigator.Enabled {
		startMs := navigatorChartStartMs(histKlines)
		maxTimeSec := navigatorMaxCloseTimeSec(histKlines)
		htfData := loadNavigatorHTFData(e.cfg.HTF, e.cfg.Symbol, e.cfg.Interval, startMs, maxTimeSec, e.cfg.Navigator.Periods)
		navData = BuildNavigatorResult(e.cfg.Navigator, histKlines, histRSX, histWozduh, e.cfg.Interval, e.cfg.HTF, htfData)
		navigators = map[string]NavigatorResultDTO{"price": navData}
	}

	return &BacktestRunResult{
		TotalTrades:    metrics.totalTrades,
		WinRate:        metrics.winRate,
		NetProfit:      metrics.netProfit,
		ProfitFactor:   metrics.profitFactor,
		MaxDrawdown:    metrics.maxDrawdown,
		RecoveryFactor: metrics.recoveryFactor,
		Cancelled:      cancelled,
		Trades:         trades,
		EquityCurve:    equity,
		ChartData:      chartData,
		NavigatorData:  navData,
		Navigators:     navigators,
	}
}

type backtestClosedTrade struct {
	timeSec       int64
	entryTime     int64
	side          string
	entryPrice    float64
	exitPrice     float64
	stopLossPrice float64
	exitReason    string
	pnlPct        float64
	dollarPnL     float64
	duration      string
}

type backtestMetrics struct {
	totalTrades    int
	winRate        float64
	netProfit      float64
	profitFactor   float64
	maxDrawdown    float64
	recoveryFactor float64
}

func (e *BacktestEngine) openPosition(
	klines []exchange.Kline,
	barIndex int,
	side string,
	entryPrice float64,
	entryTime int64,
	atr float64,
	balance float64,
	lotMod float64,
) (btPosition, bool) {
	isLong := side == "BUY"
	stopPrice := computePositionStop(klines, barIndex, isLong, atr, GetRiskSettings())
	if stopPrice <= 0 {
		return btPosition{}, false
	}

	risk := GetRiskSettings()
	maxLev := float64(risk.Leverage)
	if maxLev <= 0 {
		maxLev = 1
	}
	qty, _ := execution.CalculateTargetQuantity(
		balance,
		risk.RiskPerTrade,
		entryPrice,
		stopPrice,
		lotMod,
		maxLev,
	)
	if qty <= 0 {
		return btPosition{}, false
	}

	return btPosition{
		side:       side,
		entryPrice: entryPrice,
		entryTime:  entryTime,
		stopPrice:  stopPrice,
		qty:        qty,
	}, true
}

func checkFractalStop(pos *btPosition, candle exchange.Candle) (float64, bool) {
	if pos == nil || pos.stopPrice <= 0 {
		return 0, false
	}
	if pos.side == "BUY" && candle.Low <= pos.stopPrice {
		return pos.stopPrice, true
	}
	if pos.side == "SELL" && candle.High >= pos.stopPrice {
		return pos.stopPrice, true
	}
	return 0, false
}

func (e *BacktestEngine) closeBacktestPosition(
	pos *btPosition,
	exitPrice float64,
	barTimeSec int64,
	exitReason string,
	balance float64,
	closedTrades *[]backtestClosedTrade,
	equity *[]BacktestEquityPoint,
	peak, maxDrawdownPct, maxDrawdownUSD float64,
) (float64, float64, float64, float64) {
	if pos == nil || pos.qty <= 0 {
		return balance, peak, maxDrawdownPct, maxDrawdownUSD
	}

	balanceBefore := balance
	netPnL := calcBacktestNetPnL(pos.side, pos.entryPrice, exitPrice, pos.qty, e.cfg.FeeRate)
	balance += netPnL

	pnlPct := 0.0
	if balanceBefore > 0 {
		pnlPct = netPnL / balanceBefore * 100
	}

	*closedTrades = append(*closedTrades, backtestClosedTrade{
		timeSec:       barTimeSec,
		entryTime:     pos.entryTime,
		side:          pos.displaySide(),
		entryPrice:    pos.entryPrice,
		exitPrice:     exitPrice,
		stopLossPrice: pos.stopPrice,
		exitReason:    exitReason,
		pnlPct:        pnlPct,
		dollarPnL:     netPnL,
		duration:      formatBacktestDuration(pos.entryTime, barTimeSec),
	})
	recordEquityPoint(equity, barTimeSec, balance)
	peak, maxDrawdownPct, maxDrawdownUSD = updateDrawdown(balance, peak, maxDrawdownPct, maxDrawdownUSD)
	return balance, peak, maxDrawdownPct, maxDrawdownUSD
}

func calcBacktestNetPnL(side string, entryPrice, exitPrice, qty, feeRate float64) float64 {
	if qty <= 0 || entryPrice <= 0 || exitPrice <= 0 {
		return 0
	}
	var rawPnL float64
	if side == "BUY" {
		rawPnL = (exitPrice - entryPrice) * qty
	} else {
		rawPnL = (entryPrice - exitPrice) * qty
	}
	fee := (entryPrice*qty*feeRate) + (exitPrice*qty*feeRate)
	return rawPnL - fee
}

func applyEntrySlippage(side string, openPrice, slippagePct float64) float64 {
	if openPrice <= 0 || slippagePct <= 0 {
		return openPrice
	}
	slip := slippagePct / 100.0
	if side == "BUY" {
		return openPrice * (1 + slip)
	}
	return openPrice * (1 - slip)
}

func applyExitSlippage(side string, marketPrice, slippagePct float64) float64 {
	if marketPrice <= 0 || slippagePct <= 0 {
		return marketPrice
	}
	slip := slippagePct / 100.0
	if side == "BUY" {
		return marketPrice * (1 - slip)
	}
	return marketPrice * (1 + slip)
}

func calcPnLPct(side string, entryPrice, exitPrice float64) float64 {
	if entryPrice <= 0 {
		return 0
	}
	if side == "SELL" {
		return (entryPrice - exitPrice) / entryPrice * 100
	}
	return (exitPrice - entryPrice) / entryPrice * 100
}

func formatBacktestDuration(entrySec, exitSec int64) string {
	if exitSec <= entrySec {
		return "0m"
	}
	seconds := exitSec - entrySec
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	return fmt.Sprintf("%dh %02dm", hours, minutes)
}

func recordEquityPoint(curve *[]BacktestEquityPoint, timeSec int64, balance float64) {
	if len(*curve) > 0 {
		last := (*curve)[len(*curve)-1]
		if last.Time == timeSec && last.Value == balance {
			return
		}
	}
	*curve = append(*curve, BacktestEquityPoint{Time: timeSec, Value: balance})
}

func updateDrawdown(balance, peak, maxPct, maxUSD float64) (float64, float64, float64) {
	if balance > peak {
		peak = balance
	}
	ddUSD := peak - balance
	ddPct := 0.0
	if peak > 0 {
		ddPct = ddUSD / peak * 100
	}
	if ddPct > maxPct {
		maxPct = ddPct
	}
	if ddUSD > maxUSD {
		maxUSD = ddUSD
	}
	return peak, maxPct, maxUSD
}

func computeBacktestMetrics(initial, final float64, trades []backtestClosedTrade, maxDrawdownPct, maxDrawdownUSD float64) backtestMetrics {
	total := len(trades)
	wins := 0
	var grossProfit, grossLoss float64

	for _, t := range trades {
		if t.dollarPnL > 0 {
			wins++
			grossProfit += t.dollarPnL
		} else if t.dollarPnL < 0 {
			grossLoss += -t.dollarPnL
		}
	}

	netProfit := 0.0
	if initial > 0 {
		netProfit = (final - initial) / initial * 100
	}

	winRate := 0.0
	if total > 0 {
		winRate = float64(wins) / float64(total) * 100
	}

	profitFactor := 0.0
	switch {
	case grossLoss > 0:
		profitFactor = grossProfit / grossLoss
	case grossProfit > 0:
		profitFactor = netProfit
	}

	recoveryFactor := 0.0
	if maxDrawdownUSD > 0 {
		recoveryFactor = (final - initial) / maxDrawdownUSD
	}

	return backtestMetrics{
		totalTrades:    total,
		winRate:        winRate,
		netProfit:      netProfit,
		profitFactor:   profitFactor,
		maxDrawdown:    maxDrawdownPct,
		recoveryFactor: recoveryFactor,
	}
}

func applyBacktestRSXMarkers(chartData []BacktestChartPoint, klines []exchange.Kline, rsxValues []float64) {
	if len(chartData) == 0 || len(klines) == 0 || len(rsxValues) != len(klines) {
		return
	}
	rsxPoints := BuildRSXChart(klines, rsxValues, GetRSXSettings().DivLookback)
	if len(rsxPoints) != len(klines) {
		return
	}
	markerByTime := make(map[int64]string)
	for i, k := range klines {
		if rsxPoints[i].Marker == "" {
			continue
		}
		markerByTime[k.OpenTime/1000] = rsxPoints[i].Marker
	}
	for i := range chartData {
		if m, ok := markerByTime[chartData[i].Time]; ok {
			chartData[i].Marker = m
		}
	}
}

func populateBacktestPointFromMarker(pt *BacktestChartPoint, marker *Marker, prevBlue float64, prevReady bool) {
	falcon := marker.FalconSnapshot()
	pt.RSX = falcon.JurikRSX
	pt.Jurik = falcon.JurikRSX
	pt.RSXSignal = falcon.JurikRSXSignal
	pt.RsiPrice = falcon.RsiPrice
	pt.EmaRsi = falcon.EmaRsi
	pt.RsiRsi = falcon.RsiRsi
	pt.RsiHl2 = falcon.RsiHl2
	pt.RsiVolFast = falcon.RsiVolFast
	pt.RsiVolSlow = falcon.RsiVolSlow
	pt.MacdRsi = falcon.MacdRsi
	pt.RsiAd = falcon.RsiAd
	pt.RsiHl2Vol = falcon.RsiHl2Vol
	pt.VolCrossMarker = falcon.VolCrossMarker
	pt.VolChanMid = falcon.VolChanMid
	pt.VolChanUp = falcon.VolChanUp
	pt.VolChanDn = falcon.VolChanDn
	pt.PriceChanMid = falcon.PriceChanMid
	pt.PriceChanUp = falcon.PriceChanUp
	pt.PriceChanDn = falcon.PriceChanDn
	pt.WozduhUp = falcon.RsiVolFast
	pt.WozduhDown = falcon.RsiVolSlow
	if prevReady {
		pt.VolumeSpikeUp = DetectWozduxVolumeSpikeUp(prevBlue, falcon.BlueLine, falcon.RedLine)
		pt.VolumeSpikeDown = DetectWozduxVolumeSpikeDown(prevBlue, falcon.BlueLine, falcon.RedLine)
	}
}

// ParseBacktestDateRange converts YYYY-MM-DD strings to inclusive millisecond bounds (UTC).
func ParseBacktestDateRange(startDate, endDate string) (int64, int64, error) {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return 0, 0, fmt.Errorf("parse start date: %w", err)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return 0, 0, fmt.Errorf("parse end date: %w", err)
	}
	if end.Before(start) {
		return 0, 0, fmt.Errorf("end date before start date")
	}
	end = end.Add(24*time.Hour - time.Millisecond)
	return start.UTC().UnixMilli(), end.UTC().UnixMilli(), nil
}
