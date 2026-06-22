package domain

import "trading_bot/exchange"

// Kline is the canonical OHLCV bar for indicator math (layer-neutral).
type Kline = Candle

// Candle is a normalized OHLCV bar for dashboard streaming.
type Candle struct {
	OpenTime int64
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

// CandleFromKline converts an exchange kline into a domain candle.
func CandleFromKline(k exchange.Kline) Candle {
	return Candle{
		OpenTime: k.OpenTime,
		Open:     k.Open,
		High:     k.High,
		Low:      k.Low,
		Close:    k.Close,
		Volume:   k.Volume,
	}
}
