package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

const (
	wsMinBackoff       = time.Second
	wsMaxBackoff       = 30 * time.Second
	wsStableConnection = 60 * time.Second
)

// BinanceExchange implements USDⓈ-M Futures REST/WebSocket via go-binance futures client.
type BinanceExchange struct {
	client    *futures.Client
	norm      *Normalizer
	isTestnet bool
}

// NewBinanceExchange creates a Binance USDⓈ-M Futures client.
// isTestnet=false uses mainnet: REST https://fapi.binance.com, WS wss://fstream.binance.com
func NewBinanceExchange(apiKey, apiSecret string, isTestnet bool) (*BinanceExchange, error) {
	futures.UseTestnet = isTestnet

	client := futures.NewClient(apiKey, apiSecret)

	norm := NewNormalizer()
	if err := loadNormalizerLimits(context.Background(), norm, client, isTestnet); err != nil {
		return nil, fmt.Errorf("load exchange limits: %w", err)
	}

	return &BinanceExchange{
		client:    client,
		norm:      norm,
		isTestnet: isTestnet,
	}, nil
}

// RESTKlinesEndpoint returns the USDⓈ-M Futures klines URL used by this client.
func (b *BinanceExchange) RESTKlinesEndpoint() string {
	if b.isTestnet {
		return "https://testnet.binancefuture.com/fapi/v1/klines"
	}
	return "https://fapi.binance.com/fapi/v1/klines"
}

// UsesFuturesClient reports whether the REST client is a *futures.Client (never spot).
func (b *BinanceExchange) UsesFuturesClient() bool {
	return b.client != nil
}

// IsTestnet reports whether this client targets the Binance futures testnet.
func (b *BinanceExchange) IsTestnet() bool {
	return b.isTestnet
}

// Ping calls GET /fapi/v1/ping on the futures API.
func (b *BinanceExchange) Ping() error {
	return b.client.NewPingService().Do(context.Background())
}

// CreateMarketOrder sends a signed POST /fapi/v1/order (MARKET).
func (b *BinanceExchange) CreateMarketOrder(symbol, side string, quantity float64) (string, error) {
	orderSide := futures.SideTypeBuy
	if side == "SELL" {
		orderSide = futures.SideTypeSell
	}

	qtyStr, err := b.norm.FormatQuantity(symbol, quantity)
	if err != nil {
		qtyStr = fmt.Sprintf("%.6f", quantity)
	}

	resp, err := b.client.NewCreateOrderService().
		Symbol(symbol).
		Side(orderSide).
		Type(futures.OrderTypeMarket).
		Quantity(qtyStr).
		Do(context.Background())
	if err != nil {
		log.Printf("[BinanceExchange] ⚠️ Failed to create MARKET order: %v", err)
		return "", err
	}

	return fmt.Sprintf("%d", resp.OrderID), nil
}

// CreateLimitOrder sends a signed GTC limit order on futures.
func (b *BinanceExchange) CreateLimitOrder(symbol, side string, quantity, price float64) (string, error) {
	orderSide, err := parseFuturesOrderSide(side)
	if err != nil {
		return "", err
	}

	formattedQty, err := b.norm.FormatQuantity(symbol, quantity)
	if err != nil {
		return "", fmt.Errorf("format quantity: %w", err)
	}

	formattedPrice, err := b.norm.FormatPrice(symbol, price)
	if err != nil {
		return "", fmt.Errorf("format price: %w", err)
	}

	response, err := b.client.NewCreateOrderService().
		Symbol(symbol).
		Side(orderSide).
		Type(futures.OrderTypeLimit).
		TimeInForce(futures.TimeInForceTypeGTC).
		Quantity(formattedQty).
		Price(formattedPrice).
		Do(context.Background())
	if err != nil {
		return "", fmt.Errorf("create limit order: %w", err)
	}

	return marshalFuturesOrderResponse(response)
}

// CreateStopMarketOrder places a STOP_MARKET conditional order via POST /fapi/v1/algoOrder.
func (b *BinanceExchange) CreateStopMarketOrder(symbol, side string, quantity, stopPrice float64) error {
	orderSide := futures.SideTypeBuy
	if side == "SELL" {
		orderSide = futures.SideTypeSell
	}

	triggerPriceStr, err := b.norm.FormatPrice(symbol, stopPrice)
	if err != nil {
		triggerPriceStr = fmt.Sprintf("%.2f", stopPrice)
	}

	_, err = b.client.NewCreateAlgoOrderService().
		AlgoType(futures.OrderAlgoTypeConditional).
		Symbol(symbol).
		Side(orderSide).
		Type(futures.AlgoOrderTypeStopMarket).
		TriggerPrice(triggerPriceStr).
		WorkingType(futures.WorkingTypeMarkPrice).
		ClosePosition(true).
		Do(context.Background())
	if err == nil {
		return nil
	}

	log.Printf("[BinanceExchange] ⚠️ STOP_MARKET closePosition failed, retrying with reduceOnly: %v", err)

	if quantity <= 0 {
		log.Printf("[BinanceExchange] ⚠️ Failed to create STOP_MARKET algo order: %v", err)
		return err
	}

	qtyStr, qtyErr := b.norm.FormatQuantity(symbol, quantity)
	if qtyErr != nil {
		qtyStr = fmt.Sprintf("%.6f", quantity)
	}

	_, err = b.client.NewCreateAlgoOrderService().
		AlgoType(futures.OrderAlgoTypeConditional).
		Symbol(symbol).
		Side(orderSide).
		Type(futures.AlgoOrderTypeStopMarket).
		TriggerPrice(triggerPriceStr).
		Quantity(qtyStr).
		ReduceOnly(true).
		WorkingType(futures.WorkingTypeMarkPrice).
		Do(context.Background())
	if err != nil {
		log.Printf("[BinanceExchange] ⚠️ Failed to create STOP_MARKET algo order: %v", err)
		return err
	}

	return nil
}

// GetPositionAmt returns futures positionAmt for symbol (negative = short, positive = long).
func (b *BinanceExchange) GetPositionAmt(symbol string) (float64, error) {
	if b.client == nil {
		return 0, fmt.Errorf("futures client is not configured")
	}

	risks, err := b.client.NewGetPositionRiskService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("get position risk: %w", err)
	}

	for _, risk := range risks {
		if risk == nil || risk.Symbol != symbol {
			continue
		}

		amt, err := strconv.ParseFloat(risk.PositionAmt, 64)
		if err != nil {
			return 0, fmt.Errorf("parse positionAmt for %s: %w", symbol, err)
		}

		if risk.PositionSide == string(futures.PositionSideTypeBoth) || risk.PositionSide == "" {
			return amt, nil
		}
	}

	for _, risk := range risks {
		if risk == nil || risk.Symbol != symbol {
			continue
		}
		amt, err := strconv.ParseFloat(risk.PositionAmt, 64)
		if err != nil {
			return 0, fmt.Errorf("parse positionAmt for %s: %w", symbol, err)
		}
		return amt, nil
	}

	return 0, nil
}

// ChangeLeverage sets initial leverage for a symbol (POST /fapi/v1/leverage).
func (b *BinanceExchange) ChangeLeverage(symbol string, leverage int) error {
	if b.client == nil {
		return fmt.Errorf("futures client is not configured")
	}
	if leverage < 1 {
		return fmt.Errorf("leverage must be >= 1, got %d", leverage)
	}

	_, err := b.client.NewChangeLeverageService().
		Symbol(symbol).
		Leverage(leverage).
		Do(context.Background())
	if err != nil {
		return fmt.Errorf("change leverage for %s to %dx: %w", symbol, leverage, err)
	}

	return nil
}

// GetFuturesBalance returns available wallet balance for the given asset (e.g. USDT).
func (b *BinanceExchange) GetFuturesBalance(asset string) (float64, error) {
	if b.client == nil {
		return 0, fmt.Errorf("futures client is not configured")
	}

	balances, err := b.client.NewGetBalanceService().Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("get futures balance: %w", err)
	}

	for _, bal := range balances {
		if bal == nil || bal.Asset != asset {
			continue
		}

		available, err := strconv.ParseFloat(bal.AvailableBalance, 64)
		if err != nil {
			return 0, fmt.Errorf("parse available balance for %s: %w", asset, err)
		}
		return available, nil
	}

	return 0, fmt.Errorf("asset %q not found in futures balance", asset)
}

// CancelAllOpenOrders cancels all open futures and algo orders for the symbol.
func (b *BinanceExchange) CancelAllOpenOrders(symbol string) error {
	if err := b.client.NewCancelAllOpenOrdersService().Symbol(symbol).Do(context.Background()); err != nil {
		return fmt.Errorf("cancel open orders for %s: %w", symbol, err)
	}

	if err := b.client.NewCancelAllAlgoOpenOrdersService().Symbol(symbol).Do(context.Background()); err != nil {
		return fmt.Errorf("cancel algo open orders for %s: %w", symbol, err)
	}

	return nil
}

func marshalFuturesOrderResponse(response *futures.CreateOrderResponse) (string, error) {
	raw, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal order response: %w", err)
	}

	return string(raw), nil
}

func parseFuturesOrderSide(side string) (futures.SideType, error) {
	switch side {
	case "BUY":
		return futures.SideTypeBuy, nil
	case "SELL":
		return futures.SideTypeSell, nil
	default:
		return "", fmt.Errorf("unsupported order side %q", side)
	}
}

// StreamKlines maintains a resilient futures WebSocket kline feed with exponential backoff reconnect.
func (b *BinanceExchange) StreamKlines(ctx context.Context, symbol, interval string, outCh chan<- Kline) error {
	_ = b

	backoff := wsMinBackoff

	for {
		wsHandler := func(event *futures.WsKlineEvent) {
			slog.Debug("Received futures kline event", "symbol", event.Symbol)

			kline, err := mapFuturesWsKlineEvent(event)
			if err != nil {
				slog.Error("failed to parse ws kline", "error", err)
				return
			}

			select {
			case <-ctx.Done():
				return
			case outCh <- kline:
			}
		}

		errHandler := func(err error) {
			slog.Error("binance futures kline websocket error", "error", err)
		}

		doneC, stopC, err := futures.WsKlineServe(symbol, interval, wsHandler, errHandler)
		if err != nil {
			slog.Warn("failed to start kline websocket", "error", err, "retry_in", backoff)
			if sleepErr := sleepWithContext(ctx, backoff); sleepErr != nil {
				return nil
			}
			backoff = nextBackoff(backoff)
			continue
		}

		connectedAt := time.Now()

		select {
		case <-ctx.Done():
			stopC <- struct{}{}
			slog.Info("Closing websocket connection")
			return nil
		case <-doneC:
			slog.Warn("websocket connection closed by exchange, reconnecting", "backoff", backoff)
			if time.Since(connectedAt) > wsStableConnection {
				backoff = wsMinBackoff
			}
			if sleepErr := sleepWithContext(ctx, backoff); sleepErr != nil {
				return nil
			}
			backoff = nextBackoff(backoff)
		}
	}
}

func mapFuturesWsKlineEvent(event *futures.WsKlineEvent) (Kline, error) {
	k := event.Kline

	open, err := parseKlineFloat(k.Open, "open")
	if err != nil {
		return Kline{}, err
	}

	high, err := parseKlineFloat(k.High, "high")
	if err != nil {
		return Kline{}, err
	}

	low, err := parseKlineFloat(k.Low, "low")
	if err != nil {
		return Kline{}, err
	}

	closePrice, err := parseKlineFloat(k.Close, "close")
	if err != nil {
		return Kline{}, err
	}

	volume, err := parseKlineFloat(k.Volume, "volume")
	if err != nil {
		return Kline{}, err
	}

	return Kline{
		OpenTime: k.StartTime,
		Open:     open,
		High:     high,
		Low:      low,
		Close:    closePrice,
		Volume:   volume,
	}, nil
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > wsMaxBackoff {
		return wsMaxBackoff
	}
	return next
}

func parseKlineFloat(value, field string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s %q: %w", field, value, err)
	}
	return parsed, nil
}

var _ Exchange = (*BinanceExchange)(nil)
