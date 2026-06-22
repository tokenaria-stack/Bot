package indicators

import "math"

const priceEqualTolerance = 0.001 // 0.1% relative tolerance for Class B double top/bottom

// DivDirection classifies divergence direction.
type DivDirection string

const (
	Bullish DivDirection = "BULLISH"
	Bearish DivDirection = "BEARISH"
	NoDiv   DivDirection = "NONE"
)

// DivClass classifies divergence strength (Islands + window method taxonomy).
type DivClass string

const (
	ClassA DivClass = "CLASS_A" // Strongest: new price extreme, weak oscillator
	ClassB DivClass = "CLASS_B" // Price double top/bottom, weak oscillator
	ClassC DivClass = "CLASS_C" // New price extreme, oscillator double top/bottom
	None   DivClass = "NONE"
)

// DivergenceResult holds a detected classic divergence between price and oscillator peaks.
type DivergenceResult struct {
	Direction DivDirection
	Class     DivClass
	PriceP1   Peak
	PriceP2   Peak
	OscP1     Peak
	OscP2     Peak
}

// CheckClassicDivergence compares the last two aligned price/oscillator peaks for classic divergence.
func CheckClassicDivergence(pricePeaks, oscPeaks []Peak, indexTolerance int) DivergenceResult {
	empty := DivergenceResult{Direction: NoDiv, Class: None}

	if len(pricePeaks) < 2 || len(oscPeaks) < 2 {
		return empty
	}

	priceP2 := pricePeaks[len(pricePeaks)-1]
	priceP1, ok := previousPeakOfType(pricePeaks, priceP2.Type)
	if !ok {
		return empty
	}

	oscP2, ok := findMatchingPeak(priceP2, oscPeaks, indexTolerance)
	if !ok {
		return empty
	}

	oscP1, ok := findMatchingPeak(priceP1, oscPeaks, indexTolerance)
	if !ok {
		return empty
	}

	result := DivergenceResult{
		PriceP1: priceP1,
		PriceP2: priceP2,
		OscP1:   oscP1,
		OscP2:   oscP2,
	}

	switch priceP2.Type {
	case PeakHigh:
		switch {
		case priceP2.Value > priceP1.Value && oscP2.Value < oscP1.Value:
			result.Direction = Bearish
			result.Class = ClassA
			return result
		case pricesApproximatelyEqual(priceP2.Value, priceP1.Value) && oscP2.Value < oscP1.Value:
			result.Direction = Bearish
			result.Class = ClassB
			return result
		}
	case PeakLow:
		switch {
		case priceP2.Value < priceP1.Value && oscP2.Value > oscP1.Value:
			result.Direction = Bullish
			result.Class = ClassA
			return result
		case pricesApproximatelyEqual(priceP2.Value, priceP1.Value) && oscP2.Value > oscP1.Value:
			result.Direction = Bullish
			result.Class = ClassB
			return result
		}
	}

	return empty
}

// CheckTripleDivergence detects a three-swing exhaustion pattern between price and oscillator peaks.
func CheckTripleDivergence(pricePeaks, oscPeaks []Peak, indexTolerance int) DivergenceResult {
	empty := DivergenceResult{Direction: NoDiv, Class: None}

	if len(pricePeaks) < 3 || len(oscPeaks) < 3 {
		return empty
	}

	p3 := pricePeaks[len(pricePeaks)-1]
	p2 := pricePeaks[len(pricePeaks)-2]
	p1 := pricePeaks[len(pricePeaks)-3]

	if p3.Type != p2.Type || p2.Type != p1.Type {
		return empty
	}

	oscP3, ok := findMatchingPeak(p3, oscPeaks, indexTolerance)
	if !ok {
		return empty
	}

	oscP2, ok := findMatchingPeak(p2, oscPeaks, indexTolerance)
	if !ok {
		return empty
	}

	oscP1, ok := findMatchingPeak(p1, oscPeaks, indexTolerance)
	if !ok {
		return empty
	}

	switch p3.Type {
	case PeakHigh:
		if p3.Value > p2.Value && p2.Value > p1.Value &&
			oscP3.Value < oscP2.Value && oscP2.Value < oscP1.Value {
			return DivergenceResult{
				Direction: Bearish,
				Class:     ClassA,
				PriceP1:   p1,
				PriceP2:   p3,
				OscP1:     oscP1,
				OscP2:     oscP3,
			}
		}
	case PeakLow:
		if p3.Value < p2.Value && p2.Value < p1.Value &&
			oscP3.Value > oscP2.Value && oscP2.Value > oscP1.Value {
			return DivergenceResult{
				Direction: Bullish,
				Class:     ClassA,
				PriceP1:   p1,
				PriceP2:   p3,
				OscP1:     oscP1,
				OscP2:     oscP3,
			}
		}
	}

	return empty
}

func previousPeakOfType(peaks []Peak, peakType PeakType) (Peak, bool) {
	for i := len(peaks) - 2; i >= 0; i-- {
		if peaks[i].Type == peakType {
			return peaks[i], true
		}
	}
	return Peak{}, false
}

func findMatchingPeak(pricePeak Peak, oscPeaks []Peak, indexTolerance int) (Peak, bool) {
	var (
		best    Peak
		found   bool
		minDist = math.MaxFloat64
	)

	for _, oscPeak := range oscPeaks {
		if oscPeak.Type != pricePeak.Type {
			continue
		}

		dist := math.Abs(float64(pricePeak.Index - oscPeak.Index))
		if dist > float64(indexTolerance) {
			continue
		}

		if dist < minDist {
			minDist = dist
			best = oscPeak
			found = true
		}
	}

	return best, found
}

func pricesApproximatelyEqual(a, b float64) bool {
	if a == 0 && b == 0 {
		return true
	}

	base := math.Max(math.Abs(a), math.Abs(b))
	return math.Abs(a-b) <= base*priceEqualTolerance
}
