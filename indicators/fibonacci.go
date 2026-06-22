package indicators

import "math"

const (
	priceATRPaddingMult = 0.5
	timeZoneBarPadding  = 2
)

// FibZoneType classifies a Fibonacci kill zone.
type FibZoneType string

const (
	Retracement FibZoneType = "RETRACEMENT"
	Extension   FibZoneType = "EXTENSION"
	TimeZone    FibZoneType = "TIMEZONE"
)

// FibZone is a price or time target band with ATR/bar padding and active-state flag.
type FibZone struct {
	Type        FibZoneType
	Ratio       float64
	TargetValue float64 // price level or bar index
	UpperBound  float64
	LowerBound  float64
	IsActive    bool
}

var (
	retracementRatios = []float64{0.382, 0.5, 0.618, 0.786}
	extensionRatios   = []float64{1.272, 1.618, 2.618}
	timeZoneRatios    = []float64{1.0, 1.618, 2.618, 4.236}
)

// FibonacciEngine computes dynamic Fibonacci kill zones with ATR padding.
type FibonacciEngine struct{}

// NewFibonacciEngine creates a FibonacciEngine.
func NewFibonacciEngine() *FibonacciEngine {
	return &FibonacciEngine{}
}

// CalculatePriceZones builds retracement and extension kill zones between startPrice and endPrice.
// Retracements pull back from endPrice; extensions project from endPrice in the trend direction.
func (e *FibonacciEngine) CalculatePriceZones(startPrice, endPrice, currentPrice, currentATR float64) []FibZone {
	diff := endPrice - startPrice
	padding := currentATR * priceATRPaddingMult

	zones := make([]FibZone, 0, len(retracementRatios)+len(extensionRatios))
	for _, ratio := range retracementRatios {
		target := endPrice - diff*ratio
		zones = append(zones, newPriceZone(Retracement, ratio, target, padding, currentPrice))
	}
	for _, ratio := range extensionRatios {
		target := endPrice + diff*(ratio-1)
		zones = append(zones, newPriceZone(Extension, ratio, target, padding, currentPrice))
	}
	return zones
}

// CalculateTimeZones projects Fibonacci time targets forward from endIndex based on wave length.
func (e *FibonacciEngine) CalculateTimeZones(startIndex, endIndex, currentIndex int) []FibZone {
	diff := endIndex - startIndex
	zones := make([]FibZone, len(timeZoneRatios))

	for i, ratio := range timeZoneRatios {
		target := endIndex + int(float64(diff)*ratio)
		lower := float64(target - timeZoneBarPadding)
		upper := float64(target + timeZoneBarPadding)
		cur := float64(currentIndex)

		zones[i] = FibZone{
			Type:        TimeZone,
			Ratio:       ratio,
			TargetValue: float64(target),
			LowerBound:  lower,
			UpperBound:  upper,
			IsActive:    cur >= lower && cur <= upper,
		}
	}
	return zones
}

// FindConfluence returns merged kill zones where targets from two sets align within maxDeviation.
func FindConfluence(zonesA, zonesB []FibZone, maxDeviation float64) []FibZone {
	if maxDeviation < 0 {
		maxDeviation = 0
	}

	out := make([]FibZone, 0)
	for _, a := range zonesA {
		for _, b := range zonesB {
			if math.Abs(a.TargetValue-b.TargetValue) > maxDeviation {
				continue
			}
			out = append(out, mergeConfluenceZone(a, b))
		}
	}
	return out
}

func newPriceZone(zoneType FibZoneType, ratio, target, padding, currentPrice float64) FibZone {
	lower := target - padding
	upper := target + padding
	return FibZone{
		Type:        zoneType,
		Ratio:       ratio,
		TargetValue: target,
		LowerBound:  lower,
		UpperBound:  upper,
		IsActive:    currentPrice >= lower && currentPrice <= upper,
	}
}

func mergeConfluenceZone(a, b FibZone) FibZone {
	target := (a.TargetValue + b.TargetValue) / 2
	lower := math.Min(a.LowerBound, b.LowerBound)
	upper := math.Max(a.UpperBound, b.UpperBound)

	zoneType := a.Type
	if a.Type != b.Type {
		zoneType = Retracement
	}

	return FibZone{
		Type:        zoneType,
		Ratio:       a.Ratio,
		TargetValue: target,
		LowerBound:  lower,
		UpperBound:  upper,
		IsActive:    a.IsActive || b.IsActive,
	}
}
