package market

import (
	"trading_bot/exchange"
)

const (
	RSXColorGreen         = "#089981"
	RSXColorRed           = "#f23645"
	RSXColorNeutral       = "#e1d2b5"
	rsxMacroPivotRadius   = 7
	rsxPeakIndexTolerance = 2
)

// RSXPoint holds chart metadata for a single RSX oscillator bar.
// Marker is a Phase F socket (always empty until a new strategy label surface exists).
type RSXPoint struct {
	Color  string
	Marker string
}

// RSXColor returns RSX line color from the current bar vs previous bar (no hysteresis).
func RSXColor(currentRSX, prevRSX float64) string {
	isRising := currentRSX > prevRSX
	isFalling := currentRSX < prevRSX

	color := RSXColorNeutral
	if isRising && currentRSX > 50 {
		color = RSXColorGreen
	} else if isFalling && currentRSX < 50 {
		color = RSXColorRed
	}
	return color
}

// BuildRSXChart assigns RSX line colors for a precomputed series.
// Trading labels (L/LL/S/SS) are not published — Phase F purge.
func BuildRSXChart(klines []exchange.Kline, rsxValues []float64, lookback int) []RSXPoint {
	n := len(klines)
	if n == 0 || len(rsxValues) != n {
		return nil
	}
	_ = lookback

	points := make([]RSXPoint, n)
	for i := 0; i < n; i++ {
		prevRSX := 0.0
		if i > 0 {
			prevRSX = rsxValues[i-1]
		}
		points[i].Color = RSXColor(rsxValues[i], prevRSX)
	}
	return points
}

func buildRSXPriceSeries(klines []exchange.Kline, source string) []float64 {
	out := make([]float64, len(klines))
	for i, k := range klines {
		out[i] = RSXSourcePrice(k.High, k.Low, k.Close, source)
	}
	return out
}
