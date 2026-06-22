// Package vector_db integrates with Qdrant for pattern and fractal embeddings.
package vector_db

import (
	"fmt"

	"trading_bot/exchange"
)

// PatternStore persists and retrieves fractal/pattern embeddings in Qdrant.
type PatternStore struct{}

// VectorizeCandles builds a price-independent float32 embedding from candle closes.
func VectorizeCandles(klines []exchange.Kline) ([]float32, error) {
	if len(klines) < 2 {
		return nil, fmt.Errorf("not enough klines to vectorize: got %d, need at least 2", len(klines))
	}

	basePrice := klines[0].Close
	if basePrice == 0 {
		return nil, fmt.Errorf("base close price is zero, cannot normalize")
	}

	vector := make([]float32, len(klines))
	for i, k := range klines {
		deviation := ((k.Close - basePrice) / basePrice) * 100
		vector[i] = float32(deviation)
	}

	return vector, nil
}

const (
	// ReportVectorSize is the fixed embedding length for market report snapshots.
	ReportVectorSize = 6

	regimeExpansion = "EXPANSION"
	regimeSqueeze   = "SQUEEZE"
	regimeClimax    = "CLIMAX"
)

// ReportSnapshot is a strategy.Report projection used for vector embeddings.
// Defined here to avoid an import cycle with strategy.
type ReportSnapshot struct {
	JurikValue      float64
	DivergenceScore int
	Regime          string
	FalconRedLine   float64
	FalconBlueLine  float64
	FibActive       bool
}

// VectorizeReport converts a market snapshot into a fixed-size normalized embedding.
func VectorizeReport(s ReportSnapshot) []float32 {
	return []float32{
		float32(s.JurikValue / 100.0),
		float32(s.DivergenceScore) / 100.0,
		regimeToFloat(s.Regime),
		float32(s.FalconRedLine / 100.0),
		float32(s.FalconBlueLine / 100.0),
		fibActiveToFloat(s.FibActive),
	}
}

func regimeToFloat(regime string) float32 {
	switch regime {
	case regimeExpansion:
		return 1.0
	case regimeSqueeze:
		return 0.0
	case regimeClimax:
		return -1.0
	default:
		return 0.0
	}
}

func fibActiveToFloat(active bool) float32 {
	if active {
		return 1.0
	}
	return 0.0
}
