package exchange

import "strings"

// NormalizeFuturesSymbol strips TradingView-style suffixes (e.g. BTCUSDT.P, BTCUSDT_PERP)
// to the plain pair name required by Binance USDⓈ-M Futures REST/WS (BTCUSDT).
func NormalizeFuturesSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	symbol = strings.ToUpper(symbol)
	if symbol == "" {
		return symbol
	}
	if idx := strings.Index(symbol, "."); idx > 0 {
		symbol = symbol[:idx]
	}
	symbol = strings.TrimSuffix(symbol, "_PERP")
	return symbol
}
