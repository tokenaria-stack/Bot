package indicators

import (
	"errors"
	"math"
)

// PeakType classifies an extreme as a local high or low.
type PeakType string

const (
	PeakHigh PeakType = "HIGH"
	PeakLow  PeakType = "LOW"
)

// Peak holds the coordinates of a detected extreme in a numeric series.
type Peak struct {
	Index int      // Candle or sample index in the source array
	Value float64  // Price, AO, RSI, etc.
	Type  PeakType // HIGH or LOW
}

// FindExtremes detects local highs and lows using a symmetric sliding window.
// Complexity: O(N * W), where N = len(data) and W = window.
func FindExtremes(data []float64, window int) ([]Peak, error) {
	if len(data) < window*2+1 {
		return nil, errors.New("not enough data for the given window size")
	}

	var peaks []Peak

	for i := window; i < len(data)-window; i++ {
		isHigh := true
		isLow := true
		currentValue := data[i]

		for j := i - window; j <= i+window; j++ {
			if i == j {
				continue
			}
			if data[j] >= currentValue {
				isHigh = false
			}
			if data[j] <= currentValue {
				isLow = false
			}
		}

		if isHigh {
			peaks = append(peaks, Peak{
				Index: i,
				Value: currentValue,
				Type:  PeakHigh,
			})
		}
		if isLow {
			peaks = append(peaks, Peak{
				Index: i,
				Value: currentValue,
				Type:  PeakLow,
			})
		}
	}

	return peaks, nil
}

// FilterPeaksByATR keeps peaks whose amplitude exceeds local noise measured by ATR.
func FilterPeaksByATR(data []float64, peaks []Peak, atrValues []float64, window int, multiplier float64) []Peak {
	var filtered []Peak

	for _, p := range peaks {
		if p.Index < 0 || p.Index >= len(data) || p.Index >= len(atrValues) {
			continue
		}

		start := p.Index - window
		if start < 0 {
			start = 0
		}

		end := p.Index + window
		if end >= len(data) {
			end = len(data) - 1
		}

		localExtremum := data[start]
		for i := start + 1; i <= end; i++ {
			switch p.Type {
			case PeakHigh:
				if data[i] < localExtremum {
					localExtremum = data[i]
				}
			case PeakLow:
				if data[i] > localExtremum {
					localExtremum = data[i]
				}
			}
		}

		amplitude := math.Abs(p.Value - localExtremum)
		if amplitude >= atrValues[p.Index]*multiplier {
			filtered = append(filtered, p)
		}
	}

	return filtered
}
