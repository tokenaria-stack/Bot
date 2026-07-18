package market

import (
	"context"
	"log"
	"sync"
	"time"
	"trading_bot/decision"

	"trading_bot/data"
	"trading_bot/exchange"
)

const (
	DefaultTradingTimeframe    = "1m"
	tradingHistoryPrefetchBars = IndicatorWarmupBars
)

// RuntimeState is a contract socket for the future strategy FSM (Core 5.x).
// The Data-core itself never leaves StateIdle.
type RuntimeState string

const (
	StateIdle       RuntimeState = "IDLE"
	StateInPosition RuntimeState = "IN_POSITION"
)

// Runtime routes market data: WS ticks → TFrames → chart callbacks →
// persistence. All trading/scoring logic was purged in Core 5.0 Phase F;
// strategies will plug back in through ScoreEngine contracts (score_types.go).
type Runtime struct {
	mu             sync.RWMutex
	state          RuntimeState
	frames         map[string]*Frame
	exchangeClient *exchange.BinanceExchange
	htfProvider    *exchange.HTFProvider
	readOnly       bool
	sandboxMode    bool
	symbol         string
	timeframe      string
	onKlineBar     func(timeframe string, kline exchange.Kline, isClosed bool)

	TickLiveCh chan struct{}

	persistQ *data.PersistenceQueue

	mtfTracker      *WalkForwardMTFTracker
	navigatorPanes  map[string]NavigatorUISettings
	navMu           sync.RWMutex
	closedTelemetry map[string]closedBarTelemetry
}

type closedBarTelemetry struct {
	score  decision.ScoreDecision
	regime string
}

func NewRuntime(
	frames map[string]*Frame,
	exchangeClient *exchange.BinanceExchange,
	htfProvider *exchange.HTFProvider,
	readOnly bool,
	sandboxMode bool,
	symbol string,
	timeframe string,
) *Runtime {
	if timeframe == "" {
		timeframe = DefaultTradingTimeframe
	}
	return &Runtime{
		state:           StateIdle,
		frames:          frames,
		exchangeClient:  exchangeClient,
		htfProvider:     htfProvider,
		readOnly:        readOnly,
		sandboxMode:     sandboxMode,
		symbol:          symbol,
		timeframe:       timeframe,
		TickLiveCh:      make(chan struct{}, 1000),
		closedTelemetry: make(map[string]closedBarTelemetry),
	}
}

// TradingTimeframe returns the active decision timeframe.
func (m *Runtime) TradingTimeframe() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.timeframe != "" {
		return m.timeframe
	}
	return DefaultTradingTimeframe
}

func (m *Runtime) HTFProvider() *exchange.HTFProvider {
	return m.htfProvider
}

// SetNavigatorPanes updates walk-forward MTF navigator settings (from dashboard UI).
func (m *Runtime) SetNavigatorPanes(panes map[string]NavigatorUISettings) {
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

func (m *Runtime) rebuildMTFTrackerIfReady() {
	if m == nil || m.htfProvider == nil {
		m.mtfTracker = nil
		return
	}
	frame := m.workFrame()
	if frame == nil || !frame.HasMinBars(IndicatorWarmupBars) {
		m.mtfTracker = nil
		log.Printf("[Master] MTF tracker deferred: warming up (%d bars, need %d)",
			barCount(frame), IndicatorWarmupBars)
		return
	}
	m.rebuildMTFTracker()
}

func (m *Runtime) rebuildMTFTracker() {
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
	if frame := m.workFrame(); frame != nil {
		klines := frame.GetKlines()
		if len(klines) > 0 {
			tracker.SetChartStartMs(klines[0].OpenTime)
		}
	}
	tracker.Prefetch()
	m.mtfTracker = tracker
	log.Printf("[Master] Walk-forward MTF tracker ready: %v", tfs)
}

func barCount(frame *Frame) int {
	if frame == nil {
		return 0
	}
	return len(frame.GetKlines())
}

// NotifyKlineGapFillComplete is called after background SQLite gap-fill for a symbol/interval.
func (m *Runtime) NotifyKlineGapFillComplete(symbol, interval string) {
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

func (m *Runtime) hydrateTradingHistoryFromStore(symbol, interval string) {
	m.mu.RLock()
	frame := m.frames[interval]
	workTF := m.timeframe
	m.mu.RUnlock()
	if frame == nil {
		frame = m.workFrame()
	}
	if frame == nil {
		return
	}
	m.reconcileKlineGap(symbol, interval, frame)
	if interval == workTF {
		log.Printf("[Master] Hydrated trading history: %s %s (%d bars)", symbol, interval, len(frame.GetKlines()))
	}
}

func (m *Runtime) ensureMTFTrackerReady(frame *Frame) {
	if m == nil || m.mtfTracker != nil {
		return
	}
	if frame == nil || !frame.HasMinBars(IndicatorWarmupBars) {
		return
	}
	m.rebuildMTFTracker()
}

func (m *Runtime) syncMTFState(frame *Frame) {
	if m == nil || frame == nil || m.mtfTracker == nil {
		return
	}
	klines := frame.GetKlines()
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
	frame.SetCurrentMTFState(m.mtfTracker.States())
}

// SetPersistenceQueue binds the sole SQLite archive writer (Shot 9E).
func (m *Runtime) SetPersistenceQueue(q *data.PersistenceQueue) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.persistQ = q
	m.mu.Unlock()
}

// ScoreDecisionForTelemetry returns the last closed-bar ScoreDecision for dashboard/API consumers.
// Contract socket: empty until a strategy engine is plugged back in (Core 5.x).
func (m *Runtime) ScoreDecisionForTelemetry(marker *Frame) decision.ScoreDecision {
	if m == nil || marker == nil {
		return decision.ScoreDecision{Factors: make(map[string]decision.ScoreFactor)}
	}
	tf := marker.Timeframe()
	if tf == "" {
		tf = m.TradingTimeframe()
	}
	if t, ok := m.closedBarTelemetryFor(tf); ok {
		return t.score
	}
	return decision.ScoreDecision{Factors: make(map[string]decision.ScoreFactor)}
}

// ClosedVolatilityRegimeForTelemetry returns the regime fixed at the last closed bar.
func (m *Runtime) ClosedVolatilityRegimeForTelemetry(marker *Frame) string {
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

// SeedClosedBarTelemetry snapshots closed-bar regime from current Frame state.
// Call once after history hydration (not on GET /api/state).
func (m *Runtime) SeedClosedBarTelemetry() {
	if m == nil {
		return
	}
	m.mu.RLock()
	workTF := m.timeframe
	frame := m.frames[workTF]
	m.mu.RUnlock()
	if frame == nil {
		return
	}
	m.ensureMTFTrackerReady(frame)
	m.syncMTFState(frame)
}

func (m *Runtime) closedBarTelemetryFor(tf string) (closedBarTelemetry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.closedTelemetry[tf]
	return t, ok
}

// SetOnKlineBar registers a callback for live kline updates on any subscribed timeframe.
// isClosed is Binance k.x (bar finalized). Shot 9B: used for atomic chart delivery on every TF.
func (m *Runtime) SetOnKlineBar(fn func(timeframe string, kline exchange.Kline, isClosed bool)) {
	m.mu.Lock()
	m.onKlineBar = fn
	m.mu.Unlock()
}

// Run drains TickLiveCh. In the sterile Data-core the handler only keeps
// chart-facing MTF state fresh; strategy evaluation plugs in here later.
func (m *Runtime) Run(ctx context.Context) {
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

func (m *Runtime) handleLiveTick() {
	frame := m.workFrame()
	if frame == nil || !frame.HasMinBars(IndicatorWarmupBars) {
		return
	}
	m.ensureMTFTrackerReady(frame)
	m.syncMTFState(frame)
}

// State returns the current master state machine value for dashboard telemetry.
// Always StateIdle until a strategy FSM is plugged back in.
func (m *Runtime) State() RuntimeState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *Runtime) workFrame() *Frame {
	if a, ok := m.frames[m.timeframe]; ok {
		return a
	}
	return m.frames[DefaultTradingTimeframe]
}

// routeTick is the single canonical delivery path for one WS tick (live or
// replayed from the boot buffer): Frame update → chart callback → indicators.
// One candle = one lifecycle: BootController reconcile uses exactly this path.
func (m *Runtime) routeTick(tick exchange.WsTick) {
	m.mu.RLock()
	frame, exists := m.frames[tick.Timeframe]
	m.mu.RUnlock()

	if !exists {
		return
	}

	frame.UpdateKlineTick(tick.Kline, tick.IsClosed)

	m.mu.RLock()
	klineCB := m.onKlineBar
	m.mu.RUnlock()
	if klineCB != nil {
		// Shot 9B: every TF gets atomic chart delivery (OHLCV + plots).
		klineCB(tick.Timeframe, tick.Kline, tick.IsClosed)
	}

	if tick.IsClosed {
		frame.UpdateIndicators()
	}
}

// StartDataFeed runs a background listener that routes WebSocket ticks to frames.
func (m *Runtime) StartDataFeed(ctx context.Context, wsOutCh <-chan exchange.WsTick) {
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
