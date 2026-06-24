package exchange

import "context"

// Kline represents a single OHLCV candle (OpenTime/CloseTime in Unix milliseconds).
type Kline struct {
	OpenTime  int64
	CloseTime int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// Exchange describes common operations supported by cryptocurrency exchanges.
type Exchange interface {
	// Ping checks connectivity to the exchange API.
	Ping() error

	// GetKlines returns historical candles for the given symbol and interval.
	GetKlines(symbol, interval string, limit int) ([]Candle, error)

	// StreamKlines pushes live candles into outCh until ctx is cancelled.
	StreamKlines(ctx context.Context, symbol, interval string, outCh chan<- Kline) error
}
