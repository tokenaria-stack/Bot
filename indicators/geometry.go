package indicators

import "math"

const (
	touchATRMultiplier    = 0.5
	volumeBreakoutFactor  = 1.5
	triangleFlatSlopeEps  = 1e-9
	triangleConvergeMinDX = 1.0
)

// Trendline is a line through two ZigZag swing nodes (Peak index = x, Value = y).
type Trendline struct {
	P1           Peak
	P2           Peak
	IsResistance bool
	Touches      int
	m            float64
	b            float64
	valid        bool
}

// NewTrendline builds a trendline from two peaks (typically from ZigZag nodes).
func NewTrendline(p1, p2 Peak, isResistance bool) Trendline {
	tl := Trendline{
		P1:           p1,
		P2:           p2,
		IsResistance: isResistance,
	}
	tl.Equation()
	return tl
}

// Equation computes y = m*x + b where x is bar index and y is price.
func (t *Trendline) Equation() (m, b float64) {
	dx := float64(t.P2.Index - t.P1.Index)
	if dx == 0 {
		t.valid = false
		t.m = 0
		t.b = t.P1.Value
		return t.m, t.b
	}

	t.m = (t.P2.Value - t.P1.Value) / dx
	t.b = t.P1.Value - t.m*float64(t.P1.Index)
	t.valid = true
	return t.m, t.b
}

// ValueAt returns the trendline price at the given bar index.
func (t *Trendline) ValueAt(index int) float64 {
	if !t.valid {
		t.Equation()
	}
	return t.m*float64(index) + t.b
}

// UpdateTouches records a touch when price comes within 0.5*ATR of the line
// at barIndex without breaking through (resistance: no close above; support: no close below).
func (t *Trendline) UpdateTouches(barIndex int, high, low, atr float64) {
	if atr <= 0 {
		return
	}

	lineVal := t.ValueAt(barIndex)
	tolerance := touchATRMultiplier * atr

	if t.IsResistance {
		approached := high >= lineVal-tolerance && high <= lineVal
		if approached {
			t.Touches++
		}
		return
	}

	approached := low <= lineVal+tolerance && low >= lineVal
	if approached {
		t.Touches++
	}
}

// CheckBreakout detects a volume-confirmed breakout through the trendline.
// Returns ok and a strength score (higher when Touches were accumulated before breakout).
func (t *Trendline) CheckBreakout(currentIndex int, close, open, volume, avgVolume float64, isResistance bool) (bool, int) {
	if avgVolume <= 0 || volume <= avgVolume*volumeBreakoutFactor {
		return false, 0
	}

	lineVal := t.ValueAt(currentIndex)

	if isResistance {
		if close <= lineVal || close <= open {
			return false, 0
		}
	} else {
		if close >= lineVal || close >= open {
			return false, 0
		}
	}

	score := 1 + t.Touches
	if score < 1 {
		score = 1
	}
	return true, score
}

// DetectTriangle classifies converging resistance/support trendlines.
// Returns "symmetrical", "ascending", "descending", or "" if not a triangle.
func DetectTriangle(resistance, support Trendline) string {
	mR, bR := resistance.Equation()
	mS, bS := support.Equation()

	if !resistance.valid || !support.valid {
		return ""
	}

	dm := mR - mS
	if math.Abs(dm) < triangleFlatSlopeEps {
		return ""
	}

	intersectX := (bS - bR) / dm
	patternEnd := maxInt(resistance.P2.Index, support.P2.Index)
	if intersectX <= float64(patternEnd)+triangleConvergeMinDX {
		return ""
	}

	resFlat := math.Abs(mR) < triangleFlatSlopeEps
	supFlat := math.Abs(mS) < triangleFlatSlopeEps

	switch {
	case resFlat && mS > 0:
		return "ascending"
	case supFlat && mR < 0:
		return "descending"
	case mS > 0 && mR < 0:
		return "symmetrical"
	default:
		return ""
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
