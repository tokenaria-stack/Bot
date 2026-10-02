package starlens

import "fmt"

// HistBin is one display interval over a NumberPicture.
// Intervals are half-open [From, To). The first bin is open below
// and the last bin is open above when more than one bin exists.
// These bins are not stored on a population.
type HistBin struct {
	From     float64 `json:"from"`
	To       float64 `json:"to"`
	Count    int     `json:"count"`
	OpenLow  bool    `json:"openLow,omitempty"`
	OpenHigh bool    `json:"openHigh,omitempty"`
}

// DisplayBinCount is presentation only. It is not a research constant.
func DisplayBinCount(nObs int) int {
	if nObs <= 1 {
		return 1
	}
	if nObs < 80 {
		return nObs
	}
	return 64
}

// CountHalfOpen counts values into (-inf, cuts[0)), [cuts[i], cuts[i+1)), [cuts[last], +inf).
// CountBins remains the original right-closed counter. Display histograms use this
// so a boundary value belongs to exactly one [from, to) bin.
func CountHalfOpen(values, cuts []float64) ([]int, error) {
	if len(cuts) == 0 {
		return nil, fmt.Errorf("starlens: histogram cuts are empty")
	}
	for i := 1; i < len(cuts); i++ {
		if !(cuts[i] > cuts[i-1]) {
			return nil, fmt.Errorf("starlens: histogram cuts are not increasing")
		}
	}
	bins := make([]int, len(cuts)+1)
	for _, v := range values {
		placed := false
		for i, cut := range cuts {
			if v < cut {
				bins[i]++
				placed = true
				break
			}
		}
		if !placed {
			bins[len(cuts)]++
		}
	}
	return bins, nil
}

// DisplayHistogram bins NumberPicture values for the Lens wire.
func DisplayHistogram(values []float64) ([]HistBin, error) {
	if len(values) == 0 {
		return nil, nil
	}
	lo, hi := values[0], values[0]
	for _, v := range values[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	if lo == hi {
		return []HistBin{{From: lo, To: hi, Count: len(values), OpenHigh: true}}, nil
	}
	k := DisplayBinCount(len(values))
	if k < 2 {
		k = 2
	}
	step := (hi - lo) / float64(k)
	cuts := make([]float64, k-1)
	for i := 0; i < k-1; i++ {
		cuts[i] = lo + float64(i+1)*step
	}
	counts, err := CountHalfOpen(values, cuts)
	if err != nil {
		return nil, err
	}
	out := make([]HistBin, k)
	for i := 0; i < k; i++ {
		from := lo
		to := hi
		if i > 0 {
			from = cuts[i-1]
		}
		if i < k-1 {
			to = cuts[i]
		}
		out[i] = HistBin{From: from, To: to, Count: counts[i]}
	}
	out[0].OpenLow = true
	out[k-1].OpenHigh = true
	return out, nil
}
