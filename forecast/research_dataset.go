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

	if len(trows) != len(lrows) {
		return nil, z, fmt.Errorf("forecast: research dataset row count mismatch tape=%d labels=%d", len(trows), len(lrows))
	}
	for i := range trows {
		if trows[i].At != lrows[i].At {
			return nil, z, fmt.Errorf("forecast: research dataset At mismatch index=%d tape=%d label=%d", i, trows[i].At, lrows[i].At)
		}
	}

	acc := ResearchAccounting{
		TotalCandidates:  len(trows),
		ExcludedByReason: map[LabelReason]int{},
	}
	out := make([]ResearchRow, 0, len(trows))
	for i := range trows {
		tr, lr := trows[i], lrows[i]
		if !tr.Ready {
			acc.FeatureNotReady++
			continue
		}
		switch lr.Outcome {
		case OutcomeUpFirst:
			out = append(out, ResearchRow{At: tr.At, Features: cloneFeatureVector(tr.Values), Outcome: lr.Outcome})
			acc.TrainableUP++
		case OutcomeDownFirst:
			out = append(out, ResearchRow{At: tr.At, Features: cloneFeatureVector(tr.Values), Outcome: lr.Outcome})
			acc.TrainableDOWN++
		case OutcomeTimeout:
			out = append(out, ResearchRow{At: tr.At, Features: cloneFeatureVector(tr.Values), Outcome: lr.Outcome})
			acc.TrainableTIMEOUT++
		case OutcomeAmbiguous:
			acc.ExcludedByReason[lr.Reason]++
		default:
			return nil, z, fmt.Errorf("forecast: research dataset unrecognized outcome %q at %d", lr.Outcome, tr.At)
		}
	}
	acc.TrainableTotal = acc.TrainableUP + acc.TrainableDOWN + acc.TrainableTIMEOUT
	excluded := 0
	for _, n := range acc.ExcludedByReason {
		excluded += n
	}
	if acc.TotalCandidates != acc.FeatureNotReady+excluded+acc.TrainableTotal {
		return nil, z, fmt.Errorf("forecast: research dataset partition does not close: total=%d notReady=%d excluded=%d trainable=%d",
			acc.TotalCandidates, acc.FeatureNotReady, excluded, acc.TrainableTotal)
	}
	if len(out) != acc.TrainableTotal {
		return nil, z, fmt.Errorf("forecast: research dataset trainable len=%d accounting=%d", len(out), acc.TrainableTotal)
	}
	return out, acc, nil
}

func cloneFeatureVector(src []float64) []float64 {
	if src == nil {
		return nil
	}
	return append([]float64(nil), src...)
}
