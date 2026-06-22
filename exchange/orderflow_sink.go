package exchange

// OrderFlowSink receives live aggTrade and liquidation events from the WebSocket layer.
type OrderFlowSink interface {
	PushAggTrade(price, qty float64, timeMs int64, isBuyerMaker bool)
	PushLiquidation(price, qty float64, side string, timeMs int64)
}
