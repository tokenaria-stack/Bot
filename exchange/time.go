package exchange

// ChartTimeSec converts a canonical Unix-millisecond open time to Lightweight Charts
// Unix seconds. Debt #83 C3: ms → sec only — no unit inference, no EnsureUnixMillis.
// Callers must supply Unix milliseconds (REST/WS C1, Frame/Ingress C2, SQLite ms).
func ChartTimeSec(openTimeMs int64) int64 {
	return openTimeMs / 1000
}

// EnsureUnixMillis converts 10-digit Unix seconds to 13-digit milliseconds via a
// magnitude heuristic. After #83 C1–C3 it has no production callers (ChartTimeSec
// no longer uses it). Retained only while tests/probes still reference NormalizeKline.
// Do not use for new code — source adapters must declare their unit explicitly.
func EnsureUnixMillis(ts int64) int64 {
	const unixMillisThreshold int64 = 1_000_000_000_000
	if ts > 0 && ts < unixMillisThreshold {
		return ts * 1000
	}
	return ts
}

// NormalizeKline applies EnsureUnixMillis to OpenTime/CloseTime only.
// No production callers remain after C1/C2; kept for legacy tests until cleaned up.
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
