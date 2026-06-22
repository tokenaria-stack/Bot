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
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"trading_bot/data"
	"trading_bot/domain"
	"trading_bot/exchange"
	"trading_bot/indicators"
	"trading_bot/strategy"
)

const (
	defaultTimeframe      = "1m"
	maxCandlesInState     = 500
	historyFetchLimit     = 1000
	indicatorWarmupBars   = 100
	orderFlowWarmupBars   = 0
	binanceMaxKlinesLimit = 1000
	defaultStaticDir      = "web"
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
	analysts   map[string]*strategy.ChiefAnalyst
	rest       *exchange.BinanceExchange
	orderFlow  *domain.OrderFlowStore
	symbol     string
	staticDir  string
	upgrader   websocket.Upgrader
	clients    map[*WSClient]bool
	clientTF   map[*WSClient]string
	clientsMu  sync.Mutex
	tradesMu   sync.RWMutex
	trades     []ChartTrade
	paperTrading bool
	sandboxMode  bool
	entryRisk    *strategy.RiskManager
}

// MarketState is the JSON payload for GET /api/state.
type MarketState struct {
	Status           string              `json:"status,omitempty"`
	Symbol           string              `json:"symbol"`
	Timeframe        string              `json:"timeframe"`
	UpdatedAt        int64               `json:"updatedAt"`
	VolatilityRegime string              `json:"volatilityRegime"`
	Jurik            float64             `json:"jurik"`
	RedLine          float64             `json:"redLine"`
	GreenLine        float64             `json:"greenLine"`
	LongScore        int                 `json:"longScore"`
	ShortScore       int                 `json:"shortScore"`
	BrainStatus      string              `json:"brainStatus"`
	AIStatus         string              `json:"aiStatus"`
	TickBufferLen    int                 `json:"tickBufferLen,omitempty"`
	Candles          []ChartCandle       `json:"candles"`
	Oscillators      []ChartOscillator   `json:"oscillators"`
	FibZones         []ChartFibZone      `json:"fibZones"`
	Trades           []ChartTrade        `json:"trades,omitempty"`
	PaperTrading     bool                `json:"paperTrading,omitempty"`
	SandboxMode      bool                `json:"sandboxMode,omitempty"`
}

// ChartTrade is a virtual or live trade marker for the price chart (time in Unix seconds).
type ChartTrade struct {
	Time   int64   `json:"time"`
	Side   string  `json:"side"`
	Price  float64 `json:"price"`
	Reason string  `json:"reason,omitempty"`
	Kind   string  `json:"kind,omitempty"` // "entry" or "exit"
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
	VolumeSpikeUp   bool    `json:"volumeSpikeUp"`
	VolumeSpikeDown bool    `json:"volumeSpikeDown"`
}

type warmingUpResponse struct {
	Status string `json:"status"`
}

type wsEnvelope struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

type tickPayload struct {
	Timeframe   string  `json:"timeframe,omitempty"`
	Time        int64   `json:"time"`
	Open        float64 `json:"open"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Close       float64 `json:"close"`
	Volume      float64 `json:"volume,omitempty"`
	Jurik       float64 `json:"jurik"`
	RSXColor    string  `json:"rsxColor,omitempty"`
	RSXMarker   string  `json:"rsxMarker,omitempty"`
	RSX         float64 `json:"rsx,omitempty"`
	RSXSignal   float64 `json:"rsx_signal,omitempty"`
	RedLine     float64 `json:"redLine"`
	GreenLine   float64 `json:"greenLine"`
	BlueLine    float64 `json:"blueLine"`
	LongScore   int     `json:"longScore"`
	ShortScore  int     `json:"shortScore"`
	BrainStatus string  `json:"brainStatus"`
	AIStatus    string  `json:"aiStatus"`
}

type markerPayload struct {
	Time   int64   `json:"time"`
	Side   string  `json:"side"`
	Price  float64 `json:"price"`
	Reason string  `json:"reason,omitempty"`
	Kind   string  `json:"kind,omitempty"`
}

type historyChunkResponse struct {
	ChartData []ChartPoint `json:"chartData"`
	HasMore   bool         `json:"hasMore"`
}

type historyResponse struct {
	Status      string            `json:"status"`
	Timeframe   string            `json:"timeframe"`
	Candles     []ChartCandle     `json:"candles"`
	Oscillators []ChartOscillator `json:"oscillators"`
	Trades      []ChartTrade      `json:"trades,omitempty"`
	HasMore     bool              `json:"hasMore"`
}

// NewDashboardServer creates a dashboard server bound to ChiefAnalyst instances.
func NewDashboardServer(
	analysts map[string]*strategy.ChiefAnalyst,
	rest *exchange.BinanceExchange,
	symbol string,
	orderFlow *domain.OrderFlowStore,
	entryRisk *strategy.RiskManager,
	paperTrading bool,
	sandboxMode bool,
) *DashboardServer {
	return &DashboardServer{
		analysts:     analysts,
		rest:         rest,
		orderFlow:    orderFlow,
		symbol:       exchange.NormalizeFuturesSymbol(symbol),
		staticDir:    defaultStaticDir,
		clients:      make(map[*WSClient]bool),
		clientTF:     make(map[*WSClient]string),
		paperTrading: paperTrading,
		sandboxMode:  sandboxMode,
		entryRisk:    entryRisk,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Start listens on port and serves static assets, /api/state, and /ws.
func (d *DashboardServer) Start(port string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/state", d.handleState)
	mux.HandleFunc("/api/history", d.handleHistory)
	mux.HandleFunc("/api/history/chunk", d.handleHistoryChunk)
	mux.HandleFunc("/api/settings/thresholds", d.handleThresholds)
	mux.HandleFunc("/api/settings/matrix", d.handleScoringMatrix)
	mux.HandleFunc("/api/settings/indicators", d.handleIndicatorSettings)
	mux.HandleFunc("/api/settings/risk", d.handleRiskSettings)
	mux.HandleFunc("/api/backtest/run", d.handleBacktestRun)
	mux.HandleFunc("/ws", d.handleWS)

	webRoot, err := filepath.Abs(d.staticDir)
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServer(http.Dir(webRoot)))

	log.Printf("[Dashboard] listening on http://localhost%s", port)
	return http.ListenAndServe(port, mux)
}

// ChartCandleFromDomain converts a domain candle; OpenTime must be milliseconds.
// Returns false when OHLC is zero, missing, or non-finite.
func ChartCandleFromDomain(c domain.Candle) (ChartCandle, bool) {
	if !validOHLC(c.Open, c.High, c.Low, c.Close) {
		return ChartCandle{}, false
	}
	return ChartCandle{
		Time:   c.OpenTime / 1000,
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

// BroadcastTick pushes a live candle + oscillator + brain telemetry update to all dashboard clients.
func (d *DashboardServer) BroadcastTick(
	candle domain.Candle,
	jurik, rsxSignal, redLine, greenLine, blueLine float64,
	rsxColor string,
	longScore, shortScore int,
	brainStatus, aiStatus string,
) {
	chart, ok := ChartCandleFromDomain(candle)
	if !ok {
		return
	}
	d.broadcast(wsEnvelope{
		Type: "tick",
		Data: tickPayload{
			Timeframe:   "1m",
			Time:        chart.Time,
			Open:        chart.Open,
			High:        chart.High,
			Low:         chart.Low,
			Close:       chart.Close,
			Volume:      chart.Volume,
			Jurik:       jurik,
			RSX:         jurik,
			RSXSignal:   rsxSignal,
			RSXColor:    rsxColor,
			RedLine:     redLine,
			GreenLine:   greenLine,
			BlueLine:    blueLine,
			LongScore:   longScore,
			ShortScore:  shortScore,
			BrainStatus: brainStatus,
			AIStatus:    aiStatus,
		},
	})
}

// BroadcastPriceBar pushes a live OHLCV update for any standard timeframe.
func (d *DashboardServer) BroadcastPriceBar(timeframe string, candle domain.Candle) {
	chart, ok := ChartCandleFromDomain(candle)
	if !ok {
		return
	}
	d.broadcast(wsEnvelope{
		Type: "tick",
		Data: tickPayload{
			Timeframe: timeframe,
			Time:      chart.Time,
			Open:      chart.Open,
			High:      chart.High,
			Low:       chart.Low,
			Close:     chart.Close,
			Volume:    chart.Volume,
		},
	})
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

func (d *DashboardServer) broadcast(msg wsEnvelope) {
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Dashboard] broadcast marshal: %v", err)
		return
	}

	d.clientsMu.Lock()
	snapshot := make([]*WSClient, 0, len(d.clients))
	for client := range d.clients {
		snapshot = append(snapshot, client)
	}
	d.clientsMu.Unlock()

	var dead []*WSClient
	for _, client := range snapshot {
		if err := client.WriteMessage(websocket.TextMessage, payload); err != nil {
			_ = client.Close()
			dead = append(dead, client)
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

func parseRSXLookback(_ *http.Request) int {
	return strategy.GetRSXSettings().DivLookback
}

func (d *DashboardServer) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tf := r.URL.Query().Get("tf")
	if tf == "" {
		tf = defaultTimeframe
	}
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

	state, err := d.buildMarketState(spec, parseRSXLookback(r))
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

func (d *DashboardServer) applyRSXSettingsToAnalysts() {
	for _, analyst := range d.analysts {
		if analyst != nil {
			analyst.ReapplyRSXSettings()
		}
	}
}

// BacktestRequest is the JSON payload for POST /api/backtest/run.
type BacktestRequest struct {
	Symbol     string                                `json:"symbol"`
	Interval   string                                `json:"interval"`
	StartDate  string                                `json:"startDate"`
	EndDate    string                                `json:"endDate"`
	Settings   *strategy.BacktestRunSettings         `json:"settings"`
	Navigator  strategy.NavigatorUISettings          `json:"navigator,omitempty"`
	Navigators map[string]strategy.NavigatorUISettings `json:"navigators,omitempty"`
}

// ChartPoint is one candle with full indicator values for the backtest chart.
type ChartPoint struct {
	Time            int64   `json:"time"`
	Open            float64 `json:"open"`
	High            float64 `json:"high"`
	Low             float64 `json:"low"`
	Close           float64 `json:"close"`
	Volume          float64 `json:"volume,omitempty"`
	Jurik           float64 `json:"jurik,omitempty"`
	RSX             float64 `json:"rsx"`
	RSXSignal       float64 `json:"rsx_signal"`
	RsiPrice        float64 `json:"rsiPrice,omitempty"`
	EmaRsi          float64 `json:"emaRsi,omitempty"`
	RsiRsi          float64 `json:"rsiRsi,omitempty"`
	RsiHl2          float64 `json:"rsiHl2,omitempty"`
	RsiVolFast      float64 `json:"rsiVolFast,omitempty"`
	RsiVolSlow      float64 `json:"rsiVolSlow,omitempty"`
	MacdRsi         float64 `json:"macdRsi,omitempty"`
	RsiAd           float64 `json:"rsiAd,omitempty"`
	RsiHl2Vol       float64 `json:"rsiHl2Vol,omitempty"`
	VolCrossMarker  string  `json:"volCrossMarker,omitempty"`
	VolChanMid      float64 `json:"volChanMid,omitempty"`
	VolChanUp       float64 `json:"volChanUp,omitempty"`
	VolChanDn       float64 `json:"volChanDn,omitempty"`
	PriceChanMid    float64 `json:"priceChanMid,omitempty"`
	PriceChanUp     float64 `json:"priceChanUp,omitempty"`
	PriceChanDn     float64 `json:"priceChanDn,omitempty"`
	Color           string  `json:"color,omitempty"`
	Marker          string  `json:"marker,omitempty"`
	VolumeSpikeUp   bool    `json:"volumeSpikeUp,omitempty"`
	VolumeSpikeDown bool    `json:"volumeSpikeDown,omitempty"`
	WozduhUp        float64 `json:"wozduh_up,omitempty"`
	WozduhDown      float64 `json:"wozduh_down,omitempty"`
}

// BacktestTrade is a single simulated trade in a backtest result.
type BacktestTrade struct {
	Time          int64   `json:"time"`
	EntryTime     int64   `json:"entryTime"`
	Side          string  `json:"side"`
	EntryPrice    float64 `json:"entryPrice"`
	ExitPrice     float64 `json:"exitPrice"`
	StopLossPrice float64 `json:"stopLossPrice"`
	ExitReason    string  `json:"exitReason"`
	PnL           float64 `json:"pnl"`
	Duration      string  `json:"duration"`
}

// EquityPoint is one point on the equity curve (time in Unix seconds).
type EquityPoint struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

// BacktestResult is returned after a backtest run completes.
type BacktestResult struct {
	TotalTrades    int             `json:"totalTrades"`
	WinRate        float64         `json:"winRate"`
	NetProfit      float64         `json:"netProfit"`
	ProfitFactor   float64         `json:"profitFactor"`
	MaxDrawdown    float64         `json:"maxDrawdown"`
	RecoveryFactor float64         `json:"recoveryFactor"`
	Trades         []BacktestTrade `json:"trades"`
	EquityCurve    []EquityPoint   `json:"equityCurve"`
	ChartData      []ChartPoint                           `json:"chartData"`
	NavigatorData  strategy.NavigatorResultDTO            `json:"navigatorData"`
	NavigatorPrice strategy.NavigatorResultDTO            `json:"navigatorPrice"` // legacy alias for navigatorData
	Navigators     map[string]strategy.NavigatorResultDTO `json:"navigators,omitempty"`
}

func truncateLogBody(b []byte, max int) string {
	if max <= 0 || len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "...(truncated)"
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
	candles, err := d.rest.FetchHistoricalKlines(symbol, spec.BinanceInterval, effectiveStartMs, endMs)
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
		candles, err = d.rest.FetchHistoricalKlines(symbol, spec.BinanceInterval, effectiveStartMs, endMs)
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

	engine := strategy.NewBacktestEngine(strategy.BacktestConfig{
		Symbol:     symbol,
		Interval:   spec.BinanceInterval,
		EntryRisk:  d.entryRisk,
		FeeRate:    strategy.DefaultScalpFeeRate,
		Matrix:     matrix,
		Navigator:  req.Navigator,
		Navigators: navigators,
	})
	runResult, err := engine.Run(candles)
	if err != nil {
		log.Printf("[Backtest] simulation failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[Backtest] complete: trades=%d net=%.2f%% winRate=%.1f%% chartPoints=%d candles=%d",
		runResult.TotalTrades, runResult.NetProfit, runResult.WinRate, len(runResult.ChartData), len(candles))

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

func backtestResultFromStrategy(run *strategy.BacktestRunResult) BacktestResult {
	if run == nil {
		return BacktestResult{}
	}

	trades := make([]BacktestTrade, len(run.Trades))
	for i, t := range run.Trades {
		trades[i] = BacktestTrade{
			Time:          t.Time,
			EntryTime:     t.EntryTime,
			Side:          t.Side,
			EntryPrice:    t.EntryPrice,
			ExitPrice:     t.ExitPrice,
			StopLossPrice: t.StopLossPrice,
			ExitReason:    t.ExitReason,
			PnL:           t.PnL,
			Duration:      t.Duration,
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
			WozduhUp:        p.WozduhUp,
			WozduhDown:      p.WozduhDown,
		}
	}

	return BacktestResult{
		TotalTrades:    run.TotalTrades,
		WinRate:        run.WinRate,
		NetProfit:      run.NetProfit,
		ProfitFactor:   run.ProfitFactor,
		MaxDrawdown:    run.MaxDrawdown,
		RecoveryFactor: run.RecoveryFactor,
		Trades:         trades,
		EquityCurve:    equity,
		ChartData:      chartData,
		NavigatorData:  run.NavigatorData,
		NavigatorPrice: run.NavigatorData,
		Navigators:     run.Navigators,
	}
}

func (d *DashboardServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tf := r.URL.Query().Get("tf")
	endTimeSec, _ := strconv.ParseInt(r.URL.Query().Get("endTime"), 10, 64)
	if tf == "" || endTimeSec <= 0 {
		http.Error(w, "tf and endTime required", http.StatusBadRequest)
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
		Trades:    d.sessionTrades(),
	}

	rsxLookback := parseRSXLookback(r)

	if IsOrderFlowTimeframe(spec) {
		fetchLimit := historyFetchLimit
		klines := d.loadOrderFlowKlines(spec, endTimeSec, fetchLimit)
		resp.Candles, resp.Oscillators = buildChartSeriesTrimmed(klines, orderFlowWarmupBars, rsxLookback)
		resp.HasMore = len(klines) >= historyFetchLimit
		writeJSON(w, resp)
		return
	}

	if spec.Kind == TFRAMOnly {
		klines := d.ramKlines(spec.ID)
		resp.Candles, resp.Oscillators = buildChartSeriesTrimmed(klines, indicatorWarmupBars, rsxLookback)
		resp.HasMore = false
		writeJSON(w, resp)
		return
	}

	if d.rest == nil {
		http.Error(w, "REST client unavailable", http.StatusServiceUnavailable)
		return
	}

	endTimeMs := historyEndTimeToMs(endTimeSec)
	fetchLimit := historyFetchLimit + indicatorWarmupBars
	if fetchLimit > binanceMaxKlinesLimit {
		fetchLimit = binanceMaxKlinesLimit
	}

	candles, err := d.rest.GetKlinesBefore(d.symbol, spec.BinanceInterval, fetchLimit, endTimeMs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	klines := candlesToKlines(candles)
	resp.Candles, resp.Oscillators = buildChartSeriesTrimmed(klines, indicatorWarmupBars, rsxLookback)
	resp.HasMore = len(klines) >= fetchLimit
	writeJSON(w, resp)
}

func (d *DashboardServer) handleHistoryChunk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = historyFetchLimit
	}
	if limit > binanceMaxKlinesLimit {
		limit = binanceMaxKlinesLimit
	}

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

	if d.rest == nil {
		http.Error(w, "REST client unavailable", http.StatusServiceUnavailable)
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

	fetchStartMs := fetchEndMs - intervalMs*int64(limit+indicatorWarmupBars)
	if fetchStartMs < 0 {
		fetchStartMs = 0
	}

	candles, err := d.rest.FetchHistoricalKlines(symbol, spec.BinanceInterval, fetchStartMs, fetchEndMs)
	if err != nil {
		log.Printf("[HistoryChunk] fetch failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	klines := candlesToKlines(candles)
	rsxLookback := parseRSXLookback(r)
	chartCandles, oscillators := buildChartSeriesTrimmed(klines, indicatorWarmupBars, rsxLookback)
	chartData := chartPointsFromSeries(chartCandles, oscillators)

	hasMore := len(chartCandles) >= limit && fetchStartMs > 0
	writeJSON(w, historyChunkResponse{
		ChartData: chartData,
		HasMore:   hasMore,
	})
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

func (d *DashboardServer) buildMarketState(spec TimeframeSpec, rsxLookback int) (*MarketState, error) {
	klines := d.loadKlines(spec)
	if (spec.Kind == TFRAMOnly || IsOrderFlowTimeframe(spec)) && len(klines) == 0 {
		state := &MarketState{
			Status:       "ready",
			Symbol:       d.symbol,
			Timeframe:    spec.ID,
			UpdatedAt:    time.Now().Unix(),
			Trades:       d.sessionTrades(),
			PaperTrading: d.paperTrading,
			SandboxMode:  d.sandboxMode,
		}
		if IsOrderFlowTimeframe(spec) && d.orderFlow != nil && d.orderFlow.Ticks != nil {
			state.TickBufferLen = d.orderFlow.Ticks.Len()
		}
		return state, nil
	}
	if len(klines) == 0 {
		return nil, errWarmingUp
	}

	windowSize := maxCandlesInState + indicatorWarmupBars
	trimBars := indicatorWarmupBars
	if IsOrderFlowTimeframe(spec) {
		windowSize = maxCandlesInState
		trimBars = orderFlowWarmupBars
	}
	if len(klines) > windowSize {
		klines = klines[len(klines)-windowSize:]
	}

	candles, oscillators := buildChartSeriesTrimmed(klines, trimBars, rsxLookback)

	state := &MarketState{
		Status:       "ready",
		Symbol:       d.symbol,
		Timeframe:    spec.ID,
		UpdatedAt:    time.Now().Unix(),
		Candles:      candles,
		Oscillators:  oscillators,
		Trades:       d.sessionTrades(),
		PaperTrading: d.paperTrading,
		SandboxMode:  d.sandboxMode,
	}
	if IsOrderFlowTimeframe(spec) && d.orderFlow != nil && d.orderFlow.Ticks != nil {
		state.TickBufferLen = d.orderFlow.Ticks.Len()
	}

	if analyst, ok := d.analysts[spec.ID]; ok {
		d.enrichFromAnalyst(state, analyst, klines)
	} else if analyst, ok := d.analysts[spec.BinanceInterval]; ok && spec.Kind == TFBinanceREST {
		d.enrichFromAnalyst(state, analyst, klines)
	}

	return state, nil
}

func (d *DashboardServer) loadKlines(spec TimeframeSpec) []exchange.Kline {
	if IsOrderFlowTimeframe(spec) {
		return d.loadOrderFlowKlines(spec, 0, maxCandlesInState)
	}
	if spec.Kind == TFRAMOnly {
		return d.ramKlines(spec.ID)
	}

	if analyst, ok := d.analysts[spec.ID]; ok {
		return analyst.GetKlines()
	}
	if analyst, ok := d.analysts[spec.BinanceInterval]; ok {
		return analyst.GetKlines()
	}
	if d.rest == nil {
		return nil
	}

	candles, err := d.rest.GetKlines(d.symbol, spec.BinanceInterval, maxCandlesInState+indicatorWarmupBars)
	if err != nil {
		log.Printf("[Dashboard] REST klines %s: %v", spec.BinanceInterval, err)
		return nil
	}
	return candlesToKlines(candles)
}

func (d *DashboardServer) ramKlines(tfID string) []exchange.Kline {
	if analyst, ok := d.analysts[tfID]; ok {
		return analyst.GetKlines()
	}
	return nil
}

func (d *DashboardServer) loadOrderFlowKlines(spec TimeframeSpec, endTimeSec int64, maxCandles int) []exchange.Kline {
	if d.orderFlow == nil || d.orderFlow.Ticks == nil {
		return nil
	}

	var trades []domain.AggTrade
	if endTimeSec > 0 {
		trades = d.orderFlow.Ticks.Before(historyEndTimeToMs(endTimeSec))
	} else {
		trades = d.orderFlow.Ticks.All()
	}

	chartCandles := SynthesizeMicroCandles(trades, spec.ID)
	if len(chartCandles) == 0 {
		return nil
	}
	if len(chartCandles) > maxCandles {
		chartCandles = chartCandles[len(chartCandles)-maxCandles:]
	}
	return chartCandlesToKlines(chartCandles)
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

func (d *DashboardServer) enrichFromAnalyst(state *MarketState, analyst *strategy.ChiefAnalyst, klines []exchange.Kline) {
	if report, err := analyst.GenerateMarketReport(); err == nil {
		state.VolatilityRegime = string(report.Volatility.Regime)
		state.Jurik = report.JurikValue
		state.RedLine = report.Falcon.RedLine
		state.GreenLine = report.Falcon.GreenLine
		state.FibZones = chartFibZonesFromReport(report.FibZones)

		telemetry := strategy.EvaluateScalpSignal(context.Background(), *report, strategy.DefaultScalpFeeRate, nil)
		state.LongScore = telemetry.LongScore
		state.ShortScore = telemetry.ShortScore
		state.BrainStatus = strategy.TelemetryBrainStatus(telemetry, *report, d.entryRisk)
		state.AIStatus = strategy.TelemetryAIStatus(context.Background(), *report, nil)
		return
	}

	if len(klines) == 0 {
		return
	}
	falcon := strategy.NewFalconEngine()
	last := klines[len(klines)-1]
	sig := falcon.Evaluate(last.High, last.Low, last.Close, last.Volume)
	state.Jurik = sig.JurikRSX
	state.RedLine = sig.RedLine
	state.GreenLine = sig.GreenLine
}

func buildChartSeries(klines []exchange.Kline, rsxLookback int) ([]ChartCandle, []ChartOscillator) {
	sorted := make([]exchange.Kline, len(klines))
	copy(sorted, klines)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].OpenTime < sorted[j].OpenTime
	})

	candles := make([]ChartCandle, 0, len(sorted))
	oscillators := make([]ChartOscillator, 0, len(sorted))
	falcon := strategy.NewFalconEngine()
	var prevBlue float64

	type evaluatedBar struct {
		k   exchange.Kline
		sig strategy.FalconSignals
	}
	evaluated := make([]evaluatedBar, 0, len(sorted))
	rsxValues := make([]float64, 0, len(sorted))
	for _, k := range sorted {
		sig := falcon.Evaluate(k.High, k.Low, k.Close, k.Volume)
		evaluated = append(evaluated, evaluatedBar{k: k, sig: sig})
		rsxValues = append(rsxValues, sig.JurikRSX)
	}
	rsxPoints := strategy.BuildRSXChart(sorted, rsxValues, rsxLookback)

	for i, ev := range evaluated {
		chart, ok := ChartCandleFromKline(ev.k)
		if !ok {
			continue
		}
		sig := ev.sig
		rsxMeta := rsxPoints[i]
		spikeUp := false
		spikeDown := false
		if len(candles) > 0 {
			spikeUp = strategy.DetectWozduxVolumeSpikeUp(prevBlue, sig.BlueLine, sig.RedLine)
			spikeDown = strategy.DetectWozduxVolumeSpikeDown(prevBlue, sig.BlueLine, sig.RedLine)
		}
		osc := ChartOscillator{
			Time:            chart.Time,
			Jurik:           sig.JurikRSX,
			RSX:             sig.JurikRSX,
			RSXSignal:       sig.JurikRSXSignal,
			Red:             sig.RedLine,
			Green:           sig.GreenLine,
			Blue:            sig.BlueLine,
			RsiPrice:        sig.RsiPrice,
			EmaRsi:          sig.EmaRsi,
			RsiRsi:          sig.RsiRsi,
			RsiHl2:          sig.RsiHl2,
			RsiVolFast:      sig.RsiVolFast,
			RsiVolSlow:      sig.RsiVolSlow,
			MacdRsi:         sig.MacdRsi,
			RsiAd:           sig.RsiAd,
			RsiHl2Vol:       sig.RsiHl2Vol,
			VolCrossMarker:  sig.VolCrossMarker,
			VolChanMid:      sig.VolChanMid,
			VolChanUp:       sig.VolChanUp,
			VolChanDn:       sig.VolChanDn,
			PriceChanMid:    sig.PriceChanMid,
			PriceChanUp:     sig.PriceChanUp,
			PriceChanDn:     sig.PriceChanDn,
			Color:           rsxMeta.Color,
			Marker:          rsxMeta.Marker,
			VolumeSpikeUp:   spikeUp,
			VolumeSpikeDown: spikeDown,
		}

		if len(candles) > 0 && candles[len(candles)-1].Time == chart.Time {
			candles[len(candles)-1] = chart
			oscillators[len(oscillators)-1] = osc
		} else {
			candles = append(candles, chart)
			oscillators = append(oscillators, osc)
		}
		prevBlue = sig.BlueLine
	}
	return candles, oscillators
}

// historyEndTimeToMs converts Lightweight Charts endTime (Unix seconds) to Binance milliseconds.
func historyEndTimeToMs(endTimeSec int64) int64 {
	if endTimeSec <= 0 {
		return 0
	}
	// Guard: if a client accidentally sends milliseconds, pass through unchanged.
	if endTimeSec > 1e12 {
		return endTimeSec
	}
	return endTimeSec * 1000
}

func buildChartSeriesTrimmed(klines []exchange.Kline, trim, rsxLookback int) ([]ChartCandle, []ChartOscillator) {
	candles, oscillators := buildChartSeries(klines, rsxLookback)
	if trim <= 0 || len(candles) <= trim {
		return candles, oscillators
	}
	return candles[trim:], oscillators[trim:]
}

func candlesToKlines(candles []exchange.Candle) []exchange.Kline {
	klines := make([]exchange.Kline, len(candles))
	for i, c := range candles {
		klines[i] = exchange.Kline{
			OpenTime: c.OpenTime,
			Open:     c.Open,
			High:     c.High,
			Low:      c.Low,
			Close:    c.Close,
			Volume:   c.Volume,
		}
	}
	return klines
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func chartFibZonesFromReport(zones []indicators.FibZone) []ChartFibZone {
	if len(zones) == 0 {
		return nil
	}
	out := make([]ChartFibZone, 0, len(zones))
	for _, z := range zones {
		if z.Type != indicators.Retracement && z.Type != indicators.Extension {
			continue
		}
		out = append(out, ChartFibZone{
			Ratio:    z.Ratio,
			Price:    z.TargetValue,
			IsActive: z.IsActive,
		})
	}
	return out
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
	d.clientTF[client] = defaultTimeframe
	d.clientsMu.Unlock()

	defer func() {
		d.clientsMu.Lock()
		delete(d.clients, client)
		delete(d.clientTF, client)
		d.clientsMu.Unlock()
		_ = client.Close()
	}()

	log.Printf("[Dashboard] ws client connected: %s", r.RemoteAddr)

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
			log.Printf("[Dashboard] ws subscribe tf=%s from %s", msg.TF, r.RemoteAddr)
		}
	}
}
