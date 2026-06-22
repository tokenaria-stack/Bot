package strategy

import "trading_bot/exchange"

const (
	fractalSLLookback    = 15
	fractalSLLongBuffer  = 0.999
	fractalSLShortBuffer = 1.001
)

// findPositionalFractal returns a structural stop level from the last fractalSLLookback bars.
// Long: lowest low × 0.999; Short: highest high × 1.001.
func findPositionalFractal(klines []exchange.Kline, currentIndex int, isLong bool) float64 {
	if len(klines) == 0 || currentIndex < 0 {
		return 0
	}
	if currentIndex >= len(klines) {
		currentIndex = len(klines) - 1
	}

	start := currentIndex - (fractalSLLookback - 1)
	if start < 0 {
		start = 0
	}

	if isLong {
		low := klines[start].Low
		for i := start + 1; i <= currentIndex; i++ {
			if klines[i].Low < low {
				low = klines[i].Low
			}
		}
		if low <= 0 {
			return 0
		}
		return low * fractalSLLongBuffer
	}

	high := klines[start].High
	for i := start + 1; i <= currentIndex; i++ {
		if klines[i].High > high {
			high = klines[i].High
		}
	}
	if high <= 0 {
		return 0
	}
	return high * fractalSLShortBuffer
}
