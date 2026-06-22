package indicators

import (
	"math"
	"trading_bot/domain"
)

// CalculateATR вычисляет Average True Range для массива свечей
// period - период сглаживания (по умолчанию 14)
func CalculateATR(klines []domain.Kline, period int) []float64 {
	if len(klines) < period+1 {
		return nil
	}

	atr := make([]float64, len(klines))
	trueRanges := make([]float64, len(klines))

	// 1. Вычисляем True Range (TR)
	for i := 1; i < len(klines); i++ {
		highMinusLow := klines[i].High - klines[i].Low
		highMinusPrevClose := math.Abs(klines[i].High - klines[i-1].Close)
		lowMinusPrevClose := math.Abs(klines[i].Low - klines[i-1].Close)

		tr := highMinusLow
		if highMinusPrevClose > tr {
			tr = highMinusPrevClose
		}
		if lowMinusPrevClose > tr {
			tr = lowMinusPrevClose
		}
		trueRanges[i] = tr
	}

	// 2. Первое значение ATR
	sumTR := 0.0
	for i := 1; i <= period; i++ {
		sumTR += trueRanges[i]
	}
	atr[period] = sumTR / float64(period)

	// 3. Последующие значения сглаживаются
	for i := period + 1; i < len(klines); i++ {
		atr[i] = ((atr[i-1] * float64(period-1)) + trueRanges[i]) / float64(period)
	}

	return atr
}
