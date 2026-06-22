package indicators

// Indicator defines a streaming interface for any mathematical calculation.
// Update accepts a new sample (price or output of another indicator)
// and returns the current calculated result.
type Indicator interface {
	Update(val float64) float64
	Value() float64
}

// CandleIndicator defines a streaming interface for OHLC-based calculations.
type CandleIndicator interface {
	UpdateCandle(high, low, close float64) float64
	Value() float64
}
