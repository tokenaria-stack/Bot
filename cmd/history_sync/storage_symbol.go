package main

import (
	"strings"

	"trading_bot/exchange"
)

// pairSymbol is the Binance Vision / REST pair (BTCUSDT), never the SQLite spot suffix.
func pairSymbol(symbol string) string {
	s := exchange.NormalizeFuturesSymbol(symbol)
	s = strings.TrimSuffix(s, "_SPOT")
	return exchange.NormalizeFuturesSymbol(s)
}

// persistStorageSymbol is the SQLite klines.symbol for this Vision market.
// Spot MUST be BTCUSDT_SPOT so GetWindow / LoadContinuousContractBeforeEnd can stitch.
// Futures stays BTCUSDT (canonical UM key).
func persistStorageSymbol(market, pair string) string {
	pair = pairSymbol(pair)
	if strings.EqualFold(strings.TrimSpace(market), "spot") {
		return exchange.SpotStorageSymbol(pair)
	}
	return pair
}
