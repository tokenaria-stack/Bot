package strategy

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"sync"
	"time"

	"trading_bot/exchange"
	"trading_bot/execution"
	"trading_bot/vector_db"
)

const (
	positionSymbol         = "BTCUSDT"
	positionSyncInterval   = 5 * time.Second
	orderCooldown          = 3 * time.Second
	defaultFeeRate         = 0.0012 // Binance USDⓈ-M Futures taker approx.
	DefaultHuntTimeframe   = "1m"
)

type MasterState string

const (
	StateIdle       MasterState = "IDLE"
	StateInPosition MasterState = "IN_POSITION"
)

type MasterGeneral struct {
	mu          sync.RWMutex
	state       MasterState
	targetSide  string
	analysts         map[string]*Marker
	signalAnalyst    *Analyst
	chief            *ChiefAnalyst
	positionSizer    *execution.RiskManager
	exchangeClient   *exchange.BinanceExchange
	readOnly         bool
	sandboxMode      bool
	huntTimeframe    string
	balance          float64
	positionQty          float64
	entryPrice           float64
	currentStopPrice     float64
	lastPositionSync     time.Time
	lastOrderTime        time.Time
	feeRate              float64
	memoryStore          *vector_db.MemoryStore
	onTick               func(kline exchange.Kline, jurik, redLine, greenLine, blueLine float64)
	onKlineBar             func(timeframe string, kline exchange.Kline)
	onTrade              func(event TradeEvent)

	TickHuntCh  chan struct{}
	TickLiveCh chan struct{}
}

func NewMasterGeneral(
	analysts map[string]*Marker,
	signalAnalyst *Analyst,
	positionSizer *execution.RiskManager,
	exchangeClient *exchange.BinanceExchange,
	memoryStore *vector_db.MemoryStore,
	readOnly bool,
	sandboxMode bool,
	huntTimeframe string,
) *MasterGeneral {
	if huntTimeframe == "" {
		huntTimeframe = DefaultHuntTimeframe
	}
	if signalAnalyst == nil {
		signalAnalyst = NewAnalyst(sandboxMode)
	}
	return &MasterGeneral{
		state:          StateIdle,
		analysts:       analysts,
		signalAnalyst:  signalAnalyst,
		chief:          NewChiefAnalyst(),
		positionSizer:  positionSizer,
		exchangeClient: exchangeClient,
		memoryStore:    memoryStore,
		readOnly:       readOnly,
		sandboxMode:    sandboxMode,
		huntTimeframe:  huntTimeframe,
		feeRate:        defaultFeeRate,
		TickHuntCh:     make(chan struct{}, 100),
		TickLiveCh:     make(chan struct{}, 1000),
	}
}

// SetOnTick registers a callback invoked after each 1m kline update with live oscillator values.
func (m *MasterGeneral) SetOnTick(fn func(kline exchange.Kline, jurik, redLine, greenLine, blueLine float64)) {
	m.mu.Lock()
	m.onTick = fn
	m.mu.Unlock()
}

// SetOnKlineBar registers a callback for live kline updates on any subscribed timeframe.
func (m *MasterGeneral) SetOnKlineBar(fn func(timeframe string, kline exchange.Kline)) {
	m.mu.Lock()
	m.onKlineBar = fn
	m.mu.Unlock()
}

// SetOnTrade registers a callback invoked when a scalp entry or exit is executed.
func (m *MasterGeneral) SetOnTrade(fn func(event TradeEvent)) {
	m.mu.Lock()
	m.onTrade = fn
	m.mu.Unlock()
}

// RecoverState is called once at bot startup.
// It checks for open positions on the exchange and syncs internal state.
func (m *MasterGeneral) RecoverState() {
	if m.sandboxMode {
		m.resetToCleanState()
		log.Println("[Master] Sandbox mode: starting in IDLE (Pure Strategy, risk vetoes bypassed)")
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

	amt, err := m.exchangeClient.GetPositionAmt(positionSymbol)
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

		if err := m.exchangeClient.CancelAllOpenOrders(positionSymbol); err != nil {
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
	m.balance = 0
	m.positionQty = 0
	m.entryPrice = 0
	m.currentStopPrice = 0
	m.mu.Unlock()
}

func (m *MasterGeneral) Run(ctx context.Context) {
	log.Println("[Master] Event Loop started.")
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.TickHuntCh:
			m.handleHuntTick()
		case <-m.TickLiveCh:
			m.handleLiveTick()
		}
	}
}

func (m *MasterGeneral) handleHuntTick() {
	log.Printf("[Master] ⏱ %s candle closed — evaluating strategy...", m.huntTimeframe)

	m.mu.RLock()
	state := m.state
	m.mu.RUnlock()

	switch state {
	case StateInPosition:
		if m.readOnly || m.sandboxMode {
			m.manageVirtualPosition(false)
		} else {
			m.manageReversal()
		}
	default:
		m.huntForEntry()
	}
}

func (m *MasterGeneral) handleLiveTick() {
	m.mu.RLock()
	state := m.state
	readOnly := m.readOnly
	m.mu.RUnlock()

	if state == StateInPosition && readOnly {
		m.manageVirtualPosition(true)
		return
	}

	if state == StateInPosition && !readOnly {
		m.managePosition()
	}
}

func (m *MasterGeneral) getState() MasterState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *MasterGeneral) setState(newState MasterState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != newState {
		log.Printf("[Master] State transition: %s -> %s", m.state, newState)
		m.state = newState
	}
}

func (m *MasterGeneral) huntAnalyst() *Marker {
	if a, ok := m.analysts[m.huntTimeframe]; ok {
		return a
	}
	return m.analysts[DefaultHuntTimeframe]
}

// huntForEntry looks for a setup on the hunt timeframe via Layer 3 scoring + entry risk gate.
func (m *MasterGeneral) huntForEntry() {
	m.mu.RLock()
	state := m.state
	m.mu.RUnlock()
	if state == StateInPosition {
		return
	}

	analyst := m.huntAnalyst()
	if analyst == nil {
		return
	}
	report, err := analyst.GenerateMarketReport()
	if err != nil {
		return
	}

	log.Printf("[FALCON] %s Jurik: %.2f | Red: %.2f | Green: %.2f | RSX: %q | VolCross: %q",
		m.huntTimeframe, report.Falcon.JurikRSX, report.Falcon.RedLine, report.Falcon.GreenLine,
		report.RSXMarker, report.Falcon.VolCrossMarker)

	decision := m.evaluateScalpDecision(report)
	if decision.Action != BuyAction && decision.Action != SellAction {
		return
	}

	entrySide := string(decision.Action)

	direction := "LONG"
	if entrySide == "SELL" {
		direction = "SHORT"
	}
	log.Printf("[Master] 🔥 SCALP TRIGGER: %s | Score: %d | Entry: %.2f | LotMod: %.2f | %s",
		direction, decision.Score, report.Close, decision.LotMod, decision.Reason)

	klines := analyst.GetKlines()
	stopLossPrice := computePositionStop(klines, len(klines)-1, entrySide == "BUY", report.Volatility.ATR, GetRiskSettings())
	if stopLossPrice <= 0 {
		log.Println("[Master] ❌ Fractal stop level unavailable")
		return
	}

	if m.readOnly || m.sandboxMode {
		log.Printf("[Paper Trading] Virtual order executed. %s @ %.2f | SL %.2f | %s",
			entrySide, report.Close, stopLossPrice, decision.Reason)
		m.openVirtualPosition(entrySide, report.Close, stopLossPrice, decision, analyst)
		return
	}

	m.openLivePosition(entrySide, report, decision, analyst, stopLossPrice)
}

func (m *MasterGeneral) openLivePosition(
	entrySide string,
	report *Report,
	decision ScalpDecision,
	analyst *Marker,
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

	orderReq, err := m.calculateSafePositionSize(*report, stopLossPrice, entrySide, availableBalance)
	if err != nil {
		log.Printf("[Master] ❌ Risk evaluation failed: %v", err)
		return
	}

	orderReq.Quantity *= decision.LotMod
	if orderReq.Quantity <= 0 {
		log.Println("[Master] ❌ Position size is zero after LotMod adjustment")
		return
	}

	log.Printf("[Master] 🔥 SCALP EXEC: %s | Fractal SL: %.2f | Qty: %.8f",
		direction, stopLossPrice, orderReq.Quantity)

	m.fireTradeEvent(TradeEvent{
		Side:    entrySide,
		Price:   report.Close,
		BarTime: barTimeFromAnalyst(analyst),
		Reason:  decision.Reason,
		Kind:    "entry",
	})

	if err := m.exchangeClient.ChangeLeverage(orderReq.Symbol, orderReq.Leverage); err != nil {
		log.Printf("[Master] ❌ Failed to set leverage %dx: %v", orderReq.Leverage, err)
		m.setState(StateIdle)
		return
	}

	response, err := m.exchangeClient.CreateMarketOrder(
		orderReq.Symbol,
		orderReq.Side,
		orderReq.Quantity,
	)
	if err != nil {
		log.Printf("[Master] ❌ Order failed: %v", err)
		m.setState(StateIdle)
		return
	}

	orderID, parseErr := parseOrderID(response)
	if parseErr != nil {
		log.Printf("[Master] ✅ Order sent | %s %s qty=%.8f | response: %s",
			orderReq.Side, orderReq.Symbol, orderReq.Quantity, response)
	} else {
		log.Printf("[Master] ✅ Order executed! OrderID=%d | %s %s qty=%.8f",
			orderID, orderReq.Side, orderReq.Symbol, orderReq.Quantity)
	}

	m.mu.Lock()
	m.positionQty = orderReq.Quantity
	m.entryPrice = report.Close
	m.mu.Unlock()

	stopSide := "SELL"
	if entrySide == "SELL" {
		stopSide = "BUY"
	}

	err = m.exchangeClient.CreateStopMarketOrder(
		orderReq.Symbol,
		stopSide,
		orderReq.Quantity,
		stopLossPrice,
	)
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

func (m *MasterGeneral) calculateSafePositionSize(
	report Report,
	stopLoss float64,
	side string,
	availableBalance float64,
) (*execution.OrderRequest, error) {
	signal := execution.TradeSignal{
		Symbol:   positionSymbol,
		Side:     side,
		Price:    report.Close,
		StopLoss: stopLoss,
	}
	return m.positionSizer.EvaluateSignal(signal, availableBalance)
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
	decision ScalpDecision,
	analyst *Marker,
) {
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
	m.currentStopPrice = stopPrice
	m.positionQty = 1
	m.mu.Unlock()

	m.setState(StateInPosition)
	log.Printf("[Paper Trading] Virtual position opened: %s @ %.2f | Fractal SL %.2f",
		entrySide, entryPrice, stopPrice)
}

func (m *MasterGeneral) closeVirtualPosition(exitSide, trigger string, price float64, analyst *Marker) {
	m.mu.RLock()
	entrySide := m.targetSide
	m.mu.RUnlock()

	closeSide := "CLOSE_LONG"
	if entrySide == "SELL" {
		closeSide = "CLOSE_SHORT"
	}

	reason := FormatExitReason(trigger, entrySide)
	log.Printf("[Paper Trading] Virtual position closed. %s @ %.2f | %s", closeSide, price, reason)

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
	m.currentStopPrice = 0
	m.positionQty = 0
	m.mu.Unlock()

	m.setState(StateIdle)
}

func (m *MasterGeneral) reverseVirtualPosition(newSide string, report Report, decision ScalpDecision, analyst *Marker) {
	m.mu.RLock()
	entrySide := m.targetSide
	m.mu.RUnlock()

	exitSide := "SELL"
	if entrySide == "SELL" {
		exitSide = "BUY"
	}

	m.closeVirtualPosition(exitSide, "Reverse Signal", report.Close, analyst)

	if err := m.signalAnalyst.AnalyzeSignals(&report, newSide); err != nil {
		log.Printf("[Master] ⛔ Reverse blocked by Analyst: %v (%s)", err, RiskErrorLabel(err))
		return
	}

	klines := analyst.GetKlines()
	stopLossPrice := computePositionStop(klines, len(klines)-1, newSide == "BUY", report.Volatility.ATR, GetRiskSettings())
	if stopLossPrice <= 0 {
		log.Println("[Master] ❌ Fractal stop level unavailable for reverse")
		return
	}

	decision = m.chief.Approve(decision, report)
	m.openVirtualPosition(newSide, report.Close, stopLossPrice, decision, analyst)
}

// manageVirtualPosition handles paper exits: 1m fractal SL checks and hunt-TF reversal signals.
func (m *MasterGeneral) manageVirtualPosition(liveOnly bool) {
	priceAnalyst := m.analysts["1m"]
	if priceAnalyst == nil {
		priceAnalyst = m.huntAnalyst()
	}
	if priceAnalyst == nil {
		return
	}

	m.mu.RLock()
	entrySide := m.targetSide
	stopPrice := m.currentStopPrice
	m.mu.RUnlock()

	if entrySide == "" || stopPrice == 0 {
		return
	}

	klines := priceAnalyst.GetKlines()
	if len(klines) > 0 {
		last := klines[len(klines)-1]
		exitSide := "SELL"
		if entrySide == "SELL" {
			exitSide = "BUY"
		}
		if entrySide == "BUY" && last.Low <= stopPrice {
			m.closeVirtualPosition(exitSide, "Stop Loss", stopPrice, priceAnalyst)
			return
		}
		if entrySide == "SELL" && last.High >= stopPrice {
			m.closeVirtualPosition(exitSide, "Stop Loss", stopPrice, priceAnalyst)
			return
		}
	}

	if liveOnly {
		return
	}

	huntAnalyst := m.huntAnalyst()
	if huntAnalyst == nil {
		return
	}
	huntReport, err := huntAnalyst.GenerateMarketReport()
	if err != nil {
		return
	}

	decision := m.evaluateScalpDecision(huntReport)
	switch entrySide {
	case "BUY":
		if decision.Action == BuyAction {
			return
		}
		if decision.Action == SellAction {
			m.reverseVirtualPosition("SELL", *huntReport, decision, huntAnalyst)
		}
	case "SELL":
		if decision.Action == SellAction {
			return
		}
		if decision.Action == BuyAction {
			m.reverseVirtualPosition("BUY", *huntReport, decision, huntAnalyst)
		}
	}
}

func (m *MasterGeneral) manageReversal() {
	analyst := m.huntAnalyst()
	if analyst == nil {
		return
	}
	report, err := analyst.GenerateMarketReport()
	if err != nil {
		return
	}

	m.mu.RLock()
	entrySide := m.targetSide
	qty := m.positionQty
	m.mu.RUnlock()

	if entrySide == "" || qty <= 0 || m.exchangeClient == nil {
		return
	}

	decision := m.evaluateScalpDecision(report)

	var newSide string
	switch entrySide {
	case "BUY":
		if decision.Action == BuyAction {
			return
		}
		if decision.Action != SellAction {
			return
		}
		newSide = "SELL"
	case "SELL":
		if decision.Action == SellAction {
			return
		}
		if decision.Action != BuyAction {
			return
		}
		newSide = "BUY"
	default:
		return
	}

	if err := m.signalAnalyst.AnalyzeSignals(report, newSide); err != nil {
		log.Printf("[Master] ⛔ Reverse blocked by Analyst: %v (%s)", err, RiskErrorLabel(err))
		return
	}

	klines := analyst.GetKlines()
	stopLossPrice := computePositionStop(klines, len(klines)-1, newSide == "BUY", report.Volatility.ATR, GetRiskSettings())
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

	m.fireTradeEvent(TradeEvent{
		Side:    closeMarker,
		Price:   report.Close,
		BarTime: barTimeFromAnalyst(analyst),
		Reason:  FormatExitReason("Reverse Signal", entrySide),
		Kind:    "exit",
	})

	if _, err := m.exchangeClient.CreateMarketOrder(positionSymbol, exitSide, qty); err != nil {
		log.Printf("[Master] ❌ Reverse close failed: %v", err)
		return
	}
	_ = m.exchangeClient.CancelAllOpenOrders(positionSymbol)

	m.mu.Lock()
	m.positionQty = 0
	m.currentStopPrice = 0
	m.targetSide = ""
	m.mu.Unlock()
	m.setState(StateIdle)

	m.openLivePosition(newSide, report, m.chief.Approve(decision, *report), analyst, stopLossPrice)
}

func (m *MasterGeneral) evaluateScalpDecision(report *Report) ScalpDecision {
	scoreResult := ProcessScore(context.Background(), *report, m.feeRate, m.memoryStore)
	decision := ScalpDecisionFromScoreResult(scoreResult, *report)
	if decision.Action != BuyAction && decision.Action != SellAction {
		return decision
	}
	if err := m.signalAnalyst.AnalyzeSignals(report, string(decision.Action)); err != nil {
		log.Printf("[Master] ⛔ Entry blocked by Analyst: %v (%s)", err, RiskErrorLabel(err))
		return ScalpDecision{Action: WaitAction, Reason: "Analyst blocked"}
	}
	return m.chief.Approve(decision, *report)
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

	currentPos, err := m.exchangeClient.GetPositionAmt(positionSymbol)
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
	m.mu.RUnlock()

	closeMarker := "CLOSE_LONG"
	if entrySide == "SELL" {
		closeMarker = "CLOSE_SHORT"
	}

	barTime := time.Now().Unix()
	price := 0.0
	if microAnalyst, ok := m.analysts["1m"]; ok {
		barTime = barTimeFromAnalyst(microAnalyst)
		if report, err := microAnalyst.GenerateMarketReport(); err == nil {
			price = report.Close
		}
	}

	m.fireTradeEvent(TradeEvent{
		Side:    closeMarker,
		Price:   price,
		BarTime: barTime,
		Reason:  FormatExitReason("Stop Loss", entrySide),
		Kind:    "exit",
	})

	if cancelErr := m.exchangeClient.CancelAllOpenOrders(positionSymbol); cancelErr != nil {
		log.Printf("[Master] ⚠️ Failed to cancel pending orders: %v", cancelErr)
	}

	m.mu.Lock()
	m.positionQty = 0
	m.currentStopPrice = 0
	m.targetSide = ""
	m.state = StateIdle
	m.mu.Unlock()
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

				m.mu.RLock()
				analyst, exists := m.analysts[tick.Timeframe]
				m.mu.RUnlock()

				if !exists {
					continue
				}

				analyst.UpdateKlineTick(tick.Kline, tick.IsClosed)

				m.mu.RLock()
				klineCB := m.onKlineBar
				m.mu.RUnlock()
				if klineCB != nil {
					klineCB(tick.Timeframe, tick.Kline)
				}

				if tick.Timeframe == "1m" {
					m.mu.RLock()
					tickCB := m.onTick
					m.mu.RUnlock()
					if tickCB != nil {
						if report, err := analyst.GenerateMarketReport(); err == nil {
							tickCB(
								tick.Kline,
								report.JurikValue,
								report.Falcon.RedLine,
								report.Falcon.GreenLine,
								report.Falcon.BlueLine,
							)
						}
					}

					select {
					case m.TickLiveCh <- struct{}{}:
					default:
					}
				}

				if tick.IsClosed {
					analyst.UpdateIndicators()
				}

				m.mu.RLock()
				huntTF := m.huntTimeframe
				m.mu.RUnlock()
				if tick.Timeframe == huntTF && tick.IsClosed {
					select {
					case m.TickHuntCh <- struct{}{}:
					default:
					}
				}
			}
		}
	}()
}
