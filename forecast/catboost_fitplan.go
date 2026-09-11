package forecast

import (
	"crypto/sha256"
	"fmt"
	"hash"

	"trading_bot/data"
)

// CatBoostFoldPlan is matrix-local inner/outer geometry for one outer fold.
type CatBoostFoldPlan struct {
	OuterTrainBegin  int
	OuterTrainEnd    int
	OuterValBegin    int
	OuterValEnd      int
	InnerTrainBegin  int
	InnerTrainEnd    int
	InnerValBegin    int
	InnerValEnd      int
	InnerValStartAt  int64
	TailExclusiveEnd int64
	TrainLastAt      int64
}

// CatBoostFitPlan is the resolved CatBoost run contract. Not a durable research slot.
type CatBoostFitPlan struct {
	MatrixDigest Digest
	SpecDigest   Digest
	Market       MarketKey
	FeatureIDs   []FeatureID
	TargetH      int
	ExtraGapBars int
	Folds        []CatBoostFoldPlan
}

func (p CatBoostFitPlan) Digest() Digest {
	h := sha256.New()
	hashPutString(h, "CB1P")
	hashCatBoostFitPlan(h, p)
	var d Digest
	copy(d[:], h.Sum(nil))
	return d
}

func hashCatBoostFitPlan(h hash.Hash, p CatBoostFitPlan) {
	hashPutDigest(h, p.MatrixDigest)
	hashPutDigest(h, p.SpecDigest)
	hashPutMarket(h, p.Market)
	hashPutU32(h, uint32(len(p.FeatureIDs)))
	for _, id := range p.FeatureIDs {
		hashPutString(h, string(id))
	}
	hashPutU32(h, uint32(p.TargetH))
	hashPutU32(h, uint32(p.ExtraGapBars))
	hashPutU32(h, uint32(len(p.Folds)))
	for _, f := range p.Folds {
		hashPutU32(h, uint32(f.OuterTrainBegin))
		hashPutU32(h, uint32(f.OuterTrainEnd))
		hashPutU32(h, uint32(f.OuterValBegin))
		hashPutU32(h, uint32(f.OuterValEnd))
		hashPutU32(h, uint32(f.InnerTrainBegin))
		hashPutU32(h, uint32(f.InnerTrainEnd))
		hashPutU32(h, uint32(f.InnerValBegin))
		hashPutU32(h, uint32(f.InnerValEnd))
		hashPutI64(h, f.InnerValStartAt)
		hashPutI64(h, f.TailExclusiveEnd)
		hashPutI64(h, f.TrainLastAt)
	}
}

// ResolveCatBoostFitPlan builds Gate-A geometry from a certified matrix header+At and CatBoostSpec1.
func ResolveCatBoostFitPlan(hdr OOFHeader, rows []OOFRow, matrixDigest Digest, spec CatBoostSpec1) (CatBoostFitPlan, error) {
	var z CatBoostFitPlan
	if err := spec.Validate(); err != nil {
		return z, err
	}
	sid, err := spec.Identity()
	if err != nil {
		return z, err
	}
	if hdr.FormatVersion != OOFMatrixFormatV1 || hdr.AtUnit != OOFAtUnitUnixMs {
		return z, fmt.Errorf("forecast: FITPLAN_INVALID matrix format")
	}
	if len(rows) != hdr.DevRowCount || len(hdr.Folds) != 4 {
		return z, fmt.Errorf("forecast: FITPLAN_INVALID matrix population")
	}
	if len(hdr.FeatureIDs) != Spec2FeatureWidth {
		return z, fmt.Errorf("forecast: FITPLAN_INVALID feature width %d", len(hdr.FeatureIDs))
	}
	tf := hdr.Market.Timeframe
	if tf == "" {
		return z, fmt.Errorf("forecast: FITPLAN_INVALID timeframe")
	}
	targetH := hdr.ValPlanRules.TargetH
	if targetH <= 0 {
		return z, fmt.Errorf("forecast: FITPLAN_INVALID TargetH")
	}
	ats := make([]int64, len(rows))
	for i := range rows {
		ats[i] = rows[i].At
		if _, err := EncodeOOFClass(rows[i].Outcome); err != nil {
			return z, err
		}
		if len(rows[i].Features) != len(hdr.FeatureIDs) {
			return z, fmt.Errorf("forecast: FITPLAN_INVALID row width")
		}
	}
	folds := make([]CatBoostFoldPlan, len(hdr.Folds))
	for i, of := range hdr.Folds {
		if of.TrainBegin != 0 || of.TrainEnd > len(rows) || of.ValEnd > len(rows) || of.ValBegin > of.ValEnd {
			return z, fmt.Errorf("forecast: FITPLAN_INVALID outer fold %d", i)
		}
		trainAt := ats[of.TrainBegin:of.TrainEnd]
		if len(trainAt) == 0 {
			return z, fmt.Errorf("forecast: FITPLAN_INVALID empty outer train fold %d", i)
		}
		last := trainAt[len(trainAt)-1]
		excl, err := data.NextBarOpen(last, tf)
		if err != nil {
			return z, err
		}
		split, err := SplitCausalTail(trainAt, tf, excl, spec.InnerSpanBars, targetH, spec.ExtraGapBars, spec.MinInnerTrainRows)
		if err != nil {
			return z, fmt.Errorf("forecast: fold %d inner split: %w", i, err)
		}
		base := of.TrainBegin
		folds[i] = CatBoostFoldPlan{
			OuterTrainBegin:  of.TrainBegin,
			OuterTrainEnd:    of.TrainEnd,
			OuterValBegin:    of.ValBegin,
			OuterValEnd:      of.ValEnd,
			InnerTrainBegin:  base + split.InnerTrainBegin,
			InnerTrainEnd:    base + split.InnerTrainEnd,
			InnerValBegin:    base + split.InnerValBegin,
			InnerValEnd:      base + split.InnerValEnd,
			InnerValStartAt:  split.InnerValStartAt,
			TailExclusiveEnd: split.TailExclusiveEnd,
			TrainLastAt:      split.TrainLastAt,
		}
		if folds[i].InnerValEnd > of.TrainEnd || folds[i].InnerTrainEnd > of.TrainEnd {
			return z, fmt.Errorf("forecast: fold %d inner not subset of outerTrain", i)
		}
		if of.ValBegin < of.TrainEnd && of.ValBegin < folds[i].InnerValEnd && of.ValBegin >= folds[i].InnerValBegin {
			return z, fmt.Errorf("forecast: fold %d outer val leaked into inner val", i)
		}
	}
	return CatBoostFitPlan{
		MatrixDigest: matrixDigest,
		SpecDigest:   sid.Digest,
		Market:       hdr.Market,
		FeatureIDs:   append([]FeatureID(nil), hdr.FeatureIDs...),
		TargetH:      targetH,
		ExtraGapBars: spec.ExtraGapBars,
		Folds:        folds,
	}, nil
}
