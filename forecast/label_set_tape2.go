package forecast

import "fmt"

// BuildLabelSetFromTape2 labels every feature-tape-v2 row (Ready and NotReady)
// using the canonical first-passage core. It reads tape-v2 natively and never
// opens feature-tape-v1.
func BuildLabelSetFromTape2(tape2Path string, spec2 FeatureSpec2, bars []CanonicalClosedBar, finerMarket MarketKey, finer []CanonicalClosedBar, expect *LabelExpect) (LabelHeader, []LabelRow, Digest, error) {
	b, err := buildLabelsFromTape2(tape2Path, spec2, bars, finerMarket, finer, expect)
	if err != nil {
		return LabelHeader{}, nil, Digest{}, err
	}
	return b.Header, b.Rows, b.Source, nil
}

func buildLabelsFromTape2(tape2Path string, spec2 FeatureSpec2, bars []CanonicalClosedBar, finerMarket MarketKey, finer []CanonicalClosedBar, expect *LabelExpect) (labelBuild, error) {
	var z labelBuild
	if err := validateSpecForLabels(spec2.Target); err != nil {
		return z, err
	}
	hdr, rows, foot, err := ReadTape2(tape2Path)
	if err != nil {
		return z, err
	}
	if err := matchTape2ToSpec2(hdr, spec2); err != nil {
		return z, err
	}
	ats := make([]int64, len(rows))
	for i := range rows {
		ats[i] = rows[i].At
	}
	u := labelCandidateUniverse{
		Ats:           ats,
		Market:        hdr.Primary,
		PlanDigest:    hdr.PlanDigest,
		PrimarySource: hdr.PrimarySource,
		TapeContent:   foot.ContentDigest,
	}
	return buildLabelsFromCandidates(u, spec2.Target, bars, finerMarket, finer, expect)
}

func matchTape2ToSpec2(hdr Tape2Header, spec2 FeatureSpec2) error {
	sid, err := spec2.Identity()
	if err != nil {
		return err
	}
	pid, err := spec2.Plan.Identity()
	if err != nil {
		return err
	}
	fid, err := spec2.Features.Identity()
	if err != nil {
		return err
	}
	aid, err := spec2.Analysis.Identity()
	if err != nil {
		return err
	}
	tid, err := spec2.Target.Identity()
	if err != nil {
		return err
	}
	if hdr.SpecDigest != sid.Digest {
		return fmt.Errorf("forecast: feature-tape-v2 SpecDigest mismatch")
	}
	if hdr.PlanDigest != pid.Digest {
		return fmt.Errorf("forecast: feature-tape-v2 PlanDigest mismatch")
	}
	if hdr.FeaturesDigest != fid.Digest {
		return fmt.Errorf("forecast: feature-tape-v2 FeaturesDigest mismatch")
	}
	if hdr.AnalysisDigest != aid.Digest {
		return fmt.Errorf("forecast: feature-tape-v2 AnalysisDigest mismatch")
	}
	if hdr.TargetDigest != tid.Digest {
		return fmt.Errorf("forecast: TargetDigest mismatch")
	}
	if hdr.Primary != spec2.Primary {
		return fmt.Errorf("forecast: feature-tape-v2 Primary MarketKey mismatch")
	}
	return nil
}
