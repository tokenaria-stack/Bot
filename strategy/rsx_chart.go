package strategy

import (
	"trading_bot/exchange"
	"trading_bot/indicators"
)

const (
	RSXColorGreen         = "#089981"
	RSXColorRed           = "#f23645"
	RSXColorNeutral       = "#e1d2b5"
	rsxMacroPivotRadius   = 7
	rsxPeakIndexTolerance = 2
)

// RSXPoint holds chart metadata for a single RSX oscillator bar.
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

// LatestRSXChartMarker returns the actionable RSX marker on the latest bar (L/LL/S/SS).
func LatestRSXChartMarker(klines []exchange.Kline, lookback int) string {
	if len(klines) == 0 {
		return ""
	}
	settings := GetRSXSettings()
	if lookback <= 0 {
		lookback = settings.DivLookback
	}

	falcon := NewFalconEngine()
	rsxValues := make([]float64, len(klines))
	for i, k := range klines {
		rsxValues[i] = falcon.Evaluate(k.High, k.Low, k.Close, k.Volume).JurikRSX
	}
	points := BuildRSXChart(klines, rsxValues, lookback)
	return RSXTradingMarkerAtBar(points, len(points)-1)
}

// BuildRSXChart assigns colors and dashboard markers for a precomputed RSX series.
func BuildRSXChart(klines []exchange.Kline, rsxValues []float64, lookback int) []RSXPoint {
	n := len(klines)
	if n == 0 || len(rsxValues) != n {
		return nil
	}
	settings := GetRSXSettings()
	if lookback <= 0 {
		lookback = settings.DivLookback
	}

	prices := buildRSXPriceSeries(klines, settings.Source)
	closes := make([]float64, n)
	for i, k := range klines {
		closes[i] = k.Close
	}

	points := make([]RSXPoint, n)
	for i := 0; i < n; i++ {
		prevRSX := 0.0
		if i > 0 {
			prevRSX = rsxValues[i-1]
		}
		points[i].Color = RSXColor(rsxValues[i], prevRSX)
	}

	cfg := RSXScanConfigFromSettings(settings)
	engine := indicators.NewSmartDivergenceEngine(cfg)
	bus := newBatchDataBus(rsxValues, prices, closes)
	for _, hit := range engine.ScanRSX(bus) {
		if hit.DisplayBar >= 0 && hit.DisplayBar < n && hit.Label != "" {
			points[hit.DisplayBar].Marker = hit.Label
		}
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
