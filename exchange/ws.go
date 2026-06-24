package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/gorilla/websocket"
)

// WsTick contains a parsed candle, its timeframe, and closed status.
type WsTick struct {
	Timeframe string
	Kline     Kline
	IsClosed  bool
}

type wsStreamEnvelope struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

type wsKlinePayload struct {
	Kline struct {
		StartTime int64      `json:"t"`
		CloseTime int64      `json:"T"`
		Interval  string     `json:"i"`
		Open      flexString `json:"o"`
		Close     flexString `json:"c"`
		High      flexString `json:"h"`
		Low       flexString `json:"l"`
		Volume    flexString `json:"v"`
		IsClosed  bool       `json:"x"`
	} `json:"k"`
}

type wsAggTradeEvent struct {
	Price        string `json:"p"`
	Quantity     string `json:"q"`
	TradeTime    int64  `json:"T"`
	IsBuyerMaker bool   `json:"m"`
}

type wsForceOrderEvent struct {
	Order struct {
		Side          string `json:"S"`
		AveragePrice  string `json:"ap"`
		Price         string `json:"p"`
		LastFilledQty string `json:"l"`
		TradeTime     int64  `json:"T"`
	} `json:"o"`
}

// WsClient manages a Binance WebSocket connection.
type WsClient struct {
	symbol    string
	OutCh     chan WsTick
	orderFlow OrderFlowSink

	aggTradeCount atomic.Uint64
}

// NewWsClient creates a WebSocket client for the given symbol.
func NewWsClient(symbol string, orderFlow OrderFlowSink) *WsClient {
	return &WsClient{
		symbol:    strings.ToLower(NormalizeFuturesSymbol(symbol)),
		OutCh:     make(chan WsTick, 1000),
		orderFlow: orderFlow,
	}
}

// AggTradeCount returns the number of aggTrade events ingested since start.
func (c *WsClient) AggTradeCount() uint64 {
	return c.aggTradeCount.Load()
}

// Start connects to Binance and begins listening with automatic reconnect.
func (c *WsClient) Start(ctx context.Context) error {
	go c.run(ctx)
	return nil
}

func (c *WsClient) run(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		if err := c.connectAndListen(ctx); err != nil && ctx.Err() != nil {
			return
		}
		log.Printf("[WS] disconnected, reconnecting in %s", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *WsClient) connectAndListen(ctx context.Context) error {
	streams := []string{
		fmt.Sprintf("%s@kline_1m", c.symbol),
		fmt.Sprintf("%s@kline_3m", c.symbol),
		fmt.Sprintf("%s@kline_5m", c.symbol),
		fmt.Sprintf("%s@kline_15m", c.symbol),
		fmt.Sprintf("%s@kline_30m", c.symbol),
		fmt.Sprintf("%s@kline_1h", c.symbol),
		fmt.Sprintf("%s@kline_4h", c.symbol),
		fmt.Sprintf("%s@kline_1d", c.symbol),
		fmt.Sprintf("%s@kline_1w", c.symbol),
		fmt.Sprintf("%s@kline_1M", c.symbol),
		fmt.Sprintf("%s@aggTrade", c.symbol),
		fmt.Sprintf("%s@forceOrder", c.symbol),
	}

	url := FuturesWSCombinedURL() + strings.Join(streams, "/")
	log.Printf("[WS] Connecting to %s (mainnet=%v)", url, !futures.UseTestnet)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}
	defer conn.Close()

	var msgCount int
	for {
		select {
		case <-ctx.Done():
			log.Println("[WS] Shutting down connection...")
			return ctx.Err()
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		msgCount++
		if msgCount == 1 {
			log.Printf("[WS] First combined stream message received (%d bytes)", len(message))
		}

		var envelope wsStreamEnvelope
		if err := json.Unmarshal(message, &envelope); err != nil {
			log.Printf("[WS ERROR] JSON parse failed: %v", err)
			continue
		}

		switch {
		case strings.Contains(envelope.Stream, "@kline_"):
			c.handleKline(ctx, envelope.Data)
		case strings.Contains(envelope.Stream, "@aggTrade"):
			c.handleAggTrade(envelope.Data)
		case strings.Contains(envelope.Stream, "@forceOrder"):
			c.handleForceOrder(envelope.Data)
		}
	}
}

func (c *WsClient) handleKline(ctx context.Context, raw json.RawMessage) {
	var event wsKlinePayload
	if err := json.Unmarshal(raw, &event); err != nil {
		log.Printf("[WS ERROR] kline parse: %v", err)
		return
	}
	if event.Kline.Interval == "" {
		return
	}

	kdata := event.Kline
	open, _ := kdata.Open.Float64()
	high, _ := kdata.High.Float64()
	low, _ := kdata.Low.Float64()
	closePrice, _ := kdata.Close.Float64()
	volume, _ := kdata.Volume.Float64()

	tick := WsTick{
		Timeframe: kdata.Interval,
		IsClosed:  kdata.IsClosed,
		Kline: NormalizeKline(Kline{
			OpenTime:  kdata.StartTime,
			CloseTime: kdata.CloseTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closePrice,
			Volume:    volume,
		}),
	}

	select {
	case <-ctx.Done():
	case c.OutCh <- tick:
	}
}

func (c *WsClient) handleAggTrade(raw json.RawMessage) {
	if c.orderFlow == nil {
		return
	}

	var event wsAggTradeEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		log.Printf("[WS ERROR] aggTrade parse: %v", err)
		return
	}

	price, err := strconv.ParseFloat(event.Price, 64)
	if err != nil {
		return
	}
	qty, err := strconv.ParseFloat(event.Quantity, 64)
	if err != nil {
		return
	}
	if event.TradeTime <= 0 || price <= 0 {
		return
	}

	c.orderFlow.PushAggTrade(price, qty, event.TradeTime, event.IsBuyerMaker)
	n := c.aggTradeCount.Add(1)
	if n == 1 || n%5000 == 0 {
		log.Printf("[WS] aggTrade ingested: %d total", n)
	}
}

func (c *WsClient) handleForceOrder(raw json.RawMessage) {
	if c.orderFlow == nil {
		return
	}

	var event wsForceOrderEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		log.Printf("[WS ERROR] forceOrder parse: %v", err)
		return
	}

	priceStr := event.Order.AveragePrice
	if priceStr == "" || priceStr == "0" {
		priceStr = event.Order.Price
	}
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil || price <= 0 {
		return
	}
	qty, err := strconv.ParseFloat(event.Order.LastFilledQty, 64)
	if err != nil || qty <= 0 {
		return
	}
	if event.Order.TradeTime <= 0 {
		return
	}

	c.orderFlow.PushLiquidation(price, qty, event.Order.Side, event.Order.TradeTime)
}
