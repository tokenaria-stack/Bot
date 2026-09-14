package forecast

import (
	"fmt"

	"trading_bot/ml"
)

// CausalTailSplit is one market-time inner train/val cut of a strictly
// increasing At[] (typically one outer-train population). Indexes are local
// to that At[].
type CausalTailSplit = ml.CausalTailSplit

// SplitCausalTail cuts a chronological inner validation tail from at[] using
// market-clock hops, not row subtraction. tailExclusiveEnd is the on-grid
// exclusive end of the population (typically NextBarOpen of the last At).
// It is model-agnostic.
func SplitCausalTail(at []int64, tf string, tailExclusiveEnd int64, spanBars, targetH, extraGapBars, minTrainRows int) (CausalTailSplit, error) {
	z, err := ml.SplitCausalTail(at, tf, tailExclusiveEnd, spanBars, targetH, extraGapBars, minTrainRows)
	if err != nil {
		return CausalTailSplit{}, fmt.Errorf("forecast: %v", err)
	}
	return z, nil
}
