package exchange

const unixMillisThreshold int64 = 1_000_000_000_000

// EnsureUnixMillis converts 10-digit Unix seconds to 13-digit milliseconds.
func EnsureUnixMillis(ts int64) int64 {
	if ts > 0 && ts < unixMillisThreshold {
		return ts * 1000
	}
	return ts
}

// ChartTimeSec converts any kline open time (sec or ms) to Lightweight Charts Unix seconds.
// This is the SSOT wire-axis transform for ChartCandle.time and ChartOscillator.time.
func ChartTimeSec(openTime int64) int64 {
	return EnsureUnixMillis(openTime) / 1000
}

// NormalizeKline applies EnsureUnixMillis to OpenTime/CloseTime only.
// After debt #83 C1/C2, production REST/WS/Frame/Ingress paths no longer call this;
// they assume canonical Unix ms. Retained for tests/probes and until ChartTimeSec
// stops using EnsureUnixMillis (Patch C3).
func NormalizeKline(k Kline) Kline {
	k.OpenTime = EnsureUnixMillis(k.OpenTime)
	if k.CloseTime > 0 {
		k.CloseTime = EnsureUnixMillis(k.CloseTime)
	}
	return k
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
