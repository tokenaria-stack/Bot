package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"trading_bot/exchange"
	"trading_bot/execution"
	"trading_bot/data"
	"trading_bot/domain"
	"trading_bot/vector_db"
)

const (
	positionSyncInterval = 5 * time.Second
	defaultFeeRate       = 0.0012 // Binance USDⓈ-M Futures taker approx.
	DefaultTradingTimeframe = "1m"
	tradingHistoryPrefetchBars = IndicatorWarmupBars
)

type MasterState string

const (
	StateIdle       MasterState = "IDLE"
	StateInPosition MasterState = "IN_POSITION"
)

type MasterGeneral struct {
	mu               sync.RWMutex
	state            MasterState
	targetSide       string
	analysts         map[string]*Marker
	signalAnalyst    *Analyst
	chief            *ChiefAnalyst
	exchangeClient   *exchange.BinanceExchange
	htfProvider      *exchange.HTFProvider
	readOnly         bool
	sandboxMode      bool
	symbol           string
	timeframe        string
	balance          float64
	positionQty      float64
	entryPrice       float64
	entryTimeSec     int64
	currentStopPrice float64
	lastPositionSync time.Time
	lastOrderTime    time.Time
	feeRate          float64
	memoryStore      *vector_db.MemoryStore
	onTick           func(kline exchange.Kline, jurik, redLine, greenLine, blueLine float64)
	onTelemetry      func(tick exchange.WsTick, falcon FalconSignals, decision ScoreDecision)
	onKlineBar       func(timeframe string, kline exchange.Kline, isClosed bool)
	onTrade          func(event TradeEvent)
	onClosedTrade    func(trade domain.ClosedTrade, isVirtual bool)

	TickLiveCh chan struct{}

	persistQ *data.PersistenceQueue

	mtfTracker      *WalkForwardMTFTracker
	navigatorPanes  map[string]NavigatorUISettings
	navMu           sync.RWMutex
	closedTelemetry map[string]closedBarTelemetry
}

type closedBarTelemetry struct {
	decision ScoreDecision
	regime   string
}

func NewMasterGeneral(
	analysts map[string]*Marker,
	signalAnalyst *Analyst,
	exchangeClient *exchange.BinanceExchange,
	htfProvider *exchange.HTFProvider,
	memoryStore *vector_db.MemoryStore,
	readOnly bool,
	sandboxMode bool,
	symbol string,
	timeframe string,
) *MasterGeneral {
	if timeframe == "" {
		timeframe = DefaultTradingTimeframe
	}
	if signalAnalyst == nil {
		signalAnalyst = NewAnalyst(sandboxMode)
	}
	return &MasterGeneral{
		state:          StateIdle,
		analysts:       analysts,
		signalAnalyst:  signalAnalyst,
		chief:          NewChiefAnalyst(),
		exchangeClient: exchangeClient,
		htfProvider:    htfProvider,
		memoryStore:    memoryStore,
		readOnly:       readOnly,
		sandboxMode:    sandboxMode,
		symbol:         symbol,
		timeframe:      timeframe,
		feeRate:        defaultFeeRate,
		TickLiveCh:      make(chan struct{}, 1000),
		closedTelemetry: make(map[string]closedBarTelemetry),
	}
}

// TradingTimeframe returns the active strategy/decision timeframe.
func (m *MasterGeneral) TradingTimeframe() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.timeframe != "" {
		return m.timeframe
	}
	return DefaultTradingTimeframe
}

func (m *MasterGeneral) HTFProvider() *exchange.HTFProvider {
	return m.htfProvider
}

// SetNavigatorPanes updates walk-forward MTF navigator settings (from dashboard UI).
func (m *MasterGeneral) SetNavigatorPanes(panes map[string]NavigatorUISettings) {
	if m == nil {
		return
	}
	m.navMu.Lock()
	if len(panes) == 0 {
		m.navigatorPanes = nil
	} else {
		m.navigatorPanes = make(map[string]NavigatorUISettings, len(panes))
		for k, v := range panes {
			m.navigatorPanes[k] = v
		}
	}
	m.navMu.Unlock()
	m.rebuildMTFTrackerIfReady()
}

func (m *MasterGeneral) rebuildMTFTrackerIfReady() {
	if m == nil || m.htfProvider == nil {
		m.mtfTracker = nil
		return
	}
	analyst := m.workAnalyst()
	if analyst == nil || !analyst.HasMinBars(minScoreBars) {
		m.mtfTracker = nil
		log.Printf("[Master] MTF tracker deferred: warming up (%d bars, need %d)",
			barCount(analyst), minScoreBars)
		return
	}
	m.rebuildMTFTracker()
}

func (m *MasterGeneral) rebuildMTFTracker() {
	if m == nil || m.htfProvider == nil {
		m.mtfTracker = nil
		return
	}
	m.navMu.RLock()
	panes := m.navigatorPanes
	m.mu.RLock()
	symbol := m.symbol
	tf := m.timeframe
	m.mu.RUnlock()
	m.navMu.RUnlock()

	tfs := CollectWalkForwardMTFPeriods(panes, tf)
	if len(tfs) == 0 {
		m.mtfTracker = nil
		return
	}
	priceUI, ok := panes["price"]
	if !ok {
		priceUI = NavigatorUISettings{Enabled: true, Source: navigatorSourcePrice}
	}
	tracker := NewWalkForwardMTFTracker(m.htfProvider, symbol, tf, priceUI, tfs)
	if analyst := m.workAnalyst(); analyst != nil {
		klines := analyst.GetKlines()
		if len(klines) > 0 {
			tracker.SetChartStartMs(klines[0].OpenTime)
		}
	}
	tracker.Prefetch()
	m.mtfTracker = tracker
	log.Printf("[Master] Walk-forward MTF tracker ready: %v", tfs)
}

func barCount(analyst *Marker) int {
	if analyst == nil {
		return 0
	}
	return len(analyst.GetKlines())
}

// NotifyKlineGapFillComplete is called after background SQLite gap-fill for a symbol/interval.
func (m *MasterGeneral) NotifyKlineGapFillComplete(symbol, interval string) {
	if m == nil {
		return
	}
	m.mu.RLock()
	sym := m.symbol
	tf := m.timeframe
	m.mu.RUnlock()
	if exchange.NormalizeFuturesSymbol(symbol) != exchange.NormalizeFuturesSymbol(sym) {
		return
	}
	if interval == tf {
		m.hydrateTradingHistoryFromStore(sym, interval)
	}
	m.rebuildMTFTrackerIfReady()
}

func (m *MasterGeneral) hydrateTradingHistoryFromStore(symbol, interval string) {
	m.mu.RLock()
	analyst := m.analysts[interval]
	workTF := m.timeframe
	m.mu.RUnlock()
	if analyst == nil {
		analyst = m.workAnalyst()
	}
	if analyst == nil {
		return
	}
	m.reconcileKlineGap(symbol, interval, analyst)
	if interval == workTF {
		log.Printf("[Master] Hydrated trading history: %s %s (%d bars)", symbol, interval, len(analyst.GetKlines()))
	}
}

func (m *MasterGeneral) ensureMTFTrackerReady(analyst *Marker) {
	if m == nil || m.mtfTracker != nil {
		return
	}
	if analyst == nil || !analyst.HasMinBars(minScoreBars) {
		return
	}
	m.rebuildMTFTracker()
}

func (m *MasterGeneral) syncMTFState(analyst *Marker) {
	if m == nil || analyst == nil || m.mtfTracker == nil {
		return
	}
	klines := analyst.GetKlines()
	var tickSec int64
	if len(klines) > 0 {
		last := klines[len(klines)-1]
		if last.CloseTime > 0 {
			tickSec = last.CloseTime / 1000
		} else {
			tickSec = last.OpenTime / 1000
		}
	}
	if tickSec <= 0 {
		tickSec = time.Now().Unix()
	}
	m.mtfTracker.Update(tickSec, klines)
	analyst.SetCurrentMTFState(m.mtfTracker.States())
}

// SetOnTick registers a callback invoked after each closed candle on the trading timeframe.
// Deprecated: prefer SetOnTelemetry for live dashboard scoring.
func (m *MasterGeneral) SetOnTick(fn func(kline exchange.Kline, jurik, redLine, greenLine, blueLine float64)) {
	m.mu.Lock()
	m.onTick = fn
	m.mu.Unlock()
}

// SetOnTelemetry registers a callback for every trading-TF WS tick (intra-bar + close)
// with a fresh ScoreDecision (MTF synced on bar close before scoring).
func (m *MasterGeneral) SetOnTelemetry(fn func(tick exchange.WsTick, falcon FalconSignals, decision ScoreDecision)) {
	m.mu.Lock()
	m.onTelemetry = fn
	m.mu.Unlock()
}

// SetPersistenceQueue binds the sole SQLite archive writer (Shot 9E).
func (m *MasterGeneral) SetPersistenceQueue(q *data.PersistenceQueue) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.persistQ = q
	m.mu.Unlock()
}

// ScoreDecisionForTelemetry returns the last closed-bar ScoreDecision for dashboard/API consumers.
// Read-only: does not mutate MTF or marker streaming state.
func (m *MasterGeneral) ScoreDecisionForTelemetry(marker *Marker) ScoreDecision {
	if m == nil || marker == nil {
		return ScoreDecision{Factors: make(map[string]ScoreFactor)}
	}
	tf := marker.Timeframe()
	if tf == "" {
		tf = m.TradingTimeframe()
	}
	if t, ok := m.closedBarTelemetryFor(tf); ok {
		return t.decision
	}
	return ScoreDecision{Factors: make(map[string]ScoreFactor)}
}

// ClosedVolatilityRegimeForTelemetry returns the regime fixed at the last closed bar.
func (m *MasterGeneral) ClosedVolatilityRegimeForTelemetry(marker *Marker) string {
	if m == nil || marker == nil {
		return ""
	}
	tf := marker.Timeframe()
	if tf == "" {
		tf = m.TradingTimeframe()
	}
	if t, ok := m.closedBarTelemetryFor(tf); ok && t.regime != "" {
		return t.regime
	}
	return marker.ClosedVolatilityRegime()
}

// SeedClosedBarTelemetry snapshots closed-bar scoring/regime from current analyst state.
// Call once after history hydration (not on GET /api/state).
func (m *MasterGeneral) SeedClosedBarTelemetry() {
	if m == nil {
		return
	}
	m.mu.RLock()
	workTF := m.timeframe
	analyst := m.analysts[workTF]
	m.mu.RUnlock()
	if analyst == nil {
		return
	}
	m.ensureMTFTrackerReady(analyst)
	m.syncMTFState(analyst)
	m.refreshClosedBarTelemetry(workTF, analyst)
}

func (m *MasterGeneral) refreshClosedBarTelemetry(tf string, analyst *Marker) {
	if m == nil || analyst == nil || tf == "" {
		return
	}
	// TODO: Debt - Re-enable and configure ScoreMatrix/Falcon/Divergence in later phases.
	return
	/*
		decision := m.evaluateScoreDecision(analyst)
		regime := analyst.ClosedVolatilityRegime()
		m.mu.Lock()
		if m.closedTelemetry == nil {
			m.closedTelemetry = make(map[string]closedBarTelemetry)
		}
		m.closedTelemetry[tf] = closedBarTelemetry{decision: decision, regime: regime}
		m.mu.Unlock()
	*/
}

func (m *MasterGeneral) closedBarTelemetryFor(tf string) (closedBarTelemetry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.closedTelemetry[tf]
	return t, ok
}

// SetOnKlineBar registers a callback for live kline updates on any subscribed timeframe.
// isClosed is Binance k.x (bar finalized). Shot 9B: used for atomic chart delivery on every TF.
func (m *MasterGeneral) SetOnKlineBar(fn func(timeframe string, kline exchange.Kline, isClosed bool)) {
	m.mu.Lock()
	m.onKlineBar = fn
	m.mu.Unlock()
}

// SetOnTrade registers a callback invoked when a trade entry or exit is executed.
func (m *MasterGeneral) SetOnTrade(fn func(event TradeEvent)) {
	m.mu.Lock()
	m.onTrade = fn
	m.mu.Unlock()
}

// SetOnClosedTrade registers a callback when a round-trip position is closed.
func (m *MasterGeneral) SetOnClosedTrade(fn func(trade domain.ClosedTrade, isVirtual bool)) {
	m.mu.Lock()
	m.onClosedTrade = fn
	m.mu.Unlock()
}

// RecoverState is called once at bot startup.
// It checks for open positions on the exchange and syncs internal state.
func (m *MasterGeneral) RecoverState() {
	if m.sandboxMode {
		m.resetToCleanState()
		log.Println("[Master] Sandbox mode: starting in IDLE (virtual balance, paper execution)")
		if m.readOnly {
			log.Println("[Master] Sandbox + read-only: virtual trades only")
		}
		return
	}

	if m.readOnly {
		m.resetToCleanState()
		log.Println("[Master] Read-only mode: skipping private API recovery, starting clean in IDLE state.")
		return
	}

	if m.exchangeClient == nil {
		log.Println("[Master] ⚠️ Could not recover state: exchange client is not configured")
		return
	}

	amt, err := m.exchangeClient.GetPositionAmt(m.symbol)
	if err != nil {
		log.Printf("[Master] ⚠️ Could not recover state: %v", err)
		return
	}

	if amt != 0 {
		m.mu.Lock()
		m.positionQty = math.Abs(amt)
		var side string
		if amt > 0 {
			m.targetSide = "BUY"
			m.currentStopPrice = 0
			side = "BUY"
		} else {
			m.targetSide = "SELL"
			m.currentStopPrice = math.MaxFloat64
			side = "SELL"
		}
		m.state = StateInPosition
		qty := m.positionQty
		m.mu.Unlock()

		log.Printf("[Master] 🔄 RECOVERY SUCCESS: Found active %s position (Qty: %.4f). Resuming IN_POSITION state.", side, qty)

		if err := m.exchangeClient.CancelAllOpenOrders(m.symbol); err != nil {
			log.Printf("[Master] ⚠️ Failed to cancel stale orders during recovery: %v", err)
		}
	} else {
		log.Println("[Master] 🔄 Recovery check: No active positions. Starting clean in IDLE state.")
	}
}

func (m *MasterGeneral) resetToCleanState() {
	m.mu.Lock()
	m.state = StateIdle
	m.targetSide = ""
	m.balance = BacktestInitialCapital
	m.positionQty = 0
	m.entryPrice = 0
	m.currentStopPrice = 0
	m.mu.Unlock()
}

func (m *MasterGeneral) paperBalance() float64 {
	m.mu.RLock()
	bal := m.balance
	m.mu.RUnlock()
	if bal > 0 {
		return bal
	}
	return BacktestInitialCapital
}

func (m *MasterGeneral) Run(ctx context.Context) {
	log.Println("[Master] Event Loop started.")
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.TickLiveCh:
			m.handleLiveTick()
		}
	}
}

func (m *MasterGeneral) handleLiveTick() {
	analyst := m.workAnalyst()
	if analyst == nil {
		return
	}
	if !analyst.HasMinBars(minScoreBars) {
		log.Printf("[Master] ⏳ Warmup: %d/%d bars on %s — skipping evaluation",
			len(analyst.GetKlines()), minScoreBars, m.timeframe)
		return
	}

	m.ensureMTFTrackerReady(analyst)
	m.syncMTFState(analyst)

	m.mu.RLock()
	state := m.state
	readOnly := m.readOnly
	sandbox := m.sandboxMode
	m.mu.RUnlock()

	switch state {
	case StateInPosition:
		if readOnly || sandbox {
			m.manageVirtualPosition()
		} else {
			m.manageReversal()
			m.managePosition()
		}
	default:
		m.tryForEntry()
	}
}

func (m *MasterGeneral) getState() MasterState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// State returns the current master state machine value for dashboard telemetry.
func (m *MasterGeneral) State() MasterState {
	return m.getState()
}

func (m *MasterGeneral) setState(newState MasterState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != newState {
		log.Printf("[Master] State transition: %s -> %s", m.state, newState)
		m.state = newState
	}
}

func (m *MasterGeneral) workAnalyst() *Marker {
	if a, ok := m.analysts[m.timeframe]; ok {
		return a
	}
	return m.analysts[DefaultTradingTimeframe]
}

// tryForEntry looks for a setup on the trading timeframe via Layer 3 scoring + entry risk gate.
func (m *MasterGeneral) tryForEntry() {
	m.mu.RLock()
	state := m.state
	m.mu.RUnlock()
	if state == StateInPosition {
		return
	}

	analyst := m.workAnalyst()
	if analyst == nil {
		return
	}
	if !analyst.HasMinBars(minScoreBars) {
		return
	}

	decision := m.evaluateScoreDecision(analyst)
	if !decision.HasFinalSignal() {
		return
	}

	entrySide := string(decision.FinalAction)

	direction := "LONG"
	if entrySide == "SELL" {
		direction = "SHORT"
	}
	closePrice := analyst.LastClose()
	barIndex := len(analyst.GetKlines()) - 1
	logTradeSource(barIndex, entrySide, decision)
	log.Printf("[Master] 🔥 ENTRY TRIGGER: %s | Score: %d | Entry: %.2f | LotMod: %.2f | %s",
		direction, decision.WinningScore(), closePrice, decision.LotMod, decision.Reason)

	klines := analyst.GetKlines()
	stopLossPrice := computePositionStop(klines, len(klines)-1, entrySide == "BUY", analyst.LastATR(), GetRiskSettings())
	if stopLossPrice <= 0 {
		log.Printf("[VETO] Trade blocked: Stop is zero (side=%s tf=%s)", direction, m.timeframe)
		return
	}

	if m.readOnly || m.sandboxMode {
		log.Printf("[Paper Trading] Virtual order executed. %s @ %.2f | SL %.2f | %s",
			entrySide, closePrice, stopLossPrice, decision.Reason)
		m.openVirtualPosition(entrySide, closePrice, stopLossPrice, decision, analyst)
		return
	}

	m.openLivePosition(entrySide, analyst, decision, stopLossPrice)
}

func (m *MasterGeneral) openLivePosition(
	entrySide string,
	analyst *Marker,
	decision ScoreDecision,
	stopLossPrice float64,
) {
	if m.exchangeClient == nil {
		log.Println("[Master] ❌ Exchange client is not configured")
		m.setState(StateIdle)
		return
	}

	availableBalance, err := m.exchangeClient.GetFuturesBalance("USDT")
	if err != nil {
		log.Printf("[Master] ❌ Failed to fetch USDT balance: %v", err)
		return
	}

	m.mu.Lock()
	m.balance = availableBalance
	m.targetSide = entrySide
	m.mu.Unlock()

	direction := "LONG"
	if entrySide == "SELL" {
		direction = "SHORT"
	}

	closePrice := analyst.LastClose()
	qty, leverage, err := m.calculateTargetQuantity(closePrice, stopLossPrice, decision.LotMod, availableBalance)
	if err != nil {
		log.Printf("[VETO] Trade blocked: Stop/Qty is zero (%v)", err)
		return
	}

	log.Printf("[Master] 🔥 EXEC: %s | Fractal SL: %.2f | Qty: %.8f | Lev: %.0fx",
		direction, stopLossPrice, qty, leverage)

	levInt := int(leverage)
	if levInt < 1 {
		levInt = 1
	}

	if err := m.exchangeClient.ChangeLeverage(m.symbol, levInt); err != nil {
		log.Printf("[Master] ❌ Failed to set leverage %dx: %v", levInt, err)
		m.setState(StateIdle)
		return
	}

	response, err := m.exchangeClient.CreateMarketOrder(m.symbol, entrySide, qty)
	if err != nil {
		log.Printf("[Master] ❌ Order failed: %v", err)
		m.setState(StateIdle)
		return
	}

	orderID, parseErr := parseOrderID(response)
	if parseErr != nil {
		log.Printf("[Master] ✅ Order sent | %s %s qty=%.8f | response: %s",
			entrySide, m.symbol, qty, response)
	} else {
		log.Printf("[Master] ✅ Order executed! OrderID=%d | %s %s qty=%.8f",
			orderID, entrySide, m.symbol, qty)
	}

	m.fireTradeEvent(TradeEvent{
		Side:    entrySide,
		Price:   closePrice,
		BarTime: barTimeFromAnalyst(analyst),
		Reason:  decision.Reason,
		Kind:    "entry",
	})

	m.mu.Lock()
	m.positionQty = qty
	m.entryPrice = closePrice
	m.entryTimeSec = barTimeFromAnalyst(analyst)
	m.mu.Unlock()

	stopSide := "SELL"
	if entrySide == "SELL" {
		stopSide = "BUY"
	}

	err = m.exchangeClient.CreateStopMarketOrder(m.symbol, stopSide, qty, stopLossPrice)
	if err != nil {
		log.Printf("[Master] ⚠️ Hard stop failed (position is open): %v", err)
	} else {
		log.Printf("[Master] 🛡️ Fractal stop placed! %s @ %.2f", stopSide, stopLossPrice)
		m.mu.Lock()
		m.currentStopPrice = stopLossPrice
		m.mu.Unlock()
	}

	m.setState(StateInPosition)
}

func (m *MasterGeneral) calculateTargetQuantity(
	entryPrice, stopLoss, lotMod, balance float64,
) (qty, leverage float64, err error) {
	risk := GetRiskSettings()
	maxLev := float64(risk.Leverage)
	if maxLev <= 0 {
		maxLev = 1
	}
	qty, leverage = execution.CalculateTargetQuantity(
		balance,
		risk.RiskPerTrade,
		entryPrice,
		stopLoss,
		lotMod,
		maxLev,
	)
	if qty <= 0 {
		return 0, 0, fmt.Errorf("position size is zero")
	}
	return qty, leverage, nil
}

func barTimeFromAnalyst(analyst *Marker) int64 {
	if analyst == nil {
		return time.Now().Unix()
	}
	klines := analyst.GetKlines()
	if len(klines) == 0 {
		return time.Now().Unix()
	}
	return klines[len(klines)-1].OpenTime / 1000
}

func (m *MasterGeneral) fireTradeEvent(event TradeEvent) {
	m.mu.RLock()
	cb := m.onTrade
	m.mu.RUnlock()
	if cb == nil {
		return
	}
	if event.BarTime == 0 {
		event.BarTime = time.Now().Unix()
	}
	m.mu.Lock()
	m.lastOrderTime = time.Now()
	m.mu.Unlock()
	cb(event)
}

func (m *MasterGeneral) openVirtualPosition(
	entrySide string,
	entryPrice, stopPrice float64,
	decision ScoreDecision,
	analyst *Marker,
) {
	balance := m.paperBalance()
	qty, leverage, err := m.calculateTargetQuantity(entryPrice, stopPrice, decision.LotMod, balance)
	if err != nil {
		log.Printf("[Paper Trading] ❌ Sizing failed: %v", err)
		return
	}

	m.fireTradeEvent(TradeEvent{
		Side:    entrySide,
		Price:   entryPrice,
		BarTime: barTimeFromAnalyst(analyst),
		Reason:  decision.Reason,
		Kind:    "entry",
	})

	m.mu.Lock()
	m.targetSide = entrySide
	m.entryPrice = entryPrice
	m.entryTimeSec = barTimeFromAnalyst(analyst)
	m.currentStopPrice = stopPrice
	m.positionQty = qty
	m.mu.Unlock()

	m.setState(StateInPosition)
	log.Printf("[Paper Trading] Virtual position opened: %s @ %.2f | Qty %.8f | Lev %.0fx | Fractal SL %.2f",
		entrySide, entryPrice, qty, leverage, stopPrice)
}

func (m *MasterGeneral) closeVirtualPosition(exitSide, trigger string, price float64, analyst *Marker) {
	m.mu.RLock()
	entrySide := m.targetSide
	entryPrice := m.entryPrice
	entryTime := m.entryTimeSec
	stopPrice := m.currentStopPrice
	qty := m.positionQty
	balanceBefore := m.balance
	m.mu.RUnlock()

	closeSide := "CLOSE_LONG"
	if entrySide == "SELL" {
		closeSide = "CLOSE_SHORT"
	}

	netPnL := m.calcNetPnL(entrySide, entryPrice, price, qty)
	balanceAfter := balanceBefore + netPnL
	if balanceAfter < 0 {
		balanceAfter = 0
	}

	exitTime := barTimeFromAnalyst(analyst)
	reason := FormatExitReason(trigger, entrySide)
	m.emitClosedTrade(entrySide, entryPrice, price, stopPrice, qty, entryTime, exitTime, reason, balanceBefore, true)

	log.Printf("[Paper Trading] Virtual position closed. %s @ %.2f | Net PnL %.2f | %s",
		closeSide, price, netPnL, reason)

	m.fireTradeEvent(TradeEvent{
		Side:    closeSide,
		Price:   price,
		BarTime: barTimeFromAnalyst(analyst),
		Reason:  reason,
		Kind:    "exit",
	})

	m.mu.Lock()
	m.targetSide = ""
	m.entryPrice = 0
	m.entryTimeSec = 0
	m.currentStopPrice = 0
	m.positionQty = 0
	m.balance = balanceAfter
	m.mu.Unlock()

	m.setState(StateIdle)
}

func (m *MasterGeneral) emitClosedTrade(
	entrySide string,
	entryPrice, exitPrice, stopPrice, qty float64,
	entryTimeSec, exitTimeSec int64,
	exitReason string,
	balanceBefore float64,
	isVirtual bool,
) {
	if qty <= 0 || entryPrice <= 0 || exitPrice <= 0 || entrySide == "" {
		return
	}

	netPnL := m.calcNetPnL(entrySide, entryPrice, exitPrice, qty)
	fee := (entryPrice*qty*m.feeRate) + (exitPrice*qty*m.feeRate)
	pnlPct := 0.0
	if balanceBefore > 0 {
		pnlPct = netPnL / balanceBefore * 100
	}
	if exitTimeSec <= 0 {
		exitTimeSec = time.Now().Unix()
	}
	if entryTimeSec <= 0 {
		entryTimeSec = exitTimeSec
	}

	trade := domain.ClosedTrade{
		EntryTime:     entryTimeSec,
		ExitTime:      exitTimeSec,
		Side:          domain.DisplaySideFromEntry(entrySide),
		EntryPrice:    entryPrice,
		ExitPrice:     exitPrice,
		StopLossPrice: stopPrice,
		Fee:           fee,
		PnL:           pnlPct,
		PnLDollar:     netPnL,
		ExitReason:    exitReason,
		Duration:      domain.FormatTradeDuration(entryTimeSec, exitTimeSec),
	}

	m.mu.RLock()
	cb := m.onClosedTrade
	m.mu.RUnlock()
	if cb != nil {
		cb(trade, isVirtual)
	}
}

func (m *MasterGeneral) calcNetPnL(side string, entryPrice, exitPrice, qty float64) float64 {
	if qty <= 0 || entryPrice <= 0 || exitPrice <= 0 {
		return 0
	}
	var rawPnL float64
	if side == "BUY" {
		rawPnL = (exitPrice - entryPrice) * qty
	} else {
		rawPnL = (entryPrice - exitPrice) * qty
	}
	fee := (entryPrice*qty*m.feeRate) + (exitPrice*qty*m.feeRate)
	return rawPnL - fee
}

func (m *MasterGeneral) reverseVirtualPosition(newSide string, closePrice float64, decision ScoreDecision, analyst *Marker) {
	m.mu.RLock()
	entrySide := m.targetSide
	m.mu.RUnlock()

	exitSide := "SELL"
	if entrySide == "SELL" {
		exitSide = "BUY"
	}

	m.closeVirtualPosition(exitSide, "Reverse Signal", closePrice, analyst)

	if err := m.signalAnalyst.AnalyzeSignals(analyst, newSide); err != nil {
		log.Printf("[Master] ⛔ Reverse blocked by Analyst: %v (%s)", err, RiskErrorLabel(err))
		return
	}

	klines := analyst.GetKlines()
	stopLossPrice := computePositionStop(klines, len(klines)-1, newSide == "BUY", analyst.LastATR(), GetRiskSettings())
	if stopLossPrice <= 0 {
		log.Println("[Master] ❌ Fractal stop level unavailable for reverse")
		return
	}

	m.openVirtualPosition(newSide, closePrice, stopLossPrice, decision, analyst)
}

// manageVirtualPosition handles paper exits: fractal SL and reversal on the trading timeframe.
func (m *MasterGeneral) manageVirtualPosition() {
	analyst := m.workAnalyst()
	if analyst == nil {
		return
	}

	m.mu.RLock()
	entrySide := m.targetSide
	stopPrice := m.currentStopPrice
	m.mu.RUnlock()

	if entrySide == "" || stopPrice == 0 {
		return
	}

	klines := analyst.GetKlines()
	if len(klines) > 0 {
		last := klines[len(klines)-1]
		exitSide := "SELL"
		if entrySide == "SELL" {
			exitSide = "BUY"
		}
		if entrySide == "BUY" && last.Low <= stopPrice {
			m.closeVirtualPosition(exitSide, "Stop Loss", stopPrice, analyst)
			return
		}
		if entrySide == "SELL" && last.High >= stopPrice {
			m.closeVirtualPosition(exitSide, "Stop Loss", stopPrice, analyst)
			return
		}
	}

	if !analyst.HasMinBars(minScoreBars) {
		return
	}

	decision := m.evaluateScoreDecision(analyst)
	switch entrySide {
	case "BUY":
		if decision.FinalAction == BuyAction {
			return
		}
		if decision.FinalAction == SellAction {
			m.reverseVirtualPosition("SELL", analyst.LastClose(), decision, analyst)
		}
	case "SELL":
		if decision.FinalAction == SellAction {
			return
		}
		if decision.FinalAction == BuyAction {
			m.reverseVirtualPosition("BUY", analyst.LastClose(), decision, analyst)
		}
	}
}

func (m *MasterGeneral) manageReversal() {
	// During warmup only protective exits (exchange SL) are active — no reversal orders.
	analyst := m.workAnalyst()
	if analyst == nil {
		return
	}
	if !analyst.HasMinBars(minScoreBars) {
		return
	}

	m.mu.RLock()
	entrySide := m.targetSide
	entryPrice := m.entryPrice
	entryTime := m.entryTimeSec
	stopPrice := m.currentStopPrice
	qty := m.positionQty
	balanceBefore := m.balance
	m.mu.RUnlock()

	if entrySide == "" || qty <= 0 || m.exchangeClient == nil {
		return
	}

	decision := m.evaluateScoreDecision(analyst)

	var newSide string
	switch entrySide {
	case "BUY":
		if decision.FinalAction == BuyAction {
			return
		}
		if decision.FinalAction != SellAction {
			return
		}
		newSide = "SELL"
	case "SELL":
		if decision.FinalAction == SellAction {
			return
		}
		if decision.FinalAction != BuyAction {
			return
		}
		newSide = "BUY"
	default:
		return
	}

	if err := m.signalAnalyst.AnalyzeSignals(analyst, newSide); err != nil {
		log.Printf("[Master] ⛔ Reverse blocked by Analyst: %v (%s)", err, RiskErrorLabel(err))
		return
	}

	klines := analyst.GetKlines()
	stopLossPrice := computePositionStop(klines, len(klines)-1, newSide == "BUY", analyst.LastATR(), GetRiskSettings())
	if stopLossPrice <= 0 {
		log.Println("[Master] ❌ Fractal stop level unavailable for reverse")
		return
	}

	exitSide := "SELL"
	closeMarker := "CLOSE_LONG"
	if entrySide == "SELL" {
		exitSide = "BUY"
		closeMarker = "CLOSE_SHORT"
	}

	log.Printf("[Master] 🔄 Reverse: %s -> %s | %s", entrySide, newSide, decision.Reason)
	logTradeSource(len(analyst.GetKlines())-1, newSide, decision)

	closePrice := analyst.LastClose()
	exitTime := barTimeFromAnalyst(analyst)
	exitReason := FormatExitReason("Reverse Signal", entrySide)
	m.emitClosedTrade(entrySide, entryPrice, closePrice, stopPrice, qty, entryTime, exitTime, exitReason, balanceBefore, false)

	m.fireTradeEvent(TradeEvent{
		Side:    closeMarker,
		Price:   closePrice,
		BarTime: exitTime,
		Reason:  exitReason,
		Kind:    "exit",
	})

	if _, err := m.exchangeClient.CreateMarketOrder(m.symbol, exitSide, qty); err != nil {
		log.Printf("[Master] ❌ Reverse close failed: %v", err)
		return
	}
	_ = m.exchangeClient.CancelAllOpenOrders(m.symbol)

	m.mu.Lock()
	m.positionQty = 0
	m.currentStopPrice = 0
	m.targetSide = ""
	m.entryPrice = 0
	m.entryTimeSec = 0
	m.mu.Unlock()
	m.setState(StateIdle)

	m.openLivePosition(newSide, analyst, decision, stopLossPrice)
}

func (m *MasterGeneral) evaluateScoreDecision(marker *Marker) ScoreDecision {
	decision := CalculateScoreGlobal(marker)
	return ApplyExecutionVetoes(decision, marker, m.signalAnalyst, m.chief)
}

func parseOrderID(response string) (int64, error) {
	var parsed struct {
		OrderID int64 `json:"orderId"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return 0, err
	}
	return parsed.OrderID, nil
}

// managePosition syncs live exchange state when the fractal stop order fills.
func (m *MasterGeneral) managePosition() {
	if m.readOnly || m.exchangeClient == nil {
		return
	}

	m.mu.Lock()
	shouldSync := time.Since(m.lastPositionSync) >= positionSyncInterval
	if shouldSync {
		m.lastPositionSync = time.Now()
	}
	m.mu.Unlock()

	if !shouldSync {
		return
	}

	currentPos, err := m.exchangeClient.GetPositionAmt(m.symbol)
	if err != nil {
		log.Printf("[Master] ⚠️ Failed to sync position with exchange: %v", err)
		return
	}
	if math.Abs(currentPos) >= 1e-12 {
		return
	}

	log.Println("[Master] 🛑 Position is ZERO on exchange! Fractal stop was hit.")

	m.mu.RLock()
	entrySide := m.targetSide
	entryPrice := m.entryPrice
	entryTime := m.entryTimeSec
	stopPrice := m.currentStopPrice
	qty := m.positionQty
	balanceBefore := m.balance
	m.mu.RUnlock()

	closeMarker := "CLOSE_LONG"
	if entrySide == "SELL" {
		closeMarker = "CLOSE_SHORT"
	}

	barTime := time.Now().Unix()
	price := stopPrice
	if price <= 0 {
		price = 0.0
	}
	if work := m.workAnalyst(); work != nil {
		barTime = barTimeFromAnalyst(work)
		if price <= 0 {
			price = work.LastClose()
		}
	}

	exitReason := FormatExitReason("Stop Loss", entrySide)
	m.emitClosedTrade(entrySide, entryPrice, price, stopPrice, qty, entryTime, barTime, exitReason, balanceBefore, false)

	m.fireTradeEvent(TradeEvent{
		Side:    closeMarker,
		Price:   price,
		BarTime: barTime,
		Reason:  exitReason,
		Kind:    "exit",
	})

	if cancelErr := m.exchangeClient.CancelAllOpenOrders(m.symbol); cancelErr != nil {
		log.Printf("[Master] ⚠️ Failed to cancel pending orders: %v", cancelErr)
	}

	m.mu.Lock()
	m.positionQty = 0
	m.currentStopPrice = 0
	m.targetSide = ""
	m.entryPrice = 0
	m.entryTimeSec = 0
	m.state = StateIdle
	m.mu.Unlock()
}

// routeTick is the single canonical delivery path for one WS tick (live or
// replayed from the boot buffer): Marker update → chart callback → indicators.
// One candle = one lifecycle: BootController reconcile uses exactly this path.
func (m *MasterGeneral) routeTick(tick exchange.WsTick) {
	m.mu.RLock()
	analyst, exists := m.analysts[tick.Timeframe]
	m.mu.RUnlock()

	if !exists {
		return
	}

	analyst.UpdateKlineTick(tick.Kline, tick.IsClosed)

	m.mu.RLock()
	klineCB := m.onKlineBar
	m.mu.RUnlock()
	if klineCB != nil {
		// Shot 9B: every TF gets atomic chart delivery (OHLCV + plots).
		klineCB(tick.Timeframe, tick.Kline, tick.IsClosed)
	}

	if tick.IsClosed {
		analyst.UpdateIndicators()
	}

	// TODO: Debt — legacy scalper triggers (MTF refresh, TickLiveCh, onTelemetry)
	// stay frozen here until ScoreMatrix/Falcon are re-enabled in later phases.
}

// StartDataFeed runs a background listener that routes WebSocket ticks to analysts
// and generates state-machine triggers on closed candles.
func (m *MasterGeneral) StartDataFeed(ctx context.Context, wsOutCh <-chan exchange.WsTick) {
	go func() {
		log.Println("[Master] Data feed router started...")
		for {
			select {
			case <-ctx.Done():
				log.Println("[Master] Data feed router stopped.")
				return
			case tick, ok := <-wsOutCh:
				if !ok {
					log.Println("[Master] Data feed channel closed.")
					return
				}
				m.routeTick(tick)
			}
		}
	}()
}
