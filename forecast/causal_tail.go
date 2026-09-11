package forecast

import (
	"fmt"

	"trading_bot/data"
)

// CausalTailSplit is one market-time inner train/val cut of a strictly
// increasing At[] (typically one outer-train population). Indexes are local
// to that At[].
type CausalTailSplit struct {
	InnerTrainBegin  int
	InnerTrainEnd    int
	InnerValBegin    int
	InnerValEnd      int
	InnerValStartAt  int64
	TailExclusiveEnd int64
	TrainLastAt      int64
}

// SplitCausalTail cuts a chronological inner validation tail from at[] using
// market-clock hops, not row subtraction. tailExclusiveEnd is the on-grid
// exclusive end of the population (typically NextBarOpen of the last At).
// It is model-agnostic.
func SplitCausalTail(at []int64, tf string, tailExclusiveEnd int64, spanBars, targetH, extraGapBars, minTrainRows int) (CausalTailSplit, error) {
	var z CausalTailSplit
	if tf == "" {
		return z, fmt.Errorf("forecast: SplitCausalTail timeframe required")
	}
	if spanBars <= 0 {
		return z, fmt.Errorf("forecast: SplitCausalTail spanBars must be > 0")
	}
	if targetH < 0 || extraGapBars < 0 {
		return z, fmt.Errorf("forecast: SplitCausalTail TargetH/ExtraGapBars must be >= 0")
	}
	if minTrainRows <= 0 {
		return z, fmt.Errorf("forecast: SplitCausalTail minTrainRows must be > 0")
	}
	if err := requireStrictlyIncreasingAt(at); err != nil {
		return z, err
	}
	if len(at) == 0 {
		return z, fmt.Errorf("forecast: SplitCausalTail empty At[]")
	}
	open, err := data.CurrentBarOpen(tailExclusiveEnd, tf)
	if err != nil {
		return z, err
	}
	if open != tailExclusiveEnd {
		return z, fmt.Errorf("forecast: tailExclusiveEnd %d is off the %s grid (floor %d)", tailExclusiveEnd, tf, open)
	}
	last := at[len(at)-1]
	if last >= tailExclusiveEnd {
		return z, fmt.Errorf("forecast: last At %d is not < tailExclusiveEnd %d", last, tailExclusiveEnd)
	}
	wantEnd, err := data.NextBarOpen(last, tf)
	if err != nil {
		return z, err
	}
	if wantEnd != tailExclusiveEnd {
		return z, fmt.Errorf("forecast: tailExclusiveEnd %d != NextBarOpen(last At %d)=%d", tailExclusiveEnd, last, wantEnd)
	}
	innerValStart, err := hopPrevOpen(tailExclusiveEnd, tf, spanBars)
	if err != nil {
		return z, err
	}
	if innerValStart >= tailExclusiveEnd {
		return z, fmt.Errorf("forecast: empty inner validation market window")
	}
	causal := targetH + extraGapBars
	lb := func(b int64) int {
		// first i with at[i] >= b
		lo, hi := 0, len(at)
		for lo < hi {
			mid := (lo + hi) / 2
			if at[mid] >= b {
				hi = mid
			} else {
				lo = mid + 1
			}
		}
		return lo
	}
	v0, v1 := lb(innerValStart), lb(tailExclusiveEnd)
	if v1 <= v0 {
		return z, fmt.Errorf("forecast: empty inner validation range")
	}
	lo, hi := 0, v0
	for lo < hi {
		mid := (lo + hi) / 2
		h, err := HorizonEnd(at[mid], tf, causal)
		if err != nil {
			return z, err
		}
		if h < innerValStart {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	trainEnd := lo
	if trainEnd <= 0 {
		return z, fmt.Errorf("forecast: empty inner train")
	}
	if trainEnd > v0 {
		return z, fmt.Errorf("forecast: inner train overlaps inner val")
	}
	if trainEnd-0 < minTrainRows {
		return z, fmt.Errorf("forecast: INNER_TRAIN_TOO_SMALL train=%d min=%d", trainEnd, minTrainRows)
	}
	for i := 0; i < trainEnd; i++ {
		h, err := HorizonEnd(at[i], tf, causal)
		if err != nil {
			return z, err
		}
		if h >= innerValStart {
			return z, fmt.Errorf("forecast: innerTrain At %d HorizonEnd=%d is not < innerValStart %d", at[i], h, innerValStart)
		}
	}
	if at[trainEnd-1] >= at[v0] {
		return z, fmt.Errorf("forecast: innerTrain last At not < innerVal first At")
	}
	return CausalTailSplit{
		InnerTrainBegin:  0,
		InnerTrainEnd:    trainEnd,
		InnerValBegin:    v0,
		InnerValEnd:      v1,
		InnerValStartAt:  innerValStart,
		TailExclusiveEnd: tailExclusiveEnd,
		TrainLastAt:      last,
	}, nil
}
