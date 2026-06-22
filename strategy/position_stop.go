package strategy

import "trading_bot/exchange"

const DefaultFractalStopBars = 15

// findFractalATRStop returns a structural stop: lowest low − ATR×mult (long) or highest high + ATR×mult (short).
func findFractalATRStop(klines []exchange.Kline, currentIndex int, isLong bool, lookback int, atr, atrMultiplier float64) float64 {
	if len(klines) == 0 || currentIndex < 0 || lookback <= 0 || atr <= 0 {
		return 0
	}
	if currentIndex >= len(klines) {
		currentIndex = len(klines) - 1
	}

	start := currentIndex - (lookback - 1)
	if start < 0 {
		start = 0
	}

	mult := atrMultiplier
	if mult <= 0 {
		mult = GetRiskSettings().ATRMultiplier
	}
	if mult <= 0 {
		mult = 1.5
	}
	offset := atr * mult

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
		return low - offset
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
	return high + offset
}

// computePositionStop returns the stop for a new position per RiskSettings.
func computePositionStop(klines []exchange.Kline, barIndex int, isLong bool, atr float64, risk *RiskSettings) float64 {
	if risk == nil {
		risk = GetRiskSettings()
	}
	if risk.StopLossType == "fractal_atr" && atr > 0 {
		stop := findFractalATRStop(klines, barIndex, isLong, DefaultFractalStopBars, atr, risk.ATRMultiplier)
		if stop > 0 {
			return stop
		}
	}
	return findPositionalFractal(klines, barIndex, isLong)
}
