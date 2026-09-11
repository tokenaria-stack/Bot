package forecast

import "fmt"

// ResearchRow2 is one Brain-V2 trainable candidate. Features stay FeatureVector2.
type ResearchRow2 struct {
	At       int64
	Features FeatureVector2
	Outcome  TargetOutcome
}

// ResearchDataset2Expect is optional extra identity the caller already knows.
// FeatureSpec2 is the required Brain-V2 world; this does not replace it.
type ResearchDataset2Expect struct {
	MinFirstAt   int64
	TapeContent  *Digest
	LabelContent *Digest
}

// BuildResearchDatasetFromTape2 joins feature-tape-v2 + LabelSet-C in memory.
// Native V2 door — does not ReadTape (v1) and does not write a dataset file.
func BuildResearchDatasetFromTape2(tape2Path, labelPath string, spec2 FeatureSpec2, expect ResearchDataset2Expect) ([]ResearchRow2, ResearchAccounting, error) {
	var z ResearchAccounting
	if err := spec2.Primary.Validate(); err != nil {
		return nil, z, fmt.Errorf("forecast: research dataset-c MarketKey: %w", err)
	}
	tid, err := spec2.Target.Identity()
	if err != nil {
		return nil, z, err
	}
	pid, err := spec2.Plan.Identity()
	if err != nil {
		return nil, z, err
	}

	th, trows, tf, err := ReadTape2(tape2Path)
	if err != nil {
		return nil, z, err
	}
	if err := matchTape2ToSpec2(th, spec2); err != nil {
		return nil, z, err
	}
	if expect.MinFirstAt > 0 && tf.FirstAt < expect.MinFirstAt {
		return nil, z, fmt.Errorf("forecast: FeatureTape FirstAt %d < research floor %d", tf.FirstAt, expect.MinFirstAt)
	}
	if expect.TapeContent != nil && *expect.TapeContent != tf.ContentDigest {
		return nil, z, fmt.Errorf("forecast: FeatureTape ContentDigest mismatch")
	}

	labExpect := &LabelExpect{
		Market:      &th.Primary,
		Target:      &tid.Digest,
		TapePlan:    &pid.Digest,
		TapeSource:  &th.PrimarySource,
		TapeContent: &tf.ContentDigest,
	}
	lh, lrows, lf, err := ReadLabelSet(labelPath, labExpect)
	if err != nil {
		return nil, z, err
	}
	if expect.LabelContent != nil && *expect.LabelContent != lf.ContentDigest {
		return nil, z, fmt.Errorf("forecast: LabelSet ContentDigest mismatch")
	}
	if lh.FormatVersion != LabelSetFormatV2 {
		return nil, z, fmt.Errorf("forecast: research dataset-c requires %s", LabelSetFormatV2)
	}
	if lh.LabelLogicVersion != LabelLogicFirstPassageFinerV1 {
		return nil, z, fmt.Errorf("forecast: research dataset-c LabelLogicVersion %q", lh.LabelLogicVersion)
	}
	if lh.TargetDigest != tid.Digest {
		return nil, z, fmt.Errorf("forecast: LabelSet TargetDigest mismatch")
	}
	if lh.Market != th.Primary || lh.Market != spec2.Primary {
		return nil, z, fmt.Errorf("forecast: LabelSet PrimaryMarketKey mismatch")
	}
	finer := spec2.Primary
	finer.Timeframe = spec2.Target.FinerTimeframe
	if lh.FinerMarket != finer {
		return nil, z, fmt.Errorf("forecast: LabelSet FinerMarketKey mismatch")
	}
	if lf.FinerSourceDigest == (Digest{}) {
		return nil, z, fmt.Errorf("forecast: LabelSet FinerSourceDigest is required")
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
	if tf.FirstAt != lf.FirstAt || tf.LastAt != lf.LastAt {
		return nil, z, fmt.Errorf("forecast: research dataset-c FirstAt/LastAt mismatch tape=%d/%d labels=%d/%d",
			tf.FirstAt, tf.LastAt, lf.FirstAt, lf.LastAt)
	}
	keep, acc, err := partitionResearchPopulation(ready, lrows)
	if err != nil {
		return nil, z, err
	}
	out := make([]ResearchRow2, 0, len(keep))
	for _, i := range keep {
		out = append(out, ResearchRow2{At: trows[i].At, Features: trows[i].Values, Outcome: lrows[i].Outcome})
	}
	if len(out) != acc.TrainableTotal {
		return nil, z, fmt.Errorf("forecast: research dataset trainable len=%d accounting=%d", len(out), acc.TrainableTotal)
	}
	return out, acc, nil
}
