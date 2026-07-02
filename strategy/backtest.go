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
	backtestEquityEvery        = 100
)

// BacktestMinBars returns the minimum candle count required before trading in backtests.
func BacktestMinBars() int {
	return IndicatorWarmupBars
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
	padDays := BacktestPadStartDays(binanceInterval, candleCount, IndicatorWarmupBars)
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
	Symbol          string
	Interval        string
	EntryAnalyst    *Analyst
	FeeRate         float64
	SlippagePct     float64
	Matrix          *ScoringMatrix
	Navigator       NavigatorUISettings
	Navigators      map[string]NavigatorUISettings
	HTF             *exchange.HTFProvider
	LongThreshold   int
	ShortThreshold  int
	RSXSettings     *RSXSettings
	WozduhPrefs     map[string]bool
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
	RedLine    float64
	GreenLine  float64
	BlueLine   float64

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
	Time             int64
	EntryTime        int64
	Side             string
	EntryPrice       float64
	ExitPrice        float64
	StopLossPrice    float64
	ExitReason       string
	EntryReason      string   `json:"entryReason,omitempty"`
	FactorsSnapshot  []string `json:"factorsSnapshot,omitempty"`
	StrategySource   string   `json:"strategySource,omitempty"`
	ActiveFactors    []string `json:"activeFactors,omitempty"`
	SignalKind       string   `json:"signalKind,omitempty"`
	EntryScore       float64  `json:"entryScore,omitempty"`
	PnL              float64
	Duration         string
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
	Annotations    []ChartAnnotation
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
	if cfg.LongThreshold <= 0 {
		cfg.LongThreshold = DefaultScoreThreshold
	}
	if cfg.ShortThreshold <= 0 {
		cfg.ShortThreshold = DefaultScoreThreshold
	}
	return &BacktestEngine{cfg: cfg}
}

type btPosition struct {
	side       string
	entryPrice float64
	entryTime  int64
	stopPrice  float64
	qty        float64
	entryAudit btTradeEntryAudit
}

func logBacktestSignal(barIndex int, decision ScoreDecision, signalKind string) {
	entryReason, factorsSnapshot, score := buildTradeEntryAudit(decision, string(decision.FinalAction))
	log.Printf("[Backtest SIGNAL] Bar %d: Action=%s, Kind=%s, Score=%.0f, Reason=%s, Factors=%v | Long=%d Short=%d Raw=%+v",
		barIndex, decision.FinalAction, signalKind, score, entryReason, factorsSnapshot,
		decision.LongScore, decision.ShortScore, decision.Factors)
	if !snapshotHasRSXFactor(factorsSnapshot) {
		log.Printf("[Info] Signal bar %d: entry NOT driven by RSX (factors: %v)", barIndex, factorsSnapshot)
	}
}

func (p *btPosition) displaySide() string {
	if p.side == "SELL" {
		return "SHORT"
	}
	return "LONG"
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
	matrix := e.activeMatrix()
	log.Printf("[Backtest] Engine thresholds: long=%d short=%d | matrix RSX=%v WozduhCross=%v Geometry=%v entrySources=%v",
		e.cfg.LongThreshold, e.cfg.ShortThreshold,
		matrix.UseRSX, matrix.UseWozduhCross, matrix.UseGeometry,
		ScoringMatrixEntrySourcesEnabledFor(matrix))

	replayCfg := StreamingReplayConfigFromBacktest(e.cfg)
	replay := RunStreamingReplay(ctx, candlesToKlines(candles), replayCfg)

	if replay.Cancelled {
		log.Printf("[Backtest] Cancelled (%d chart points)", len(replay.ChartPoints))
	}
	log.Printf("[Backtest] streaming replay: %d annotations (%d candles, %d chart points)",
		len(replay.Annotations), len(candles), len(replay.ChartPoints))
	if len(replay.Annotations) > 0 {
		first := replay.Annotations[0]
		log.Printf("[Backtest] annotation sample: time=%d (sec) pane=%s label=%s", first.Time, first.Pane, first.Label)
	}

	chartPts := replay.ChartPoints
	if len(chartPts) >= IndicatorWarmupBars {
		chartPts = chartPts[IndicatorWarmupBars-1:]
	}

	return e.assembleRunResult(
		replay.FinalBalance, replay.ClosedTrades, replay.EquityCurve, chartPts,
		replay.HistKlines, replay.HistRSX, replay.HistWozduh,
		replay.MaxDrawdownPct, replay.MaxDrawdownUSD, replay.Cancelled,
		replay.Annotations,
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
	annotations []ChartAnnotation,
) *BacktestRunResult {
	metrics := computeBacktestMetrics(BacktestInitialCapital, balance, closedTrades, maxDrawdownPct, maxDrawdownUSD)

	trades := make([]BacktestTradeResult, len(closedTrades))
	for i, t := range closedTrades {
		trades[i] = BacktestTradeResult{
			Time:            t.timeSec,
			EntryTime:       t.entryTime,
			Side:            t.side,
			EntryPrice:      t.entryPrice,
			ExitPrice:       t.exitPrice,
			StopLossPrice:   t.stopLossPrice,
			ExitReason:      t.exitReason,
			EntryReason:     t.entryReason,
			FactorsSnapshot: append([]string(nil), t.factorsSnapshot...),
			StrategySource:  t.strategySource,
			ActiveFactors:   append([]string(nil), t.activeFactors...),
			SignalKind:      t.signalKind,
			EntryScore:      t.entryScore,
			PnL:             t.pnlPct,
			Duration:        t.duration,
		}
	}

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
		Annotations:    annotations,
	}
}

type backtestClosedTrade struct {
	timeSec         int64
	entryTime       int64
	side            string
	entryPrice      float64
	exitPrice       float64
	stopLossPrice   float64
	exitReason      string
	entryReason     string
	factorsSnapshot []string
	strategySource  string
	activeFactors   []string
	signalKind      string
	entryScore      float64
	pnlPct          float64
	dollarPnL       float64
	duration        string
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
	audit btTradeEntryAudit,
) (btPosition, bool) {
	isLong := side == "BUY"
	stopPrice := computePositionStop(klines, barIndex, isLong, atr, GetRiskSettings())
	if stopPrice <= 0 {
		log.Printf("[VETO] Trade blocked: Stop is zero (side=%s bar=%d)", side, barIndex)
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
		log.Printf("[VETO] Trade blocked: Qty is zero (side=%s bar=%d)", side, barIndex)
		return btPosition{}, false
	}

	logTradeEntryExecution(entryTime, side, audit)

	return btPosition{
		side:       side,
		entryPrice: entryPrice,
		entryTime:  entryTime,
		stopPrice:  stopPrice,
		qty:        qty,
		entryAudit: audit,
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
		timeSec:         barTimeSec,
		entryTime:       pos.entryTime,
		side:            pos.displaySide(),
		entryPrice:      pos.entryPrice,
		exitPrice:       exitPrice,
		stopLossPrice:   pos.stopPrice,
		exitReason:      exitReason,
		entryReason:     pos.entryAudit.entryReason,
		factorsSnapshot: append([]string(nil), pos.entryAudit.factorsSnapshot...),
		strategySource:  pos.entryAudit.strategySource,
		activeFactors:   append([]string(nil), pos.entryAudit.activeFactors...),
		signalKind:      pos.entryAudit.signalKind,
		entryScore:      pos.entryAudit.score,
		pnlPct:          pnlPct,
		dollarPnL:       netPnL,
		duration:        formatBacktestDuration(pos.entryTime, barTimeSec),
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

func populateBacktestPointFromFalcon(pt *BacktestChartPoint, sig FalconSignals, prevBlue float64, prevReady bool) {
	pt.RsiPrice = sig.RsiPrice
	pt.EmaRsi = sig.EmaRsi
	pt.RsiRsi = sig.RsiRsi
	pt.RsiHl2 = sig.RsiHl2
	pt.RsiVolFast = sig.RsiVolFast
	pt.RsiVolSlow = sig.RsiVolSlow
	pt.MacdRsi = sig.MacdRsi
	pt.RsiAd = sig.RsiAd
	pt.RsiHl2Vol = sig.RsiHl2Vol
	pt.VolCrossMarker = sig.VolCrossMarker
	pt.VolChanMid = sig.VolChanMid
	pt.VolChanUp = sig.VolChanUp
	pt.VolChanDn = sig.VolChanDn
	pt.PriceChanMid = sig.PriceChanMid
	pt.PriceChanUp = sig.PriceChanUp
	pt.PriceChanDn = sig.PriceChanDn
	pt.WozduhUp = sig.RsiVolFast
	pt.WozduhDown = sig.RsiVolSlow
	pt.RedLine = sig.RedLine
	pt.GreenLine = sig.GreenLine
	pt.BlueLine = sig.BlueLine
	if prevReady {
		pt.VolumeSpikeUp = DetectWozduxVolumeSpikeUp(prevBlue, sig.BlueLine, sig.RedLine)
		pt.VolumeSpikeDown = DetectWozduxVolumeSpikeDown(prevBlue, sig.BlueLine, sig.RedLine)
	}
}

func populateBacktestPointFromMarker(pt *BacktestChartPoint, marker *Marker, prevBlue float64, prevReady bool) {
	falcon := marker.FalconSnapshot()
	pt.RSX = falcon.JurikRSX
	pt.Jurik = falcon.JurikRSX
	pt.RSXSignal = falcon.JurikRSXSignal
	populateBacktestPointFromFalcon(pt, falcon, prevBlue, prevReady)
}

// candlesToKlines converts backtest candles to klines for the unified RSX pipeline.
func candlesToKlines(candles []exchange.Candle) []exchange.Kline {
	out := make([]exchange.Kline, len(candles))
	for i, c := range candles {
		out[i] = exchange.Kline{
			OpenTime:  c.OpenTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
			CloseTime: c.CloseTime,
		}
	}
	return out
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
