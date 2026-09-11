package forecast

import (
	"fmt"
)

// ResearchRow is one trainable candidate: copied FeaturePlan vector + outcome.
// Exclusion metadata stays on ResearchAccounting, not on this row.
type ResearchRow struct {
	At       int64
	Features []float64
	Outcome  TargetOutcome
}

// ResearchDatasetExpect is the CURRENT intended research world for one
// consumption call. Digests are computed by the caller now — never pasted
// from a report. forecast does not import market; the caller supplies
// ResearchMarketKey / FeaturePlan / TargetSpec identities.
type ResearchDatasetExpect struct {
	Market     MarketKey
	Plan       Digest
	Target     Digest
	MinFirstAt int64 // 0 skips the floor (synthetic fixtures only)
}

// ResearchAccounting is the mutually exclusive partition of joined candidates.
type ResearchAccounting struct {
	TotalCandidates  int
	FeatureNotReady  int
	ExcludedTotal    int
	ExcludedByReason map[LabelReason]int
	TrainableUP      int
	TrainableDOWN    int
	TrainableTIMEOUT int
	TrainableTotal   int
}

// BuildResearchDataset opens both artifacts, enforces consumption provenance,
// locksteps At, then partitions eligibility. It never generates labels or
// writes a dataset file.
func BuildResearchDataset(tapePath, labelPath string, expect ResearchDatasetExpect) ([]ResearchRow, ResearchAccounting, error) {
	var z ResearchAccounting
	if err := expect.Market.Validate(); err != nil {
		return nil, z, fmt.Errorf("forecast: research dataset MarketKey: %w", err)
	}
	var zero Digest
	if expect.Plan == zero {
		return nil, z, fmt.Errorf("forecast: research dataset requires current FeaturePlan digest")
	}
	if expect.Target == zero {
		return nil, z, fmt.Errorf("forecast: research dataset requires current TargetDigest")
	}

	th, trows, tf, err := ReadTape(tapePath, &expect.Market, &expect.Plan)
	if err != nil {
		return nil, z, err
	}
	if expect.MinFirstAt > 0 && tf.FirstAt < expect.MinFirstAt {
		return nil, z, fmt.Errorf("forecast: FeatureTape FirstAt %d < research floor %d", tf.FirstAt, expect.MinFirstAt)
	}

	lh, lrows, _, err := ReadLabelSet(labelPath, nil)
	if err != nil {
		return nil, z, err
	}
	if lh.FeatureTapePlanDigest != th.PlanDigest {
		return nil, z, fmt.Errorf("forecast: LabelSet FeatureTape PlanDigest mismatch")
	}
	if lh.FeatureTapeSourceRangeDigest != tf.SourceRangeDigest {
		return nil, z, fmt.Errorf("forecast: LabelSet FeatureTape SourceRangeDigest mismatch")
	}
	if lh.FeatureTapeContentDigest != tf.ContentDigest {
		return nil, z, fmt.Errorf("forecast: LabelSet FeatureTape ContentDigest mismatch")
	}
	if lh.Market != th.Market {
		return nil, z, fmt.Errorf("forecast: LabelSet PrimaryMarketKey mismatch")
	}
	if lh.TargetDigest != expect.Target {
		return nil, z, fmt.Errorf("forecast: LabelSet TargetDigest mismatch")
	}

	tapeAts := make([]int64, len(trows))
	ready := make([]bool, len(trows))
	for i := range trows {
		tapeAts[i] = trows[i].At
		ready[i] = bool(trows[i].Ready)
	}
	labelAts := make([]int64, len(lrows))
	for i := range lrows {
		labelAts[i] = lrows[i].At
	}
	if err := refuseDatasetLockstep(tapeAts, labelAts); err != nil {
		return nil, z, err
	}
	keep, acc, err := partitionResearchPopulation(ready, lrows)
	if err != nil {
		return nil, z, err
	}
	out := make([]ResearchRow, 0, len(keep))
	for _, i := range keep {
		out = append(out, ResearchRow{At: trows[i].At, Features: cloneFeatureVector(trows[i].Values), Outcome: lrows[i].Outcome})
	}
	if len(out) != acc.TrainableTotal {
		return nil, z, fmt.Errorf("forecast: research dataset trainable len=%d accounting=%d", len(out), acc.TrainableTotal)
	}
	return out, acc, nil
}

func refuseDatasetLockstep(tapeAts, labelAts []int64) error {
	if len(tapeAts) != len(labelAts) {
		return fmt.Errorf("forecast: research dataset row count mismatch tape=%d labels=%d", len(tapeAts), len(labelAts))
	}
	for i := range tapeAts {
		if tapeAts[i] != labelAts[i] {
			return fmt.Errorf("forecast: research dataset At mismatch index=%d tape=%d label=%d", i, tapeAts[i], labelAts[i])
		}
	}
	return nil
}

// partitionResearchPopulation is the tape-format-agnostic exclusive split.
// Ready=false is FeatureNotReady even when the label is otherwise legal.
func partitionResearchPopulation(ready []bool, labels []LabelRow) ([]int, ResearchAccounting, error) {
	var z ResearchAccounting
	if len(ready) != len(labels) {
		return nil, z, fmt.Errorf("forecast: research dataset row count mismatch tape=%d labels=%d", len(ready), len(labels))
	}
	acc := ResearchAccounting{
		TotalCandidates:  len(ready),
		ExcludedByReason: map[LabelReason]int{},
	}
	keep := make([]int, 0, len(ready))
	for i := range ready {
		if !ready[i] {
			acc.FeatureNotReady++
			continue
		}
		switch labels[i].Outcome {
		case OutcomeUpFirst:
			keep = append(keep, i)
			acc.TrainableUP++
		case OutcomeDownFirst:
			keep = append(keep, i)
			acc.TrainableDOWN++
		case OutcomeTimeout:
			keep = append(keep, i)
			acc.TrainableTIMEOUT++
		case OutcomeAmbiguous:
			acc.ExcludedByReason[labels[i].Reason]++
		default:
			return nil, z, fmt.Errorf("forecast: research dataset unrecognized outcome %q at %d", labels[i].Outcome, labels[i].At)
		}
	}
	acc.TrainableTotal = acc.TrainableUP + acc.TrainableDOWN + acc.TrainableTIMEOUT
	for _, n := range acc.ExcludedByReason {
		acc.ExcludedTotal += n
	}
	if acc.TotalCandidates != acc.FeatureNotReady+acc.ExcludedTotal+acc.TrainableTotal {
		return nil, z, fmt.Errorf("forecast: research dataset partition does not close: total=%d notReady=%d excluded=%d trainable=%d",
			acc.TotalCandidates, acc.FeatureNotReady, acc.ExcludedTotal, acc.TrainableTotal)
	}
	return keep, acc, nil
}

func cloneFeatureVector(src []float64) []float64 {
	if src == nil {
		return nil
	}
	return append([]float64(nil), src...)
}
