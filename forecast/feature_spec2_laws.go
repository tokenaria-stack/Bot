package forecast

import "math"

const (
	Spec2RSXMidline         = 50.0
	Spec2MaxGapBars         = 8
	Spec2MaxSpanBars        = 8
	Spec2PatternActiveAge   = 36
	Spec2TVPivotAgePrimary  = 36
	Spec2SimpleCrossAgePrim = 12
	Spec2HTFFactAgeNative   = 12
	Spec2TargetHorizon      = 72
	Spec2UpperATR           = 2.0
	Spec2LowerATR           = 2.0
	Spec2RSXLength          = 14
	Spec2RSXSignal          = 14
	Spec2RSXSource          = "hlc3"
	Spec2TVLookback         = 90
	Spec2PrimaryTF          = "15m"
	Spec2HTF1h              = "1h"
	Spec2HTF4h              = "4h"
	Spec2FeatureWidth       = 64
	Spec2PrimaryWidth       = 42
	Spec2HTFWidth           = 11
)

// HorizonQuarterQ is H/4. Refuses if H is not divisible by 4.
func HorizonQuarterQ(h int) (int, error) {
	if h <= 0 || h%4 != 0 {
		return 0, errSpec2("H must be > 0 and divisible by 4")
	}
	return h / 4, nil
}

// SignalCrossUp / Down: closed-bar sign change, no epsilon.
func SignalCrossUp(dPrev, dNow float64) bool   { return dPrev <= 0 && dNow > 0 }
func SignalCrossDown(dPrev, dNow float64) bool { return dPrev >= 0 && dNow < 0 }

// Midline50CrossUp / Down: RSX vs 50.0, closed bars, no epsilon.
func Midline50CrossUp(prev, now float64) bool {
	return prev <= Spec2RSXMidline && now > Spec2RSXMidline
}
func Midline50CrossDown(prev, now float64) bool {
	return prev >= Spec2RSXMidline && now < Spec2RSXMidline
}

// PresentAge writes explicit absent 0/0 when age is outside [0, max].
func PresentAge(age, max int) (present, ageOut float64) {
	if age < 0 || age > max {
		return 0, 0
	}
	return 1, float64(age)
}

// NativePresentAge ages a fact in that source's native bar index (1h vs 4h vs 15m).
func NativePresentAge(candidateIdx, eventIdx, maxNativeAge int) (present, age float64) {
	if eventIdx < 0 || candidateIdx < eventIdx {
		return 0, 0
	}
	return PresentAge(candidateIdx-eventIdx, maxNativeAge)
}

// Spec2UnexpectedNativeGap is the FEATURE-SPEC-2 provenance failure for a broken
// established native series. Not NaN, not zero-fill, not stale HTF fallback.
func Spec2UnexpectedNativeGap(key MarketKey, detail string) error {
	return errSpec2("provenance: unexpected native gap " + key.String() + " " + detail)
}

// OrderedPatternOK: A < B, gap and span in primary bar index units (B-A).
func OrderedPatternOK(confirmedA, confirmedB int64, barIndexA, barIndexB int) bool {
	if !(confirmedA < confirmedB) {
		return false
	}
	if barIndexB <= barIndexA {
		return false
	}
	gap := barIndexB - barIndexA
	return gap <= Spec2MaxGapBars && gap <= Spec2MaxSpanBars
}

// SameBarConjunction: exact ConfirmedAt equality (not gap<=0 sequences).
func SameBarConjunction(confirmedA, confirmedB int64) bool {
	return confirmedA != 0 && confirmedA == confirmedB
}

// LatestCompletionAge: among completions with age<=max, the latest ConfirmedAt owns age.
func LatestCompletionAge(candidateIdx int, completionIdx []int, maxAge int) (present, age float64) {
	best := -1
	bestAge := maxAge + 1
	for _, c := range completionIdx {
		if c < 0 || c > candidateIdx {
			continue
		}
		a := candidateIdx - c
		if a < 0 || a > maxAge {
			continue
		}
		if c > best || (c == best && a < bestAge) {
			best = c
			bestAge = a
		}
	}
	if best < 0 {
		return 0, 0
	}
	return PresentAge(bestAge, maxAge)
}

func PriceDisplacementATR(closeT, closeLag, atrT float64) (float64, Ready) {
	if !(atrT > 0) || math.IsNaN(closeT) || math.IsNaN(closeLag) || math.IsNaN(atrT) || math.IsInf(atrT, 0) {
		return 0, NotReady
	}
	return (closeT - closeLag) / atrT, IsReady
}

// PriceRangePositionH: window [t-H+1 ... t] inclusive (H bars). HH>LL required.
func PriceRangePositionH(high, low, close []float64, t, h int) (float64, Ready) {
	if h <= 0 || t-h+1 < 0 || t >= len(close) || t >= len(high) || t >= len(low) {
		return 0, NotReady
	}
	hh, ll := high[t-h+1], low[t-h+1]
	for i := t - h + 1; i <= t; i++ {
		if high[i] > hh {
			hh = high[i]
		}
		if low[i] < ll {
			ll = low[i]
		}
	}
	if !(hh > ll) {
		return 0, NotReady
	}
	return (close[t] - ll) / (hh - ll), IsReady
}

// PricePathEfficiencyH: H intervals i=t-H+1..t, needs Close[t-H].
func PricePathEfficiencyH(close []float64, t, h int) (float64, Ready) {
	if h <= 0 || t-h < 0 || t >= len(close) {
		return 0, NotReady
	}
	num := math.Abs(close[t] - close[t-h])
	den := 0.0
	for i := t - h + 1; i <= t; i++ {
		den += math.Abs(close[i] - close[i-1])
	}
	if !(den > 0) {
		return 0, NotReady
	}
	return num / den, IsReady
}

func ATROverPrice(atrT, closeT float64) (float64, Ready) {
	if !(closeT > 0) || math.IsNaN(atrT) || math.IsInf(atrT, 0) {
		return 0, NotReady
	}
	return atrT / closeT, IsReady
}

func ATRChangeQ(atrT, atrLag float64) (float64, Ready) {
	if !(atrLag > 0) || math.IsNaN(atrT) || math.IsInf(atrT, 0) {
		return 0, NotReady
	}
	return atrT/atrLag - 1, IsReady
}
