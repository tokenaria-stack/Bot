package ml

import (
	"fmt"

	"trading_bot/data"
)

// CausalTailSplit is one market-time inner train/val cut. Indexes are local to At[].
type CausalTailSplit struct {
	InnerTrainBegin  int
	InnerTrainEnd    int
	InnerValBegin    int
	InnerValEnd      int
	InnerValStartAt  int64
	TailExclusiveEnd int64
	TrainLastAt      int64
}

func strictlyIncreasingAt(at []int64) error {
	if len(at) == 0 {
		return fmt.Errorf("ml: empty At[]")
	}
	for i := 1; i < len(at); i++ {
		if at[i] <= at[i-1] {
			return fmt.Errorf("ml: At not strictly increasing index=%d", i)
		}
	}
	return nil
}

// HorizonEnd is the open time of the last market bar in the H-bar path after At.
func HorizonEnd(at int64, timeframe string, bars int) (int64, error) {
	if bars < 0 {
		return 0, fmt.Errorf("ml: HorizonEnd bars must be >= 0")
	}
	t := at
	for i := 0; i < bars; i++ {
		next, err := data.NextBarOpen(t, timeframe)
		if err != nil {
			return 0, err
		}
		if next <= t {
			return 0, fmt.Errorf("ml: HorizonEnd NextBarOpen did not advance from %d", t)
		}
		t = next
	}
	return t, nil
}

// HopPrevOpen walks n previous native bars.
func HopPrevOpen(at int64, tf string, n int) (int64, error) {
	if n < 0 {
		return 0, fmt.Errorf("ml: hop count must be >= 0")
	}
	t := at
	for i := 0; i < n; i++ {
		p, err := data.PreviousBarOpen(t, tf)
		if err != nil {
			return 0, err
		}
		if p >= t {
			return 0, fmt.Errorf("ml: PreviousBarOpen did not go backward from %d", t)
		}
		t = p
	}
	return t, nil
}

// SplitCausalTail cuts a chronological inner validation tail using market-clock hops.
func SplitCausalTail(at []int64, tf string, tailExclusiveEnd int64, spanBars, targetH, extraGapBars, minTrainRows int) (CausalTailSplit, error) {
	var z CausalTailSplit
	if tf == "" {
		return z, fmt.Errorf("ml: SplitCausalTail timeframe required")
	}
	if spanBars <= 0 {
		return z, fmt.Errorf("ml: SplitCausalTail spanBars must be > 0")
	}
	if targetH < 0 || extraGapBars < 0 {
		return z, fmt.Errorf("ml: SplitCausalTail TargetH/ExtraGapBars must be >= 0")
	}
	if minTrainRows <= 0 {
		return z, fmt.Errorf("ml: SplitCausalTail minTrainRows must be > 0")
	}
	if err := strictlyIncreasingAt(at); err != nil {
		return z, err
	}
	open, err := data.CurrentBarOpen(tailExclusiveEnd, tf)
	if err != nil {
		return z, err
	}
	if open != tailExclusiveEnd {
		return z, fmt.Errorf("ml: tailExclusiveEnd %d is off the %s grid (floor %d)", tailExclusiveEnd, tf, open)
	}
	last := at[len(at)-1]
	if last >= tailExclusiveEnd {
		return z, fmt.Errorf("ml: last At %d is not < tailExclusiveEnd %d", last, tailExclusiveEnd)
	}
	wantEnd, err := data.NextBarOpen(last, tf)
	if err != nil {
		return z, err
	}
	if wantEnd != tailExclusiveEnd {
		return z, fmt.Errorf("ml: tailExclusiveEnd %d != NextBarOpen(last At %d)=%d", tailExclusiveEnd, last, wantEnd)
	}
	innerValStart, err := HopPrevOpen(tailExclusiveEnd, tf, spanBars)
	if err != nil {
		return z, err
	}
	if innerValStart >= tailExclusiveEnd {
		return z, fmt.Errorf("ml: empty inner validation market window")
	}
	causal := targetH + extraGapBars
	lb := func(b int64) int {
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
		return z, fmt.Errorf("ml: empty inner validation range")
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
		return z, fmt.Errorf("ml: empty inner train")
	}
	if trainEnd > v0 {
		return z, fmt.Errorf("ml: inner train overlaps inner val")
	}
	if trainEnd < minTrainRows {
		return z, fmt.Errorf("ml: INNER_TRAIN_TOO_SMALL train=%d min=%d", trainEnd, minTrainRows)
	}
	for i := 0; i < trainEnd; i++ {
		h, err := HorizonEnd(at[i], tf, causal)
		if err != nil {
			return z, err
		}
		if h >= innerValStart {
			return z, fmt.Errorf("ml: innerTrain At %d HorizonEnd=%d is not < innerValStart %d", at[i], h, innerValStart)
		}
	}
	if at[trainEnd-1] >= at[v0] {
		return z, fmt.Errorf("ml: innerTrain last At not < innerVal first At")
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
