package exchange

import "github.com/adshao/go-binance/v2/futures"

// FuturesWSCombinedURL returns the combined market-data WebSocket base URL for the active network.
// Mainnet: wss://fstream.binance.com/market/stream?streams=
// (Legacy wss://fstream.binance.com/stream?streams= no longer delivers market data.)
func FuturesWSCombinedURL() string {
	if futures.UseTestnet {
		return futures.BaseCombinedMarketTestnetURL
	}
	return futures.BaseCombinedMarketMainURL
}
