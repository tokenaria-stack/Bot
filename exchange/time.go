package exchange

// ChartTimeSec converts a canonical Unix-millisecond open time to Lightweight Charts
// Unix seconds. Debt #83 C3: ms → sec only — no unit inference.
// Callers must supply Unix milliseconds (REST/WS C1, Frame/Ingress C2, SQLite ms).
func ChartTimeSec(openTimeMs int64) int64 {
	return openTimeMs / 1000
}

// klineFromBinanceMs builds a Kline from Binance REST/WS fields that are
// documented as Unix milliseconds. No unit inference or conversion (debt #83 C1).
func klineFromBinanceMs(openMs, closeMs int64, open, high, low, close, volume float64) Kline {
	return Kline{
		OpenTime:  openMs,
		CloseTime: closeMs,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
	}
}
