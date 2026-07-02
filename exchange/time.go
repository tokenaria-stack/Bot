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

// NormalizeKline coerces OpenTime and CloseTime to Unix milliseconds.
func NormalizeKline(k Kline) Kline {
	k.OpenTime = EnsureUnixMillis(k.OpenTime)
	if k.CloseTime > 0 {
		k.CloseTime = EnsureUnixMillis(k.CloseTime)
	}
	return k
}
