package forecast

import "fmt"

// CausalPriceSwing is an OHLC Williams pivot. ConfirmedAt is the first closed
// right-side bar; it must be <= candidate At to be usable. RSX is not consulted.
type CausalPriceSwing struct {
	K           int     `json:"k"`
	AnchorAt    int64   `json:"anchor_at,omitempty"`
	ConfirmedAt int64   `json:"confirmed_at,omitempty"`
	Wick        float64 `json:"wick,omitempty"`
	Found       bool    `json:"found"`
	InvalidGeo  bool    `json:"invalid_geometry,omitempty"`
}

func uniqueExtreme(bars []CanonicalClosedBar, t, k int, long bool) bool {
	if k < 1 || t < k || t+k >= len(bars) {
		return false
	}
	if long {
		v := bars[t].Low
		if !isFinite(v) {
			return false
		}
		for i := t - k; i <= t+k; i++ {
			if i == t {
				continue
			}
			if !isFinite(bars[i].Low) || bars[i].Low <= v {
				return false
			}
		}
		return true
	}
	v := bars[t].High
	if !isFinite(v) {
		return false
	}
	for i := t - k; i <= t+k; i++ {
		if i == t {
			continue
		}
		if !isFinite(bars[i].High) || bars[i].High >= v {
			return false
		}
	}
	return true
}

func entryIndex(bars []CanonicalClosedBar, entryAt int64) (int, error) {
	for i := range bars {
		if bars[i].OpenTime == entryAt {
			return i, nil
		}
	}
	return -1, fmt.Errorf("forecast: price swing entry At %d not on primary", entryAt)
}

func swingFromIndex(bars []CanonicalClosedBar, t, k, entry int, long bool) CausalPriceSwing {
	z := CausalPriceSwing{K: k, Found: true, AnchorAt: bars[t].OpenTime, ConfirmedAt: bars[t+k].OpenTime}
	if long {
		z.Wick = bars[t].Low
	} else {
		z.Wick = bars[t].High
	}
	entryPx := bars[entry].Close
	if (long && !(z.Wick < entryPx)) || (!long && !(z.Wick > entryPx)) {
		z.InvalidGeo = true
	}
	return z
}

func causalSwingTimes(bars []CanonicalClosedBar, entry, k int, long bool) []int {
	var ts []int
	if k < 1 || entry < 0 {
		return ts
	}
	for t := k; t <= entry-k; t++ {
		if !uniqueExtreme(bars, t, k, long) {
			continue
		}
		if bars[t+k].OpenTime > bars[entry].OpenTime {
			continue
		}
		ts = append(ts, t)
	}
	return ts
}

// LatestCausalPriceSwing returns the latest unique OHLC swing whose right-radius
// k bars have already closed at or before entryAt.
func LatestCausalPriceSwing(bars []CanonicalClosedBar, entryAt int64, k int, long bool) (CausalPriceSwing, error) {
	z := CausalPriceSwing{K: k}
	if k < 1 {
		return z, fmt.Errorf("forecast: price swing k must be >= 1")
	}
	entry, err := entryIndex(bars, entryAt)
	if err != nil {
		return z, err
	}
	ts := causalSwingTimes(bars, entry, k, long)
	if len(ts) == 0 {
		return z, nil
	}
	return swingFromIndex(bars, ts[len(ts)-1], k, entry, long), nil
}
