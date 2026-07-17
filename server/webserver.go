package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"trading_bot/core"
	"trading_bot/data"
	"trading_bot/domain"
	"trading_bot/exchange"
	"trading_bot/server/wire"
	"trading_bot/strategy"
	"trading_bot/ui_config"
)

const (
	defaultTimeframe        = "1m"
	maxCandlesInState       = 500 // legacy default; overridden by limit query param
	defaultStateCandleLimit = 3000
	maxStateCandleLimit     = 10000
	stateTailPollLimit      = 20 // skip navigators/hasMore for lightweight tail polls
	maxBacktestChunkLimit   = 50000
	maxBacktestBars         = 100000
	historyFetchLimit       = 1000
	binanceMaxKlinesLimit   = 1000
	defaultStaticDir        = "web"
)

var errWarmingUp = errors.New("warming_up")

// WSClient wraps a WebSocket connection with a per-connection write mutex.
// gorilla/websocket permits only one concurrent writer per connection.
type WSClient struct {
	Conn *websocket.Conn
	mu   sync.Mutex
}

func (c *WSClient) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteMessage(messageType, data)
}

func (c *WSClient) WriteJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteJSON(v)
}

func (c *WSClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.Close()
}

// DashboardServer serves the trading dashboard UI and market state API.
type DashboardServer struct {
	analysts         map[string]*strategy.Marker
	rest             *exchange.BinanceExchange
	symbol           string
	staticDir        string
	upgrader         websocket.Upgrader
	clients          map[*WSClient]bool
	clientTF         map[*WSClient]string
	clientsMu        sync.Mutex
	tradesMu         sync.RWMutex
	trades           []ChartTrade
	tradeHistory     *domain.TradeHistoryStore
	paperTrading     bool
	sandboxMode      bool
	tradingTimeframe string
	signalAnalyst    *strategy.Analyst
	htfProvider      *exchange.HTFProvider
	liveNavMu        sync.RWMutex
	liveNavigators   map[string]strategy.NavigatorUISettings
	master           *strategy.MasterGeneral
	backtestRuns     *backtestRunManager
	uiRegistry       *core.UIRegistry
	projector        *wire.Projector
	// Shot 9I: rising-edge DivState → WS annotation (per timeframe).
	lastDivMu    sync.Mutex
	lastDivState map[string]float64
}

// MarketState is the JSON payload for GET /api/state.
type MarketState struct {
	Status           string                                 `json:"status,omitempty"`
	Symbol           string                                 `json:"symbol"`
	Timeframe        string                                 `json:"timeframe"`
	TradingTimeframe string                                 `json:"tradingTimeframe"`
	UpdatedAt        int64                                  `json:"updatedAt"`
	VolatilityRegime string                                 `json:"volatilityRegime"`
	Jurik            float64                                `json:"jurik"`
	LongScore        int                                    `json:"longScore"`
	ShortScore       int                                    `json:"shortScore"`
	RawAction        string                                 `json:"rawAction,omitempty"`
	FinalAction      string                                 `json:"finalAction,omitempty"`
	IsVetoed         bool                                   `json:"isVetoed,omitempty"`
	VetoReason       string                                 `json:"vetoReason,omitempty"`
	Factors          map[string]strategy.ScoreFactor        `json:"factors"`
	BrainStatus      string                                 `json:"brainStatus"`
	AIStatus         string                                 `json:"aiStatus"`
	Candles          []ChartCandle                          `json:"candles"`
	Oscillators      []ChartOscillator                      `json:"oscillators"`
	FibZones         []ChartFibZone                         `json:"fibZones"`
	Trades           []ChartTrade                           `json:"trades,omitempty"`
	MasterState      string                                 `json:"masterState,omitempty"`
	PaperTrading     bool                                   `json:"paperTrading,omitempty"`
	SandboxMode      bool                                   `json:"sandboxMode,omitempty"`
	HasMore          bool                                   `json:"hasMore,omitempty"`
	Navigators       map[string]strategy.NavigatorResultDTO `json:"navigators,omitempty"`
	Annotations      []strategy.ChartAnnotation             `json:"annotations,omitempty"`
	// Tip slot→wire map (component ids). Header/toolbar reads plots — never Falcon line fields.
	Plots map[string]float64 `json:"plots,omitempty"`
}

// ChartTrade is a virtual or live trade marker for the price chart (time in Unix seconds).
type ChartTrade struct {
	Time            int64    `json:"time"`
	Side            string   `json:"side"`
	Price           float64  `json:"price"`
	Reason          string   `json:"reason,omitempty"`
	Kind            string   `json:"kind,omitempty"` // "entry" or "exit"
	EntryReason     string   `json:"entryReason,omitempty"`
	FactorsSnapshot []string `json:"factorsSnapshot,omitempty"`
}

// ChartFibZone is a horizontal Fibonacci level for the price chart overlay.
type ChartFibZone struct {
	Ratio    float64 `json:"ratio"`
	Price    float64 `json:"price"`
	IsActive bool    `json:"isActive"`
}

// ChartCandle is a bar formatted for Lightweight Charts (time in Unix seconds).
type ChartCandle struct {
	Time   int64   `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume,omitempty"`
}

// ChartOscillator is an oscillator point for the sub-panel (time in Unix seconds).
type ChartOscillator struct {
	Time            int64   `json:"time"`
	Jurik           float64 `json:"jurik"`
	RSX             float64 `json:"rsx"`
	RSXSignal       float64 `json:"rsx_signal"`
	Red             float64 `json:"red"`
	Green           float64 `json:"green"`
	RedLine         float64 `json:"redLine"`
	GreenLine       float64 `json:"greenLine"`
	Blue            float64 `json:"blue"`
	RsiPrice        float64 `json:"rsiPrice"`
	EmaRsi          float64 `json:"emaRsi"`
	RsiRsi          float64 `json:"rsiRsi"`
	RsiHl2          float64 `json:"rsiHl2"`
	RsiVolFast      float64 `json:"rsiVolFast"`
	RsiVolSlow      float64 `json:"rsiVolSlow"`
	MacdRsi         float64 `json:"macdRsi"`
	RsiAd           float64 `json:"rsiAd"`
	RsiHl2Vol       float64 `json:"rsiHl2Vol"`
	VolCrossMarker  string  `json:"volCrossMarker,omitempty"`
	VolChanMid      float64 `json:"volChanMid"`
	VolChanUp       float64 `json:"volChanUp"`
	VolChanDn       float64 `json:"volChanDn"`
	PriceChanMid    float64 `json:"priceChanMid"`
	PriceChanUp     float64 `json:"priceChanUp"`
	PriceChanDn     float64 `json:"priceChanDn"`
	Color           string  `json:"color,omitempty"`
	Marker          string  `json:"marker,omitempty"`
	VolumeSpikeUp   bool    `json:"volumeSpikeUp,omitempty"`
	VolumeSpikeDown bool    `json:"volumeSpikeDown,omitempty"`
}

type warmingUpResponse struct {
	Status string `json:"status"`
}

type wsEnvelope struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

type tickPayload struct {
	Timeframe        string                          `json:"timeframe,omitempty"`
	Time             int64                           `json:"time"`
	Open             float64                         `json:"open"`
	High             float64                         `json:"high"`
	Low              float64                         `json:"low"`
	Close            float64                         `json:"close"`
	Volume           float64                         `json:"volume,omitempty"`
	Jurik            float64                         `json:"jurik,omitempty"`
	RSX              float64                         `json:"rsx,omitempty"`
	RSXSignal        float64                         `json:"rsx_signal,omitempty"`
	RSXColor         string                          `json:"rsxColor,omitempty"`
	RSXMarker        string                          `json:"rsxMarker,omitempty"`
	LongScore        int                             `json:"longScore,omitempty"`
	ShortScore       int                             `json:"shortScore,omitempty"`
	RawAction        string                          `json:"rawAction,omitempty"`
	FinalAction      string                          `json:"finalAction,omitempty"`
	IsVetoed         bool                            `json:"isVetoed,omitempty"`
	VetoReason       string                          `json:"vetoReason,omitempty"`
	Factors          map[string]strategy.ScoreFactor `json:"factors,omitempty"`
	BrainStatus      string                          `json:"brainStatus,omitempty"`
	AIStatus         string                          `json:"aiStatus,omitempty"`
	IsClosed         bool                            `json:"isClosed,omitempty"`
	VolatilityRegime string                          `json:"volatilityRegime,omitempty"`
	Plots            map[string]float64              `json:"plots,omitempty"`
	// Shot 9I: DAG DivState → Projector markers (no Falcon).
	Marker      string            `json:"marker,omitempty"`
	Annotations []wire.Annotation `json:"annotations,omitempty"`
}

type markerPayload struct {
	Time   int64   `json:"time"`
	Side   string  `json:"side"`
	Price  float64 `json:"price"`
	Reason string  `json:"reason,omitempty"`
	Kind   string  `json:"kind,omitempty"`
}

type historyChunkResponse struct {
	ChartData   []ChartPoint               `json:"chartData"`
	HasMore     bool                       `json:"hasMore"`
	Annotations []strategy.ChartAnnotation `json:"annotations,omitempty"`
}

type historyResponse struct {
	Status      string                                 `json:"status"`
	Timeframe   string                                 `json:"timeframe"`
	Candles     []ChartCandle                          `json:"candles"`
	Oscillators []ChartOscillator                      `json:"oscillators"`
	Trades      []ChartTrade                           `json:"trades,omitempty"`
	HasMore     bool                                   `json:"hasMore"`
	Navigators  map[string]strategy.NavigatorResultDTO `json:"navigators,omitempty"`
	Annotations []strategy.ChartAnnotation             `json:"annotations,omitempty"`
}

// NewDashboardServer creates a dashboard server bound to Marker instances.
func NewDashboardServer(
	analysts map[string]*strategy.Marker,
	rest *exchange.BinanceExchange,
	symbol string,
	signalAnalyst *strategy.Analyst,
	htfProvider *exchange.HTFProvider,
	paperTrading bool,
	sandboxMode bool,
	tradingTimeframe string,
) *DashboardServer {
	if tradingTimeframe == "" {
		tradingTimeframe = defaultTimeframe
	}
	uiReg, err := ui_config.BuildUIRegistry()
	if err != nil {
		log.Printf("[Dashboard] UI registry build failed: %v", err)
		uiReg = core.NewUIRegistry()
	}
	return &DashboardServer{
		analysts:         analysts,
		rest:             rest,
		symbol:           exchange.NormalizeFuturesSymbol(symbol),
		staticDir:        defaultStaticDir,
		clients:          make(map[*WSClient]bool),
		clientTF:         make(map[*WSClient]string),
		tradeHistory:     domain.NewTradeHistoryStore(),
		backtestRuns:     newBacktestRunManager(),
		paperTrading:     paperTrading,
		sandboxMode:      sandboxMode,
		tradingTimeframe: tradingTimeframe,
		signalAnalyst:    signalAnalyst,
		htfProvider:      htfProvider,
		liveNavigators:   defaultLiveNavigatorPanes(),
		uiRegistry:       uiReg,
		projector:        wire.NewProjector(uiReg),
		lastDivState:     make(map[string]float64),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Start listens on port and serves static assets, /api/state, and /ws.
func (d *DashboardServer) Start(port string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/state", withGzip(d.handleState))
	mux.HandleFunc("/api/history", withGzip(d.handleHistory))
	mux.HandleFunc("/api/history/chunk", withGzip(d.handleHistoryChunk))
	mux.HandleFunc("/api/settings/thresholds", withGzip(d.handleThresholds))
	mux.HandleFunc("/api/settings/matrix", withGzip(d.handleScoringMatrix))
	mux.HandleFunc("/api/settings/indicators", withGzip(d.handleIndicatorSettings))
	mux.HandleFunc("/api/settings/risk", withGzip(d.handleRiskSettings))
	mux.HandleFunc("/api/settings/navigators", withGzip(d.handleNavigatorSettings))
	mux.HandleFunc("/api/ui/manifest", withGzip(d.handleUIManifest))
	mux.HandleFunc("/api/backtest/run", withGzip(d.handleBacktestRun))
	mux.HandleFunc("/api/backtest/stop", withGzip(d.handleBacktestStop))
	mux.HandleFunc("/api/stats", withGzip(d.handleStats))
	mux.HandleFunc("/api/cache/clear", withGzip(d.handleCacheClear))
	mux.HandleFunc("/ws", d.handleWS)

	webRoot, err := filepath.Abs(d.staticDir)
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServer(http.Dir(webRoot)))

	log.Printf("[Dashboard] listening on http://localhost%s", port)
	return http.ListenAndServe(port, mux)
}

// ChartCandleFromDomain converts a domain candle; OpenTime is normalized to ms internally.
// Returns false when OHLC is zero, missing, or non-finite.
func ChartCandleFromDomain(c domain.Candle) (ChartCandle, bool) {
	if !validOHLC(c.Open, c.High, c.Low, c.Close) {
		return ChartCandle{}, false
	}
	return ChartCandle{
		Time:   exchange.ChartTimeSec(c.OpenTime),
		Open:   c.Open,
		High:   c.High,
		Low:    c.Low,
		Close:  c.Close,
		Volume: c.Volume,
	}, true
}

// ChartCandleFromKline converts an exchange kline; OpenTime must be milliseconds.
func ChartCandleFromKline(k exchange.Kline) (ChartCandle, bool) {
	return ChartCandleFromDomain(domain.CandleFromKline(k))
}

func validOHLC(open, high, low, closePrice float64) bool {
	vals := []float64{open, high, low, closePrice}
	for _, v := range vals {
		if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// RouteChartTick delivers an atomic chart frame (OHLCV + DAG plots + header tip) to
// clients subscribed to this timeframe only (Core 4.2 Timeframe-pure Transport).
// Contract: every live tick carries plots when the Analyst DAG frame is available — never price-only.
// Header Jurik/Wozduh come from DAG slots only — never FalconSnapshot.
func (d *DashboardServer) RouteChartTick(timeframe string, candle domain.Candle, isClosed bool, dagFrame *core.TickFrame) {
	chart, ok := ChartCandleFromDomain(candle)
	if !ok {
		return
	}
	if timeframe == "" {
		timeframe = d.tradingTimeframe
	}
	if dagFrame == nil {
		if a := d.analystForTimeframe(timeframe); a != nil {
			dagFrame = a.DAGTickFrame()
		}
	}
	var plots map[string]float64
	if dagFrame != nil && d.projector != nil {
		plots = d.projector.BuildTickJSON(dagFrame)
	}
	marker, anns := d.risingEdgeDivAnnotations(timeframe, dagFrame, chart.Time, isClosed)
	hdr := dagHeaderFromFrame(dagFrame)
	payload := tickPayload{
		Timeframe:   timeframe,
		Time:        chart.Time,
		Open:        chart.Open,
		High:        chart.High,
		Low:         chart.Low,
		Close:       chart.Close,
		Volume:      chart.Volume,
		IsClosed:    isClosed,
		Plots:       plots,
		Marker:      marker,
		Annotations: anns,
		Factors:     map[string]strategy.ScoreFactor{},
	}
	applyDAGHeaderToTick(&payload, hdr)
	d.routeTick(timeframe, wsEnvelope{Type: "tick", Data: payload})
}

// BroadcastChartTick is deprecated — use RouteChartTick (Core 4.2).
func (d *DashboardServer) BroadcastChartTick(timeframe string, candle domain.Candle, isClosed bool, dagFrame *core.TickFrame) {
	d.RouteChartTick(timeframe, candle, isClosed, dagFrame)
}

// BroadcastTick is deprecated (Shot 9J) — forwards to RouteChartTick (DAG header only).
func (d *DashboardServer) BroadcastTick(timeframe string, candle domain.Candle, isClosed bool, dagFrame *core.TickFrame) {
	d.RouteChartTick(timeframe, candle, isClosed, dagFrame)
}

// BroadcastPriceBar is deprecated (Shot 9B) — forwards to RouteChartTick so legacy callers stay atomic.
func (d *DashboardServer) BroadcastPriceBar(timeframe string, candle domain.Candle) {
	d.RouteChartTick(timeframe, candle, false, nil)
}

// risingEdgeDivAnnotations emits a wire marker only when closed-bar DivState changes.
func (d *DashboardServer) risingEdgeDivAnnotations(
	timeframe string,
	frame *core.TickFrame,
	timeSec int64,
	isClosed bool,
) (string, []wire.Annotation) {
	if !isClosed || d == nil || d.projector == nil || frame == nil {
		return "", nil
	}
	state := frame.Get(core.SlotDivState)
	if !jsonSafeDivState(state) {
		state = core.DivStateNone
	}
	d.lastDivMu.Lock()
	prev := d.lastDivState[timeframe]
	d.lastDivState[timeframe] = state
	d.lastDivMu.Unlock()

	if state == core.DivStateNone || state == prev {
		return "", nil
	}
	ann, ok := d.projector.BuildTickAnnotation(frame, timeSec)
	if !ok {
		return "", nil
	}
	return ann.Label, []wire.Annotation{ann}
}

func jsonSafeDivState(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// BroadcastMarker records a trade and pushes a marker to all dashboard clients.
func (d *DashboardServer) BroadcastMarker(side string, price float64, barTime int64, reason, kind string) {
	trade := ChartTrade{
		Time:   barTime,
		Side:   side,
		Price:  price,
		Reason: reason,
		Kind:   kind,
	}
	d.tradesMu.Lock()
	d.trades = append(d.trades, trade)
	d.tradesMu.Unlock()

	d.broadcast(wsEnvelope{
		Type: "marker",
		Data: markerPayload{
			Time:   barTime,
			Side:   side,
			Price:  price,
			Reason: reason,
			Kind:   kind,
		},
	})
}

func (d *DashboardServer) sessionTrades() []ChartTrade {
	d.tradesMu.RLock()
	defer d.tradesMu.RUnlock()
	if len(d.trades) == 0 {
		return nil
	}
	out := make([]ChartTrade, len(d.trades))
	copy(out, d.trades)
	return out
}

// sessionTradesForChart returns executed trade markers for the price chart.
// Telemetry-only signals (FinalAction) never appear here; orphan entry markers at
// the tail are dropped when the master state machine is IDLE (e.g. failed live order).
func (d *DashboardServer) sessionTradesForChart() []ChartTrade {
	trades := d.sessionTrades()
	if len(trades) == 0 {
		return nil
	}
	if d.master == nil || d.master.State() == strategy.StateInPosition {
		return trades
	}
	out := trades
	for len(out) > 0 {
		last := out[len(out)-1]
		kind := strings.ToLower(strings.TrimSpace(last.Kind))
		if kind == "" {
			kind = "entry"
		}
		if kind != "entry" {
			break
		}
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// RecordClosedTrade appends a completed round-trip to live or paper history.
func (d *DashboardServer) RecordClosedTrade(trade domain.ClosedTrade, isVirtual bool) {
	if d == nil || d.tradeHistory == nil {
		return
	}
	if isVirtual {
		d.tradeHistory.AppendVirtual(trade)
		return
	}
	d.tradeHistory.AppendReal(trade)
}

func (d *DashboardServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mode := r.URL.Query().Get("mode")
	switch mode {
	case "live", "paper":
	default:
		http.Error(w, "query param mode must be live or paper", http.StatusBadRequest)
		return
	}

	if d.tradeHistory == nil {
		writeJSON(w, domain.SessionStats{Mode: mode})
		return
	}
	writeJSON(w, d.tradeHistory.StatsForMode(mode))
}

func (d *DashboardServer) broadcast(msg wsEnvelope) {
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Dashboard] broadcast marshal: %v", err)
		return
	}
	d.writeToClients(payload, nil)
}

// routeTick delivers a chart tick only to clients whose subscribed TF matches (Transport purity).
// Case-sensitive: Binance "1m" (minute) ≠ "1M" (month) — never fold case.
func (d *DashboardServer) routeTick(timeframe string, msg wsEnvelope) {
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Dashboard] routeTick marshal: %v", err)
		return
	}
	want := strings.TrimSpace(timeframe)
	if want == "" {
		return
	}
	d.writeToClients(payload, func(client *WSClient) bool {
		tf := strings.TrimSpace(d.clientTF[client])
		return tf == want
	})
}

// writeToClients sends raw WS payload to all clients, or only those matching accept (when non-nil).
func (d *DashboardServer) writeToClients(payload []byte, accept func(*WSClient) bool) {
	d.clientsMu.Lock()
	type dest struct {
		client *WSClient
	}
	snapshot := make([]dest, 0, len(d.clients))
	for client := range d.clients {
		if accept != nil && !accept(client) {
			continue
		}
		snapshot = append(snapshot, dest{client: client})
	}
	d.clientsMu.Unlock()

	var dead []*WSClient
	for _, item := range snapshot {
		if err := item.client.WriteMessage(websocket.TextMessage, payload); err != nil {
			_ = item.client.Close()
			dead = append(dead, item.client)
		}
	}

	if len(dead) == 0 {
		return
	}

	d.clientsMu.Lock()
	for _, client := range dead {
		delete(d.clients, client)
		delete(d.clientTF, client)
	}
	d.clientsMu.Unlock()
}

func parseRSXLookback(r *http.Request) int {
	if r != nil {
		if v := r.URL.Query().Get("rsx_div_lookback"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				return n
			}
		}
	}
	return strategy.GetRSXSettings().DivLookback
}

func hasRSXQueryOverrides(q url.Values) bool {
	keys := []string{
		"rsx_length", "rsx_signal_length", "rsx_source", "rsx_method", "rsx_pivot_radius",
		"min_price_delta_ratio", "min_osc_delta", "rsx_div_lookback",
	}
	for _, k := range keys {
		if v := q.Get(k); v != "" {
			return true
		}
	}
	return false
}

func parseRSXSettingsFromRequest(r *http.Request) strategy.RSXSettings {
	base := strategy.GetRSXSettings()
	if r == nil || !hasRSXQueryOverrides(r.URL.Query()) {
		return base
	}
	q := r.URL.Query()
	patch := strategy.RSXSettings{}
	if v := q.Get("rsx_length"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			patch.Length = n
		}
	}
	if v := q.Get("rsx_signal_length"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			patch.SignalLength = n
		}
	}
	if v := q.Get("rsx_div_lookback"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			patch.DivLookback = n
		}
	}
	if v := q.Get("rsx_source"); v != "" {
		patch.Source = v
	}
	if v := q.Get("rsx_method"); v != "" {
		patch.DivMethod = v
	}
	if v := q.Get("rsx_pivot_radius"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			patch.PivotRadius = n
		}
	}
	out := strategy.NormalizeRSXSettings(mergeRSXSettingsFromPatch(base, patch))
	if v := q.Get("min_price_delta_ratio"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			out.MinPriceDeltaRatio = f
		}
	}
	if v := q.Get("min_osc_delta"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			out.MinOscDelta = f
		}
	}
	return out
}

func mergeRSXSettingsFromPatch(base, patch strategy.RSXSettings) strategy.RSXSettings {
	out := base
	if patch.Length > 0 {
		out.Length = patch.Length
	}
	if patch.DivLookback > 0 {
		out.DivLookback = patch.DivLookback
	}
	if patch.SignalLength > 0 {
		out.SignalLength = patch.SignalLength
	}
	if patch.Source != "" {
		out.Source = patch.Source
	}
	if patch.DivMethod != "" {
		out.DivMethod = patch.DivMethod
	}
	if patch.PivotRadius > 0 {
		out.PivotRadius = patch.PivotRadius
	}
	return out
}

func parseCandleLimit(r *http.Request, defaultLimit, maxLimit int) int {
	if r == nil {
		return defaultLimit
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func isTailPollRequest(r *http.Request, candleLimit int) bool {
	if r == nil {
		return false
	}
	switch r.URL.Query().Get("poll") {
	case "1", "true", "tail":
		return true
	default:
		return candleLimit > 0 && candleLimit <= stateTailPollLimit
	}
}

func (d *DashboardServer) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tf := r.URL.Query().Get("tf")
	if tf == "" {
		tf = d.tradingTimeframe
	}
	reqStart := time.Now()
	defer func() {
		if elapsed := time.Since(reqStart); elapsed >= 50*time.Millisecond {
			log.Printf("[Dashboard] GET /api/state tf=%s completed in %v", tf, elapsed)
		}
	}()

	spec, err := ResolveTimeframe(tf)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":    "unavailable",
			"timeframe": tf,
		})
		return
	}

	endTimeMs := parseStateEndTime(r)
	candleLimit := parseCandleLimit(r, defaultStateCandleLimit, maxStateCandleLimit)
	tailPoll := isTailPollRequest(r, candleLimit)
	if tailPoll && (candleLimit <= 0 || candleLimit > stateTailPollLimit) {
		candleLimit = 5
	}
	navigatorsOnly := r.URL.Query().Get("navigators") == "1"
	if navigatorsOnly {
		candleLimit = 0
		tailPoll = false
	}
	if err := requestCtxErr(r.Context()); err != nil {
		return
	}
	state, err := d.buildMarketState(r.Context(), spec, parseRSXLookback(r), candleLimit, endTimeMs, tailPoll, navigatorsOnly)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	if errors.Is(err, errWarmingUp) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(warmingUpResponse{Status: "warming_up"})
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := requestCtxErr(r.Context()); err != nil {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		log.Printf("[Dashboard] encode state: %v", err)
	}
}

type thresholdsRequest struct {
	Long  int `json:"long"`
	Short int `json:"short"`
}

type thresholdsResponse struct {
	Long  int `json:"long"`
	Short int `json:"short"`
}

func (d *DashboardServer) handleThresholds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req thresholdsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	strategy.SetScoreThresholds(req.Long, req.Short)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(thresholdsResponse{
		Long:  strategy.LongScoreThreshold(),
		Short: strategy.ShortScoreThreshold(),
	})
}

func (d *DashboardServer) handleScoringMatrix(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req strategy.ScoringMatrix
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	strategy.SetScoringMatrix(req)

	if err := strategy.SaveMatrixConfig(req, strategy.MatrixConfigPath); err != nil {
		log.Printf("[API] Failed to save scoring matrix to %s: %v", strategy.MatrixConfigPath, err)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(strategy.GetScoringMatrix())
}

func (d *DashboardServer) handleIndicatorSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(strategy.GetRSXSettings())
		return
	case http.MethodPost:
		var req strategy.RSXSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		applied := strategy.ApplyRSXSettings(req)
		d.applyRSXSettingsToAnalysts()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(applied)
		return
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (d *DashboardServer) handleRiskSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(strategy.GetRiskSettings())
		return
	case http.MethodPost:
		var req strategy.RiskSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		strategy.UpdateRiskSettings(req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(strategy.GetRiskSettings())
		return
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type navigatorSettingsRequest struct {
	Navigators map[string]strategy.NavigatorUISettings `json:"navigators"`
}

func defaultLiveNavigatorPanes() map[string]strategy.NavigatorUISettings {
	return strategy.DefaultLiveNavigatorPanes()
}

// BindMaster wires live execution for navigator-driven MTF updates.
func (d *DashboardServer) BindMaster(m *strategy.MasterGeneral) {
	d.master = m
	d.syncMasterNavigatorPanes()
}

func (d *DashboardServer) syncMasterNavigatorPanes() {
	if d.master == nil {
		return
	}
	d.master.SetNavigatorPanes(d.getLiveNavigatorPanes())
}

func (d *DashboardServer) getLiveNavigatorPanes() map[string]strategy.NavigatorUISettings {
	d.liveNavMu.RLock()
	defer d.liveNavMu.RUnlock()
	if len(d.liveNavigators) == 0 {
		return defaultLiveNavigatorPanes()
	}
	out := make(map[string]strategy.NavigatorUISettings, len(d.liveNavigators))
	for k, v := range d.liveNavigators {
		out[k] = v
	}
	return out
}

func (d *DashboardServer) setLiveNavigatorPanes(panes map[string]strategy.NavigatorUISettings) {
	d.liveNavMu.Lock()
	defer d.liveNavMu.Unlock()
	if len(panes) == 0 {
		d.liveNavigators = defaultLiveNavigatorPanes()
		return
	}
	d.liveNavigators = make(map[string]strategy.NavigatorUISettings, len(panes))
	for k, v := range panes {
		d.liveNavigators[k] = v
	}
}

func (d *DashboardServer) handleNavigatorSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"navigators": d.getLiveNavigatorPanes()})
		return
	case http.MethodPost:
		var req navigatorSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		panes := strategy.ResolveBacktestNavigators(
			&strategy.BacktestRunSettings{Navigators: req.Navigators},
			req.Navigators,
			strategy.NavigatorUISettings{},
		)
		d.setLiveNavigatorPanes(panes)
		d.syncMasterNavigatorPanes()
		writeJSON(w, map[string]any{"navigators": d.getLiveNavigatorPanes()})
		return
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (d *DashboardServer) handleUIManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if d.uiRegistry == nil {
		http.Error(w, "ui manifest unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d.uiRegistry.Manifest())
}

func (d *DashboardServer) applyRSXSettingsToAnalysts() {
	settings := strategy.GetRSXSettings()
	for _, analyst := range d.analysts {
		if analyst != nil {
			analyst.UpdateRSXScanConfig(settings)
		}
	}
}

// BacktestRequest is the JSON payload for POST /api/backtest/run.
type BacktestRequest struct {
	Symbol     string                                  `json:"symbol"`
	Interval   string                                  `json:"interval"`
	StartDate  string                                  `json:"startDate"`
	EndDate    string                                  `json:"endDate"`
	Settings   *strategy.BacktestRunSettings           `json:"settings"`
	Navigator  strategy.NavigatorUISettings            `json:"navigator,omitempty"`
	Navigators map[string]strategy.NavigatorUISettings `json:"navigators,omitempty"`
	MtfOptions map[string]bool                         `json:"mtfOptions,omitempty"`
	SimOnly    bool                                    `json:"simOnly"`
}

// ChartPoint is one candle with full indicator values for the backtest chart.
type ChartPoint struct {
	Time            int64                           `json:"time"`
	Open            float64                         `json:"open"`
	High            float64                         `json:"high"`
	Low             float64                         `json:"low"`
	Close           float64                         `json:"close"`
	Volume          float64                         `json:"volume,omitempty"`
	Jurik           float64                         `json:"jurik,omitempty"`
	RSX             float64                         `json:"rsx"`
	RSXSignal       float64                         `json:"rsx_signal"`
	RsiPrice        float64                         `json:"rsiPrice,omitempty"`
	EmaRsi          float64                         `json:"emaRsi,omitempty"`
	RsiRsi          float64                         `json:"rsiRsi,omitempty"`
	RsiHl2          float64                         `json:"rsiHl2,omitempty"`
	RsiVolFast      float64                         `json:"rsiVolFast,omitempty"`
	RsiVolSlow      float64                         `json:"rsiVolSlow,omitempty"`
	MacdRsi         float64                         `json:"macdRsi,omitempty"`
	RsiAd           float64                         `json:"rsiAd,omitempty"`
	RsiHl2Vol       float64                         `json:"rsiHl2Vol,omitempty"`
	VolCrossMarker  string                          `json:"volCrossMarker,omitempty"`
	VolChanMid      float64                         `json:"volChanMid,omitempty"`
	VolChanUp       float64                         `json:"volChanUp,omitempty"`
	VolChanDn       float64                         `json:"volChanDn,omitempty"`
	PriceChanMid    float64                         `json:"priceChanMid,omitempty"`
	PriceChanUp     float64                         `json:"priceChanUp,omitempty"`
	PriceChanDn     float64                         `json:"priceChanDn,omitempty"`
	Color           string                          `json:"color,omitempty"`
	Marker          string                          `json:"marker,omitempty"`
	VolumeSpikeUp   bool                            `json:"volumeSpikeUp,omitempty"`
	VolumeSpikeDown bool                            `json:"volumeSpikeDown,omitempty"`
	WozduhUp        float64                         `json:"wozduh_up,omitempty"`
	WozduhDown      float64                         `json:"wozduh_down,omitempty"`
	LongScore       int                             `json:"longScore,omitempty"`
	ShortScore      int                             `json:"shortScore,omitempty"`
	RawAction       string                          `json:"rawAction,omitempty"`
	FinalAction     string                          `json:"finalAction,omitempty"`
	IsVetoed        bool                            `json:"isVetoed,omitempty"`
	VetoReason      string                          `json:"vetoReason,omitempty"`
	Factors         map[string]strategy.ScoreFactor `json:"factors,omitempty"`
}

// SimPoint is a slim chart point (indicators only, no OHLC) for simOnly backtest responses.
type SimPoint struct {
	Time            int64   `json:"time"`
	Jurik           float64 `json:"jurik,omitempty"`
	RSX             float64 `json:"rsx,omitempty"`
	RSXSignal       float64 `json:"rsxSignal,omitempty"`
	RsiVolFast      float64 `json:"rsiVolFast,omitempty"`
	RsiVolSlow      float64 `json:"rsiVolSlow,omitempty"`
	VolCrossMarker  string  `json:"volCrossMarker,omitempty"`
	Color           string  `json:"color,omitempty"`
	Marker          string  `json:"marker,omitempty"`
	VolumeSpikeUp   bool    `json:"volumeSpikeUp,omitempty"`
	VolumeSpikeDown bool    `json:"volumeSpikeDown,omitempty"`
}

// BacktestTrade is a single simulated trade in a backtest result.
type BacktestTrade struct {
	Time            int64    `json:"time"`
	EntryTime       int64    `json:"entryTime"`
	Side            string   `json:"side"`
	EntryPrice      float64  `json:"entryPrice"`
	ExitPrice       float64  `json:"exitPrice"`
	StopLossPrice   float64  `json:"stopLossPrice"`
	ExitReason      string   `json:"exitReason"`
	EntryReason     string   `json:"entryReason,omitempty"`
	FactorsSnapshot []string `json:"factorsSnapshot,omitempty"`
	StrategySource  string   `json:"strategySource,omitempty"`
	ActiveFactors   []string `json:"activeFactors,omitempty"`
	SignalKind      string   `json:"signalKind,omitempty"`
	EntryScore      float64  `json:"entryScore,omitempty"`
	PnL             float64  `json:"pnl"`
	Duration        string   `json:"duration"`
}

// EquityPoint is one point on the equity curve (time in Unix seconds).
type EquityPoint struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

// BacktestResult is returned after a backtest run completes.
type BacktestResult struct {
	TotalTrades    int                                    `json:"totalTrades"`
	WinRate        float64                                `json:"winRate"`
	NetProfit      float64                                `json:"netProfit"`
	ProfitFactor   float64                                `json:"profitFactor"`
	MaxDrawdown    float64                                `json:"maxDrawdown"`
	RecoveryFactor float64                                `json:"recoveryFactor"`
	Cancelled      bool                                   `json:"cancelled,omitempty"`
	Trades         []BacktestTrade                        `json:"trades"`
	EquityCurve    []EquityPoint                          `json:"equityCurve"`
	ChartData      []ChartPoint                           `json:"chartData"`
	SimData        []SimPoint                             `json:"simData,omitempty"`
	NavigatorData  strategy.NavigatorResultDTO            `json:"navigatorData"`
	NavigatorPrice strategy.NavigatorResultDTO            `json:"navigatorPrice"` // legacy alias for navigatorData
	Navigators     map[string]strategy.NavigatorResultDTO `json:"navigators,omitempty"`
	Annotations    []strategy.ChartAnnotation             `json:"annotations,omitempty"`
}

func truncateLogBody(b []byte, max int) string {
	if max <= 0 || len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "...(truncated)"
}

func (d *DashboardServer) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	removed := 0
	if d.htfProvider != nil {
		removed = d.htfProvider.ClearCache(true)
	}

	log.Printf("[Dashboard] cache cleared: htf entries=%d", removed)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Cache cleared"))
}

func (d *DashboardServer) handleBacktestRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BacktestRequest
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[CRITICAL] Failed to read BacktestRequest body: %v", err)
		http.Error(w, "Bad Request: cannot read body", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		log.Printf("[CRITICAL] JSON Decode Error: %v. Body was: %s", err, truncateLogBody(bodyBytes, 8192))
		http.Error(w, "Bad Request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := data.InitDB(); err != nil {
		log.Printf("[Backtest] history DB init failed: %v", err)
		http.Error(w, "history database unavailable", http.StatusServiceUnavailable)
		return
	}

	if d.rest == nil {
		http.Error(w, "exchange client unavailable", http.StatusServiceUnavailable)
		return
	}

	spec, err := ResolveBacktestInterval(req.Interval)
	if err != nil {
		log.Printf("[Backtest] bad interval %q: %v", req.Interval, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if spec.BinanceInterval == "" {
		log.Printf("[Backtest] interval %q resolved to %q but has no Binance mapping", req.Interval, spec.ID)
		http.Error(w, "interval not supported for backtest", http.StatusBadRequest)
		return
	}

	startMs, endMs, err := strategy.ParseBacktestDateRange(req.StartDate, req.EndDate)
	if err != nil {
		log.Printf("[Backtest] bad date range start=%q end=%q: %v", req.StartDate, req.EndDate, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	intervalMs, intervalErr := data.IntervalDurationMs(spec.BinanceInterval)
	if intervalErr == nil && intervalMs > 0 {
		expectedBars := (endMs - startMs) / intervalMs
		if expectedBars > maxBacktestBars {
			errMsg := fmt.Sprintf(
				"Слишком большой период. Ожидается ~%d свечей. Максимально разрешено %d. Уменьшите период дат или выберите старший таймфрейм.",
				expectedBars, maxBacktestBars,
			)
			log.Printf("[Backtest] %s: symbol=%s interval=%s start=%s end=%s",
				errMsg, req.Symbol, spec.BinanceInterval, req.StartDate, req.EndDate)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
	}

	symbol := req.Symbol
	if symbol == "" {
		symbol = d.symbol
	}
	if symbol == "" {
		symbol = "BTCUSDT"
	}

	log.Printf("[Backtest] run request: symbol=%s interval=%s start=%s end=%s",
		symbol, spec.BinanceInterval, req.StartDate, req.EndDate)

	effectiveStartMs := startMs
	candles, err := d.rest.FetchClosedRangePages(symbol, spec.BinanceInterval, effectiveStartMs, endMs)
	if err != nil {
		log.Printf("[Backtest] fetch history failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	minBars := strategy.BacktestMinBars()
	for padAttempt := 0; len(candles) < minBars && padAttempt < 4; padAttempt++ {
		paddedStart, ok := strategy.PadBacktestStartMs(spec.BinanceInterval, effectiveStartMs, endMs, len(candles))
		if !ok {
			break
		}
		log.Printf("[Backtest] padding start (attempt %d): have %d candles, need %d — extending start %s → %s",
			padAttempt+1, len(candles), minBars,
			time.UnixMilli(effectiveStartMs).UTC().Format("2006-01-02"),
			time.UnixMilli(paddedStart).UTC().Format("2006-01-02"))
		effectiveStartMs = paddedStart
		candles, err = d.rest.FetchClosedRangePages(symbol, spec.BinanceInterval, effectiveStartMs, endMs)
		if err != nil {
			log.Printf("[Backtest] fetch history failed after padding: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}

	if len(candles) < minBars {
		msg := fmt.Sprintf("not enough candles (%d) for backtest", len(candles))
		log.Printf("[Backtest] %s: symbol=%s interval=%s start=%s end=%s (effective start %s)",
			msg, symbol, spec.BinanceInterval, req.StartDate, req.EndDate,
			time.UnixMilli(effectiveStartMs).UTC().Format("2006-01-02"))
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if effectiveStartMs != startMs {
		log.Printf("[Backtest] used padded start %s (requested %s) — fetched %d candles",
			time.UnixMilli(effectiveStartMs).UTC().Format("2006-01-02"), req.StartDate, len(candles))
	}

	matrixSnapshot := strategy.ResolveBacktestMatrix(req.Settings)
	matrix := &matrixSnapshot
	navigators := strategy.ResolveBacktestNavigators(req.Settings, req.Navigators, req.Navigator)
	strategy.ApplyMtfOptionsToNavigators(navigators, req.MtfOptions)
	strategy.EnsureBacktestNavigatorsForMatrix(navigators, matrixSnapshot)

	if req.Settings != nil && req.Settings.Risk != nil {
		strategy.UpdateRiskSettings(*req.Settings.Risk)
	}

	log.Printf("[Backtest] parsed settings: RSX=%v WozduhCross=%v Trendlines=%v entrySources=%v navigatorPanes=%d",
		matrix.UseRSX, matrix.UseWozduhCross, matrix.UseTrendlines,
		strategy.ScoringMatrixEntrySourcesEnabledFor(*matrix), len(navigators))
	for pane, ui := range navigators {
		log.Printf("[Backtest] navigator[%s] enabled=%v source=%s useLong=%v longLen=%d",
			pane, ui.Enabled, ui.Source, ui.UseLong, ui.LongLen)
	}

	longTh, shortTh := strategy.ResolveBacktestThresholds(req.Settings)
	rsxSettings, hasRSX := strategy.ResolveBacktestRSXSettings(req.Settings)
	var rsxCfg *strategy.RSXSettings
	if hasRSX {
		rsxCfg = &rsxSettings
	}
	var wozduhPrefs map[string]bool
	if req.Settings != nil && len(req.Settings.WozduhSettings) > 0 {
		wozduhPrefs = req.Settings.WozduhSettings
	}
	log.Printf("[Backtest] thresholds: long=%d short=%d", longTh, shortTh)
	if hasRSX {
		log.Printf("[Backtest] RSX settings: length=%d lookback=%d pivot_radius=%d method=%s source=%s",
			rsxSettings.Length, rsxSettings.DivLookback, rsxSettings.PivotRadius, rsxSettings.DivMethod, rsxSettings.Source)
	}

	simOnly := req.SimOnly
	if req.Settings != nil && req.Settings.SimOnly {
		simOnly = true
	}
	skipNavigators := false
	if req.Settings != nil && req.Settings.SkipNavigators {
		skipNavigators = true
	}
	if simOnly {
		log.Printf("[Backtest] simOnly=true — wire response will omit OHLC chartData")
	}
	if skipNavigators {
		log.Printf("[Backtest] skipNavigators=true — navigator geometry bypassed")
	}

	ctx, endRun := d.backtestRuns.begin(r.Context())
	defer endRun()

	engine := strategy.NewBacktestEngine(strategy.BacktestConfig{
		Symbol:         symbol,
		Interval:       spec.BinanceInterval,
		EntryAnalyst:   d.signalAnalyst,
		FeeRate:        strategy.DefaultScalpFeeRate,
		SlippagePct:    strategy.ResolveBacktestSlippage(req.Settings),
		Matrix:         matrix,
		Navigator:      req.Navigator,
		Navigators:     navigators,
		HTF:            d.htfProvider,
		LongThreshold:  longTh,
		ShortThreshold: shortTh,
		RSXSettings:    rsxCfg,
		WozduhPrefs:    wozduhPrefs,
		SimOnly:        simOnly,
		SkipNavigators: skipNavigators,
	})
	runResult, err := engine.Run(ctx, candles)
	if err != nil {
		log.Printf("[Backtest] simulation failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if runResult.Cancelled {
		log.Printf("[Backtest] stopped early: trades=%d chartPoints=%d candles=%d/%d",
			runResult.TotalTrades, len(runResult.ChartData), len(runResult.ChartData), len(candles))
	} else {
		log.Printf("[Backtest] complete: trades=%d net=%.2f%% winRate=%.1f%% chartPoints=%d candles=%d",
			runResult.TotalTrades, runResult.NetProfit, runResult.WinRate, len(runResult.ChartData), len(candles))
	}

	w.Header().Set("Content-Type", "application/json")
	result := backtestResultFromStrategy(runResult)
	respBytes, err := json.Marshal(result)
	if err != nil {
		log.Printf("[ERROR] JSON Marshal failed: %v", err)
		http.Error(w, `{"error": "Failed to serialize response due to invalid float values (NaN/Inf)"}`, http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(respBytes); err != nil {
		log.Printf("[ERROR] backtest response write failed: %v", err)
	}
}

func (d *DashboardServer) handleBacktestStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stopped := false
	if d.backtestRuns != nil {
		stopped = d.backtestRuns.stop()
	}
	log.Printf("[Backtest] stop requested: stopped=%v", stopped)
	writeJSON(w, map[string]any{"stopped": stopped})
}

func backtestResultFromStrategy(run *strategy.BacktestRunResult) BacktestResult {
	if run == nil {
		return BacktestResult{}
	}

	trades := make([]BacktestTrade, len(run.Trades))
	for i, t := range run.Trades {
		trades[i] = BacktestTrade{
			Time:            t.Time,
			EntryTime:       t.EntryTime,
			Side:            t.Side,
			EntryPrice:      t.EntryPrice,
			ExitPrice:       t.ExitPrice,
			StopLossPrice:   t.StopLossPrice,
			ExitReason:      t.ExitReason,
			EntryReason:     t.EntryReason,
			FactorsSnapshot: append([]string(nil), t.FactorsSnapshot...),
			StrategySource:  t.StrategySource,
			ActiveFactors:   append([]string(nil), t.ActiveFactors...),
			SignalKind:      t.SignalKind,
			EntryScore:      t.EntryScore,
			PnL:             t.PnL,
			Duration:        t.Duration,
		}
	}

	equity := make([]EquityPoint, len(run.EquityCurve))
	for i, p := range run.EquityCurve {
		equity[i] = EquityPoint{Time: p.Time, Value: p.Value}
	}

	chartData := make([]ChartPoint, len(run.ChartData))
	for i, p := range run.ChartData {
		chartData[i] = ChartPoint{
			Time:            p.Time,
			Open:            p.Open,
			High:            p.High,
			Low:             p.Low,
			Close:           p.Close,
			Volume:          p.Volume,
			Jurik:           p.Jurik,
			RSX:             p.RSX,
			RSXSignal:       p.RSXSignal,
			RsiPrice:        p.RsiPrice,
			EmaRsi:          p.EmaRsi,
			RsiRsi:          p.RsiRsi,
			RsiHl2:          p.RsiHl2,
			RsiVolFast:      p.RsiVolFast,
			RsiVolSlow:      p.RsiVolSlow,
			MacdRsi:         p.MacdRsi,
			RsiAd:           p.RsiAd,
			RsiHl2Vol:       p.RsiHl2Vol,
			VolCrossMarker:  p.VolCrossMarker,
			VolChanMid:      p.VolChanMid,
			VolChanUp:       p.VolChanUp,
			VolChanDn:       p.VolChanDn,
			PriceChanMid:    p.PriceChanMid,
			PriceChanUp:     p.PriceChanUp,
			PriceChanDn:     p.PriceChanDn,
			Color:           p.Color,
			Marker:          p.Marker,
			VolumeSpikeUp:   p.VolumeSpikeUp,
			VolumeSpikeDown: p.VolumeSpikeDown,
			WozduhUp:        p.RsiVolFast,
			WozduhDown:      p.RsiVolSlow,
			LongScore:       p.LongScore,
			ShortScore:      p.ShortScore,
			RawAction:       p.RawAction,
			FinalAction:     p.FinalAction,
			IsVetoed:        p.IsVetoed,
			VetoReason:      p.VetoReason,
			Factors:         p.Factors,
		}
	}

	simData := make([]SimPoint, len(run.SimData))
	for i, p := range run.SimData {
		simData[i] = SimPoint{
			Time:            p.Time,
			Jurik:           p.Jurik,
			RSX:             p.RSX,
			RSXSignal:       p.RSXSignal,
			RsiVolFast:      p.RsiVolFast,
			RsiVolSlow:      p.RsiVolSlow,
			VolCrossMarker:  p.VolCrossMarker,
			Color:           p.Color,
			Marker:          p.Marker,
			VolumeSpikeUp:   p.VolumeSpikeUp,
			VolumeSpikeDown: p.VolumeSpikeDown,
		}
	}

	return BacktestResult{
		TotalTrades:    run.TotalTrades,
		WinRate:        run.WinRate,
		NetProfit:      run.NetProfit,
		ProfitFactor:   run.ProfitFactor,
		MaxDrawdown:    run.MaxDrawdown,
		RecoveryFactor: run.RecoveryFactor,
		Cancelled:      run.Cancelled,
		Trades:         trades,
		EquityCurve:    equity,
		ChartData:      chartData,
		SimData:        simData,
		NavigatorData:  run.NavigatorData,
		NavigatorPrice: run.NavigatorData,
		Navigators:     run.Navigators,
		Annotations:    run.Annotations,
	}
}

func (d *DashboardServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := requestCtxErr(r.Context()); err != nil {
		return
	}

	tf := r.URL.Query().Get("tf")
	endTimeMs, _ := strconv.ParseInt(r.URL.Query().Get("endTimeMs"), 10, 64)
	endTimeSec, _ := strconv.ParseInt(r.URL.Query().Get("endTime"), 10, 64)
	if tf == "" || (endTimeMs <= 0 && endTimeSec <= 0) {
		http.Error(w, "tf and endTime or endTimeMs required", http.StatusBadRequest)
		return
	}

	spec, err := ResolveTimeframe(tf)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := historyResponse{
		Status:    "ready",
		Timeframe: spec.ID,
		Trades:    d.sessionTradesForChart(),
	}

	rsxSettings := parseRSXSettingsFromRequest(r)
	_ = parseRSXLookback(r) // legacy query param; replay uses RSX settings only
	candleLimit := parseCandleLimit(r, historyFetchLimit, maxStateCandleLimit)
	columnar := r.URL.Query().Get("format") == "columnar"
	slotIDs := parseSlotsParam(r.URL.Query().Get("slots"))

	if columnar {
		d.writeColumnarHistory(w, r, spec, endTimeMs, endTimeSec, rsxSettings, candleLimit, slotIDs)
		return
	}

	if spec.Kind == TFRAMOnly {
		if err := requestCtxErr(r.Context()); err != nil {
			return
		}
		resolvedEndMs := endTimeMs
		if resolvedEndMs <= 0 {
			resolvedEndMs = historyEndTimeToMs(endTimeSec)
		}
		win, okWin := d.GetWindow(r.Context(), HistoryWindowQuery{
			Spec:        spec,
			EndTimeMs:   resolvedEndMs,
			CandleLimit: candleLimit,
		})
		if !okWin || len(win.Klines) == 0 {
			http.Error(w, "no historical data available", http.StatusServiceUnavailable)
			return
		}
		klines := win.Klines
		trimBars := historyWarmupTrim(len(klines), candleLimit, strategy.IndicatorWarmupBars)
		resp.Candles, resp.Oscillators, resp.Annotations = d.buildHistoryChartSeriesTrimmed(
			r.Context(), klines, trimBars, spec.ID, rsxSettings,
		)
		resp.HasMore = false
		if err := requestCtxErr(r.Context()); err != nil {
			return
		}
		writeJSON(w, resp)
		return
	}

	resolvedEndMs := endTimeMs
	if resolvedEndMs <= 0 {
		resolvedEndMs = historyEndTimeToMs(endTimeSec)
	}
	win, okWin := d.GetWindow(r.Context(), HistoryWindowQuery{
		Spec:        spec,
		EndTimeMs:   resolvedEndMs,
		CandleLimit: candleLimit,
	})
	if err := requestCtxErr(r.Context()); err != nil {
		return
	}
	if !okWin || len(win.Klines) == 0 {
		http.Error(w, "no historical data available", http.StatusServiceUnavailable)
		return
	}
	klines := win.Klines

	trimBars := historyWarmupTrim(len(klines), candleLimit, strategy.IndicatorWarmupBars)
	if err := requestCtxErr(r.Context()); err != nil {
		return
	}
	resp.Candles, resp.Oscillators, resp.Annotations = d.buildHistoryChartSeriesTrimmed(
		r.Context(),
		klines,
		trimBars,
		spec.BinanceInterval,
		rsxSettings,
	)
	if candleLimit > 0 && len(resp.Candles) > candleLimit {
		drop := len(resp.Candles) - candleLimit
		resp.Candles = resp.Candles[drop:]
		resp.Oscillators = resp.Oscillators[drop:]
		resp.Annotations = trimAnnotations(resp.Annotations, drop, klines)
	}
	if len(resp.Candles) == 0 {
		log.Printf("[Dashboard] history replay empty for %s %s (%d klines)", d.symbol, spec.BinanceInterval, len(klines))
		http.Error(w, "history replay empty", http.StatusServiceUnavailable)
		return
	}
	if len(resp.Candles) > 0 {
		resp.HasMore = win.HasMore
	}
	if err := requestCtxErr(r.Context()); err != nil {
		return
	}
	writeJSON(w, resp)
}

func parseHistoryChunkLimit(r *http.Request) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = historyFetchLimit
	}
	if limit > maxBacktestChunkLimit {
		limit = maxBacktestChunkLimit
	}
	return limit
}

func (d *DashboardServer) handleHistoryChunk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := requestCtxErr(r.Context()); err != nil {
		return
	}

	if err := data.InitDB(); err != nil {
		log.Printf("[HistoryChunk] DB init failed: %v", err)
		http.Error(w, "history database unavailable", http.StatusServiceUnavailable)
		return
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = r.URL.Query().Get("tf")
	}
	endTimeMs, _ := strconv.ParseInt(r.URL.Query().Get("endTime"), 10, 64)
	limit := parseHistoryChunkLimit(r)

	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = d.symbol
	}
	if symbol == "" {
		symbol = "BTCUSDT"
	}

	if interval == "" || endTimeMs <= 0 {
		http.Error(w, "interval and endTime required", http.StatusBadRequest)
		return
	}

	spec, err := ResolveTimeframe(interval)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if spec.Kind != TFBinanceREST || spec.BinanceInterval == "" {
		http.Error(w, "interval not supported for history chunk", http.StatusBadRequest)
		return
	}

	intervalMs, err := data.IntervalDurationMs(spec.BinanceInterval)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fetchEndMs := endTimeMs - intervalMs
	if fetchEndMs <= 0 {
		writeJSON(w, historyChunkResponse{ChartData: []ChartPoint{}, HasMore: false})
		return
	}

	fetchStartMs := fetchEndMs - intervalMs*int64(limit+strategy.IndicatorWarmupBars)
	if fetchStartMs < 0 {
		fetchStartMs = 0
	}

	rsxSettings := parseRSXSettingsFromRequest(r)
	_ = parseRSXLookback(r) // legacy query param; replay uses RSX settings only

	if err := data.InitDB(); err == nil {
		candles, loadErr := exchange.LoadContinuousContractFromDB(symbol, spec.BinanceInterval, fetchStartMs, fetchEndMs, limit+strategy.IndicatorWarmupBars)
		if loadErr == nil && len(candles) > 0 {
			klines := candlesToKlines(candles)
			wantBars := limit + strategy.IndicatorWarmupBars
			if wantBars > 0 && len(klines) > wantBars {
				klines = klines[len(klines)-wantBars:]
			}
			trim := historyWarmupTrim(len(klines), limit, strategy.IndicatorWarmupBars)
			if err := requestCtxErr(r.Context()); err != nil {
				return
			}
			chartCandles, oscillators, annotations := d.buildHistoryChartSeriesTrimmed(r.Context(), klines, trim, spec.BinanceInterval, rsxSettings)
			if limit > 0 && len(chartCandles) > limit {
				drop := len(chartCandles) - limit
				chartCandles = chartCandles[drop:]
				oscillators = oscillators[drop:]
				annotations = trimAnnotations(annotations, drop, klines)
			}
			chartData := chartPointsFromSeries(chartCandles, oscillators)
			hasMore := len(chartCandles) >= limit && fetchStartMs > 0
			if err := requestCtxErr(r.Context()); err != nil {
				return
			}
			writeJSON(w, historyChunkResponse{
				ChartData:   chartData,
				HasMore:     hasMore,
				Annotations: annotations,
			})
			return
		}
	} else {
		log.Printf("[HistoryChunk] DB init failed: %v", err)
	}

	log.Printf("[HistoryChunk] no SQLite data for %s %s [%d..%d]", symbol, spec.BinanceInterval, fetchStartMs, fetchEndMs)
	http.Error(w, "no historical data available", http.StatusServiceUnavailable)
}

func chartPointsFromSeries(candles []ChartCandle, oscillators []ChartOscillator) []ChartPoint {
	n := len(candles)
	if len(oscillators) < n {
		n = len(oscillators)
	}
	out := make([]ChartPoint, n)
	for i := 0; i < n; i++ {
		c := candles[i]
		o := oscillators[i]
		out[i] = ChartPoint{
			Time:            c.Time,
			Open:            c.Open,
			High:            c.High,
			Low:             c.Low,
			Close:           c.Close,
			Volume:          c.Volume,
			Jurik:           o.Jurik,
			RSX:             o.RSX,
			RSXSignal:       o.RSXSignal,
			RsiPrice:        o.RsiPrice,
			EmaRsi:          o.EmaRsi,
			RsiRsi:          o.RsiRsi,
			RsiHl2:          o.RsiHl2,
			RsiVolFast:      o.RsiVolFast,
			RsiVolSlow:      o.RsiVolSlow,
			MacdRsi:         o.MacdRsi,
			RsiAd:           o.RsiAd,
			RsiHl2Vol:       o.RsiHl2Vol,
			VolCrossMarker:  o.VolCrossMarker,
			VolChanMid:      o.VolChanMid,
			VolChanUp:       o.VolChanUp,
			VolChanDn:       o.VolChanDn,
			PriceChanMid:    o.PriceChanMid,
			PriceChanUp:     o.PriceChanUp,
			PriceChanDn:     o.PriceChanDn,
			Color:           o.Color,
			Marker:          o.Marker,
			VolumeSpikeUp:   o.VolumeSpikeUp,
			VolumeSpikeDown: o.VolumeSpikeDown,
			WozduhUp:        o.RsiVolFast,
			WozduhDown:      o.RsiVolSlow,
		}
	}
	return out
}

func (d *DashboardServer) buildMarketState(ctx context.Context, spec TimeframeSpec, rsxLookback int, candleLimit int, endTimeMs int64, tailPoll bool, navigatorsOnly bool) (*MarketState, error) {
	if err := requestCtxErr(ctx); err != nil {
		return nil, err
	}
	_ = rsxLookback
	_ = endTimeMs
	if navigatorsOnly {
		return d.buildNavigatorOnlyState(ctx, spec, candleLimit)
	}

	// Shot 9H: /api/state is a lightweight metadata container.
	// Charts come from columnar REST + RouteChartTick — never Falcon export / Candles+Oscillators.
	state := &MarketState{
		Status:           "ready",
		Symbol:           d.symbol,
		Timeframe:        spec.ID,
		TradingTimeframe: d.tradingTimeframe,
		UpdatedAt:        time.Now().Unix(),
		Trades:           d.sessionTradesForChart(),
		PaperTrading:     d.paperTrading,
		SandboxMode:      d.sandboxMode,
		Candles:          []ChartCandle{},
		Oscillators:      []ChartOscillator{},
		Factors:          map[string]strategy.ScoreFactor{},
	}
	if !tailPoll && candleLimit > stateTailPollLimit {
		if err := requestCtxErr(ctx); err != nil {
			return nil, err
		}
		klines := d.loadLiveKlinesFromRAM(spec, candleLimit)
		if len(klines) > 0 {
			trimBars := historyWarmupTrim(len(klines), candleLimit, strategy.IndicatorWarmupBars)
			binanceInterval := spec.BinanceInterval
			if spec.Kind == TFRAMOnly {
				binanceInterval = spec.ID
			}
			rsxVals, wozVals := strategy.ExtractDAGNavigatorSeries(klines, strategy.GetRSXSettings())
			state.Navigators = buildNavigatorsFromSeries(
				ctx, d.symbol, klines, rsxVals, wozVals, trimBars, binanceInterval, d.getLiveNavigatorPanes(), d.htfProvider,
			)
		}
	}

	if analyst, ok := d.analysts[spec.ID]; ok {
		d.enrichFromDAG(state, analyst)
	} else if analyst, ok := d.analysts[spec.BinanceInterval]; ok && spec.Kind == TFBinanceREST {
		d.enrichFromDAG(state, analyst)
	}
	return state, nil
}

func (d *DashboardServer) buildNavigatorOnlyState(ctx context.Context, spec TimeframeSpec, displayLimit int) (*MarketState, error) {
	if err := requestCtxErr(ctx); err != nil {
		return nil, err
	}
	if displayLimit <= 0 {
		displayLimit = defaultStateCandleLimit
	}
	klines := d.loadLiveKlinesFromRAM(spec, displayLimit)
	if len(klines) == 0 {
		return nil, errWarmingUp
	}
	trimBars := historyWarmupTrim(len(klines), displayLimit, strategy.IndicatorWarmupBars)
	binanceInterval := spec.BinanceInterval
	if spec.Kind == TFRAMOnly {
		binanceInterval = spec.ID
	}

	rsxVals, wozVals := strategy.ExtractDAGNavigatorSeries(klines, strategy.GetRSXSettings())
	if len(rsxVals) == 0 {
		return nil, errWarmingUp
	}
	if err := requestCtxErr(ctx); err != nil {
		return nil, err
	}
	return &MarketState{
		Status:           "ready",
		Symbol:           d.symbol,
		Timeframe:        spec.ID,
		TradingTimeframe: d.tradingTimeframe,
		UpdatedAt:        time.Now().Unix(),
		Candles:          []ChartCandle{},
		Oscillators:      []ChartOscillator{},
		Factors:          map[string]strategy.ScoreFactor{},
		Navigators: buildNavigatorsFromSeries(
			ctx, d.symbol, klines, rsxVals, wozVals, trimBars, binanceInterval, d.getLiveNavigatorPanes(), d.htfProvider,
		),
	}, nil
}

func (d *DashboardServer) loadLiveKlinesFromRAM(spec TimeframeSpec, limit int) []exchange.Kline {
	if limit <= 0 {
		limit = defaultStateCandleLimit
	}
	want := limit + strategy.IndicatorWarmupBars
	if want > strategy.LiveKlineRAMCap {
		want = strategy.LiveKlineRAMCap
	}
	if analyst, ok := d.analysts[spec.ID]; ok {
		return analyst.GetKlinesTail(want)
	}
	if analyst, ok := d.analysts[spec.BinanceInterval]; ok && spec.Kind == TFBinanceREST {
		return analyst.GetKlinesTail(want)
	}
	return nil
}

func (d *DashboardServer) loadKlines(ctx context.Context, spec TimeframeSpec, limit int, endTimeMs int64) []exchange.Kline {
	if err := requestCtxErr(ctx); err != nil {
		return nil
	}
	if limit <= 0 {
		limit = defaultStateCandleLimit
	}
	if spec.Kind == TFRAMOnly {
		want := limit + strategy.IndicatorWarmupBars
		return d.ramKlines(spec.ID, want)
	}
	if spec.Kind == TFBinanceREST && spec.BinanceInterval != "" {
		klines := d.loadRESTKlinesFromStore(ctx, spec, endTimeMs, limit, d.isHistoricalKlineEnd(endTimeMs, spec.BinanceInterval))
		klines = exchange.MergeKlineSeries(klines, d.analystKlines(spec), exchange.AuthoritySettled, exchange.AuthorityFinal)
		if len(klines) > 0 {
			return klines
		}
	}
	return d.analystKlines(spec)
}

func (d *DashboardServer) analystKlines(spec TimeframeSpec) []exchange.Kline {
	if a := d.analystForTimeframe(spec.ID); a != nil {
		return a.GetKlines()
	}
	if spec.BinanceInterval != "" {
		if a := d.analystForTimeframe(spec.BinanceInterval); a != nil {
			return a.GetKlines()
		}
	}
	return nil
}

// analystForTimeframe returns the live Marker for a chart/trading timeframe id (thread-safe reads via Marker APIs).
func (d *DashboardServer) analystForTimeframe(tf string) *strategy.Marker {
	if d == nil || tf == "" {
		return nil
	}
	if a, ok := d.analysts[tf]; ok {
		return a
	}
	return nil
}

func (d *DashboardServer) loadRESTKlinesFromStore(ctx context.Context, spec TimeframeSpec, endTimeMs int64, limit int, historical bool) []exchange.Kline {
	if err := requestCtxErr(ctx); err != nil {
		return nil
	}
	if spec.BinanceInterval == "" {
		return nil
	}
	if endTimeMs <= 0 {
		endTimeMs = time.Now().UnixMilli()
	}

	intervalMs, err := data.IntervalDurationMs(spec.BinanceInterval)
	if err != nil {
		log.Printf("[Dashboard] interval duration %s: %v", spec.BinanceInterval, err)
		return nil
	}

	wantBars := limit + strategy.IndicatorWarmupBars
	startTimeMs := endTimeMs - intervalMs*int64(wantBars)
	if startTimeMs < 0 {
		startTimeMs = 0
	}

	if err := data.InitDB(); err == nil {
		if err := requestCtxErr(ctx); err != nil {
			return nil
		}
		candles, loadErr := exchange.LoadContinuousContractFromDB(d.symbol, spec.BinanceInterval, startTimeMs, endTimeMs, wantBars)
		if loadErr == nil && len(candles) > 0 {
			if err := requestCtxErr(ctx); err != nil {
				return nil
			}
			klines := candlesToKlines(candles)
			if wantBars > 0 && len(klines) > wantBars {
				klines = klines[len(klines)-wantBars:]
			}
			return klines
		}
	} else {
		log.Printf("[Dashboard] SQLite init: %v", err)
	}

	return nil
}

// isHistoricalKlineEnd reports whether endTimeMs targets deep history (scroll-left via /api/history).
func (d *DashboardServer) isHistoricalKlineEnd(endTimeMs int64, interval string) bool {
	if endTimeMs <= 0 {
		return false
	}
	intervalMs, err := data.IntervalDurationMs(interval)
	if err != nil || intervalMs <= 0 {
		return endTimeMs < time.Now().UnixMilli()
	}
	return !d.isLiveKlineEnd(endTimeMs, intervalMs)
}

func (d *DashboardServer) isLiveKlineEnd(endTimeMs, intervalMs int64) bool {
	if endTimeMs <= 0 {
		return true
	}
	nowMs := time.Now().UnixMilli()
	return endTimeMs >= nowMs-intervalMs*3
}

// sqliteHasBarsBefore reports whether SQLite holds klines older than beforeOpenTimeMs.
func (d *DashboardServer) sqliteHasBarsBefore(interval string, beforeOpenTimeMs int64) bool {
	if interval == "" || beforeOpenTimeMs <= 0 {
		return false
	}
	if err := data.InitDB(); err != nil {
		return false
	}
	symbol := d.symbol
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	for _, sym := range []string{symbol, exchange.SpotStorageSymbol(symbol)} {
		bounds, err := data.QueryKlineCacheBounds(sym, interval)
		if err == nil && bounds.HasData && bounds.MinTime < beforeOpenTimeMs {
			return true
		}
	}
	return false
}

// liveHistoryHasMore is true when SQLite (or the requested window) indicates older bars exist.
func (d *DashboardServer) liveHistoryHasMore(symbol, interval string, klines []exchange.Kline, candleLimit int, endTimeMs int64) bool {
	if len(klines) == 0 || interval == "" {
		return false
	}
	oldestMs := klines[0].OpenTime

	if err := data.InitDB(); err == nil {
		bounds, berr := data.QueryKlineCacheBounds(symbol, interval)
		spotBounds, spotErr := data.QueryKlineCacheBounds(exchange.SpotStorageSymbol(symbol), interval)
		if (berr == nil && bounds.HasData && bounds.MinTime < oldestMs) ||
			(spotErr == nil && spotBounds.HasData && spotBounds.MinTime < oldestMs) {
			return true
		}
	}

	intervalMs, err := data.IntervalDurationMs(interval)
	if err != nil {
		return false
	}
	wantBars := candleLimit + strategy.IndicatorWarmupBars
	if len(klines) >= wantBars {
		startTimeMs := endTimeMs - intervalMs*int64(wantBars)
		if startTimeMs > 0 {
			return true
		}
	}
	return false
}

func dataCandlesToKlines(rows []data.Candle) []exchange.Kline {
	out := make([]exchange.Kline, len(rows))
	for i, c := range rows {
		out[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime:  c.OpenTime,
			CloseTime: c.CloseTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
		})
	}
	return out
}

func (d *DashboardServer) ramKlines(tfID string, maxBars int) []exchange.Kline {
	if analyst, ok := d.analysts[tfID]; ok {
		if maxBars <= 0 {
			maxBars = defaultStateCandleLimit
		}
		return analyst.GetKlinesTail(maxBars)
	}
	return nil
}

// setClientTimeframe records the resolved timeframe a WS client is subscribed to.
func (d *DashboardServer) setClientTimeframe(client *WSClient, tf string) {
	spec, err := ResolveTimeframe(tf)
	if err != nil {
		return
	}
	d.clientsMu.Lock()
	d.clientTF[client] = spec.ID
	d.clientsMu.Unlock()
}

func chartCandlesToKlines(candles []ChartCandle) []exchange.Kline {
	klines := make([]exchange.Kline, 0, len(candles))
	for _, c := range candles {
		if c.Close <= 0 {
			continue
		}
		klines = append(klines, exchange.Kline{
			OpenTime: c.Time * 1000,
			Open:     c.Open,
			High:     c.High,
			Low:      c.Low,
			Close:    c.Close,
			Volume:   0,
		})
	}
	return klines
}

func (d *DashboardServer) enrichFromDAG(state *MarketState, analyst *strategy.Marker) {
	if state == nil || analyst == nil {
		return
	}
	frame := analyst.DAGTickFrame()
	hdr := dagHeaderFromFrame(frame)
	applyDAGHeaderToMarketState(state, hdr)
	if frame != nil && d.projector != nil {
		state.Plots = d.projector.BuildTickJSON(frame)
	}

	// ChartOnly (and Live UI): no ScoreEngine / Veto / Fib / Layer2 regime for header.
	// Optional tip: SlotTotalScore → LongScore only when strategies are enabled.
	state.ShortScore = 0
	state.RawAction = ""
	state.FinalAction = ""
	state.IsVetoed = false
	state.VetoReason = ""
	state.Factors = map[string]strategy.ScoreFactor{}
	state.BrainStatus = ""
	state.AIStatus = ""
	state.FibZones = nil
	state.VolatilityRegime = ""

	if !strategy.EngineAllowsStrategies() {
		state.LongScore = 0
		return
	}
	if frame != nil {
		if total := frame.Get(core.SlotTotalScore); jsonSafeDivState(total) {
			state.LongScore = int(math.Round(total))
		}
	}
}

// dagHeader holds Jurik tip aliases from DAG (Wozduh lines live only in plots).
type dagHeader struct {
	Jurik     float64
	RSX       float64
	RSXSignal float64
}

func dagHeaderFromFrame(frame *core.TickFrame) dagHeader {
	if frame == nil {
		return dagHeader{}
	}
	h := dagHeader{}
	if v := frame.Get(core.SlotJurikRSX); jsonSafeDivState(v) {
		h.Jurik = v
		h.RSX = v
	}
	if v := frame.Get(core.SlotJurikSignal); jsonSafeDivState(v) {
		h.RSXSignal = v
	}
	return h
}

func applyDAGHeaderToMarketState(state *MarketState, h dagHeader) {
	if state == nil {
		return
	}
	state.Jurik = h.Jurik
}

func applyDAGHeaderToTick(p *tickPayload, h dagHeader) {
	if p == nil {
		return
	}
	p.Jurik = h.Jurik
	p.RSX = h.RSX
	p.RSXSignal = h.RSXSignal
	// Wire purge Stage 5: no Falcon Red/Green/Blue / ScoreEngine fields on tick.
	p.LongScore = 0
	p.ShortScore = 0
	p.RawAction = ""
	p.FinalAction = ""
	p.IsVetoed = false
	p.VetoReason = ""
	p.BrainStatus = ""
	p.AIStatus = ""
	p.VolatilityRegime = ""
	if p.Factors == nil {
		p.Factors = map[string]strategy.ScoreFactor{}
	}
}

func trimAnnotations(annotations []strategy.ChartAnnotation, trim int, klines []exchange.Kline) []strategy.ChartAnnotation {
	if trim <= 0 || len(annotations) == 0 {
		return annotations
	}
	if trim >= len(klines) {
		return nil
	}
	sorted := make([]exchange.Kline, len(klines))
	copy(sorted, klines)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].OpenTime < sorted[j].OpenTime
	})
	minTime := exchange.ChartTimeSec(sorted[trim].OpenTime)
	out := make([]strategy.ChartAnnotation, 0, len(annotations))
	for _, ann := range annotations {
		if ann.Time >= minTime {
			out = append(out, ann)
		}
	}
	return out
}

// parseStateEndTime reads optional endTime from GET /api/state (Unix seconds).
func parseStateEndTime(r *http.Request) int64 {
	endTimeStr := r.URL.Query().Get("endTime")
	if endTimeStr == "" {
		return 0
	}
	val, err := strconv.ParseInt(endTimeStr, 10, 64)
	if err != nil {
		return 0
	}
	return historyEndTimeToMs(val)
}

// historyEndTimeToMs converts chart endTime (Unix seconds) to Binance milliseconds.
func historyEndTimeToMs(endTimeSec int64) int64 {
	if endTimeSec <= 0 {
		return 0
	}
	return endTimeSec * 1000
}

// historyWarmupTrim returns 0 when SQLite/Binance returned fewer bars than requested,
// meaning we hit the absolute start of history and must not drop leading price candles.
func historyWarmupTrim(gotBars, requestedBars, warmupTrim int) int {
	if warmupTrim <= 0 {
		return 0
	}
	if gotBars < requestedBars+warmupTrim {
		return 0
	}
	return warmupTrim
}

func buildNavigatorsFromSeries(
	ctx context.Context,
	symbol string,
	klines []exchange.Kline,
	rsxVals, wozVals []float64,
	trimBars int,
	interval string,
	panes map[string]strategy.NavigatorUISettings,
	htf *exchange.HTFProvider,
) map[string]strategy.NavigatorResultDTO {
	if err := requestCtxErr(ctx); err != nil {
		return nil
	}
	if len(klines) == 0 || len(rsxVals) == 0 || len(panes) == 0 {
		return nil
	}

	navKlines := klines
	navRSX := rsxVals
	navWoz := wozVals
	if trimBars > 0 && len(navKlines) > trimBars {
		navKlines = navKlines[trimBars:]
		if len(navRSX) > trimBars {
			navRSX = navRSX[trimBars:]
		}
		if len(navWoz) > trimBars {
			navWoz = navWoz[trimBars:]
		}
	}
	n := len(navKlines)
	if len(navRSX) < n {
		n = len(navRSX)
	}
	if len(navWoz) < n {
		n = len(navWoz)
	}
	if n <= 0 {
		return nil
	}
	navKlines = navKlines[len(navKlines)-n:]
	navRSX = navRSX[len(navRSX)-n:]
	navWoz = navWoz[len(navWoz)-n:]

	return strategy.BuildAllNavigators(panes, symbol, navKlines, navRSX, navWoz, interval, htf)
}

func candlesToKlines(candles []exchange.Candle) []exchange.Kline {
	klines := make([]exchange.Kline, len(candles))
	for i, c := range candles {
		klines[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime:  c.OpenTime,
			CloseTime: c.CloseTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
		})
	}
	return klines
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func (d *DashboardServer) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Dashboard] ws upgrade: %v", err)
		return
	}

	client := &WSClient{Conn: conn}

	d.clientsMu.Lock()
	d.clients[client] = true
	d.clientTF[client] = d.tradingTimeframe
	d.clientsMu.Unlock()

	defer func() {
		d.clientsMu.Lock()
		delete(d.clients, client)
		delete(d.clientTF, client)
		d.clientsMu.Unlock()
		_ = client.Close()
	}()

	if err := client.WriteJSON(map[string]string{
		"type":    "welcome",
		"message": "dashboard websocket ready",
	}); err != nil {
		log.Printf("[Dashboard] ws welcome: %v", err)
		return
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg struct {
			Type string `json:"type"`
			TF   string `json:"tf"`
		}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}
		if msg.Type == "subscribe" && msg.TF != "" {
			d.setClientTimeframe(client, msg.TF)
		}
	}
}
