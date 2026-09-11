package forecast

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"math"
	"os"
)

const (
	OOFMatrixFormatV1 = "oof-matrix-v1"
	OOFAtUnitUnixMs   = "unix_ms"
)

// OOFClassOrder is the frozen model-logit meaning for oof-matrix-v1 rows.
// logits[0]=UP_FIRST, [1]=DOWN_FIRST, [2]=TIMEOUT. Not a header digest field.
var OOFClassOrder = [...]TargetOutcome{OutcomeUpFirst, OutcomeDownFirst, OutcomeTimeout}

const (
	oofKindHeader = "header"
	oofKindRow    = "row"
	oofKindFooter = "footer"
)

// OOFHeader is file-level identity for one development-only OOF matrix.
type OOFHeader struct {
	FormatVersion string
	AtUnit        string
	Market        MarketKey
	FeatureIDs    []FeatureID
	PlanDigest    Digest
	TapeSource    Digest
	TapeContent   Digest
	LabelContent  Digest
	TargetDigest  Digest
	LabelSource   Digest
	FinerSource   Digest
	FinerWindows  int
	LabelTapePlan Digest
	LabelTapeSrc  Digest
	LabelTapeCnt  Digest
	LabelLogic    LogicVersion
	ValPlan       Identity
	ValPlanRules  ValidationPlan
	DevExclusive  int64
	DevRowCount   int
	HoldoutStart  int64
	Folds         []CompiledFold
}

// OOFRow is one exported development candidate.
type OOFRow struct {
	At       int64
	Features []float64
	Outcome  TargetOutcome
}

// OOFFooter closes an OOF matrix.
type OOFFooter struct {
	RowCount      int
	FirstAt       int64
	LastAt        int64
	ContentDigest Digest
}

type oofSourceSnap struct {
	Market      MarketKey
	PlanDigest  Digest
	FeatureIDs  []FeatureID
	VectorLen   int
	TapeSource  Digest
	TapeContent Digest
	labHdr      LabelHeader
	labFt       LabelFooter
}

func readOOFSources(tapePath, labelPath string) (oofSourceSnap, error) {
	var z oofSourceSnap
	th, _, tf, err := ReadTape(tapePath, nil, nil)
	if err != nil {
		return z, err
	}
	lh, _, lf, err := ReadLabelSet(labelPath, nil)
	if err != nil {
		return z, err
	}
	return oofSourceSnap{
		Market: th.Market, PlanDigest: th.PlanDigest, FeatureIDs: th.FeatureIDs, VectorLen: th.VectorLen,
		TapeSource: tf.SourceRangeDigest, TapeContent: tf.ContentDigest, labHdr: lh, labFt: lf,
	}, nil
}

func (a oofSourceSnap) equal(b oofSourceSnap) bool {
	if a.Market != b.Market || a.PlanDigest != b.PlanDigest || a.VectorLen != b.VectorLen {
		return false
	}
	if a.TapeSource != b.TapeSource || a.TapeContent != b.TapeContent {
		return false
	}
	if len(a.FeatureIDs) != len(b.FeatureIDs) {
		return false
	}
	for i := range a.FeatureIDs {
		if a.FeatureIDs[i] != b.FeatureIDs[i] {
			return false
		}
	}
	return a.labHdr.Market == b.labHdr.Market &&
		a.labHdr.TargetDigest == b.labHdr.TargetDigest &&
		a.labHdr.FeatureTapePlanDigest == b.labHdr.FeatureTapePlanDigest &&
		a.labHdr.FeatureTapeSourceRangeDigest == b.labHdr.FeatureTapeSourceRangeDigest &&
		a.labHdr.FeatureTapeContentDigest == b.labHdr.FeatureTapeContentDigest &&
		a.labHdr.LabelLogicVersion == b.labHdr.LabelLogicVersion &&
		a.labHdr.FinerMarket == b.labHdr.FinerMarket &&
		a.labFt.ContentDigest == b.labFt.ContentDigest &&
		a.labFt.LabelSourceRangeDigest == b.labFt.LabelSourceRangeDigest &&
		a.labFt.FinerSourceDigest == b.labFt.FinerSourceDigest &&
		a.labFt.FinerWindowCount == b.labFt.FinerWindowCount
}

// GenerateOOFMatrix opens tape+labels, builds the frozen research dataset,
// compiles validation geometry, and writes the development-only JSONL artifact.
// If outPath already contains the exact same artifact, it returns matched=true
// and does not rewrite.
//
// Future MODEL-FIT-1: fit scaler and model on each fold's Train slice only;
// transform/predict Validation only. Never fit a scaler on this whole matrix.
func GenerateOOFMatrix(tapePath, labelPath, outPath string, expect ResearchDatasetExpect, plan ValidationPlan) (OOFHeader, OOFFooter, bool, error) {
	var z OOFHeader
	var zf OOFFooter
	pre, err := readOOFSources(tapePath, labelPath)
	if err != nil {
		return z, zf, false, err
	}
	rows, _, err := BuildResearchDataset(tapePath, labelPath, expect)
	if err != nil {
		return z, zf, false, err
	}
	post, err := readOOFSources(tapePath, labelPath)
	if err != nil {
		return z, zf, false, err
	}
	if !pre.equal(post) {
		return z, zf, false, fmt.Errorf("forecast: OOF matrix source identities changed during dataset build")
	}
	hdr, exported, err := assembleOOFMatrix(rows, pre, plan)
	if err != nil {
		return z, zf, false, err
	}
	return commitOOFMatrix(outPath, hdr, exported)
}

func readOOFSources2(tape2Path, labelPath string) (oofSourceSnap, error) {
	var z oofSourceSnap
	th, _, tf, err := ReadTape2(tape2Path)
	if err != nil {
		return z, err
	}
	lh, _, lf, err := ReadLabelSet(labelPath, nil)
	if err != nil {
		return z, err
	}
	return oofSourceSnap{
		Market: th.Primary, PlanDigest: th.PlanDigest, FeatureIDs: th.FeatureIDs, VectorLen: th.VectorLen,
		TapeSource: th.PrimarySource, TapeContent: tf.ContentDigest, labHdr: lh, labFt: lf,
	}, nil
}

func researchRowsFromV2(in []ResearchRow2) []ResearchRow {
	out := make([]ResearchRow, len(in))
	for i, r := range in {
		out[i] = ResearchRow{At: r.At, Features: r.Features.Slice(), Outcome: r.Outcome}
	}
	return out
}

// GenerateOOFMatrixFromTape2 is the Brain-V2 OOF-MATRIX-C door.
// Native Tape2 + Dataset-C + ValidationPlan-C. Shared assemble/writer with V1.
func GenerateOOFMatrixFromTape2(tape2Path, labelPath, outPath string, spec2 FeatureSpec2, expect ResearchDataset2Expect, plan ValidationPlan) (OOFHeader, OOFFooter, bool, error) {
	var z OOFHeader
	var zf OOFFooter
	if plan.TargetH != spec2.Target.HorizonBars {
		return z, zf, false, fmt.Errorf("forecast: ValidationPlan TargetH %d != FeatureSpec2 HorizonBars %d", plan.TargetH, spec2.Target.HorizonBars)
	}
	if plan.Timeframe != spec2.Primary.Timeframe {
		return z, zf, false, fmt.Errorf("forecast: ValidationPlan timeframe %q != Primary %q", plan.Timeframe, spec2.Primary.Timeframe)
	}
	pre, err := readOOFSources2(tape2Path, labelPath)
	if err != nil {
		return z, zf, false, err
	}
	rows2, _, err := BuildResearchDatasetFromTape2(tape2Path, labelPath, spec2, expect)
	if err != nil {
		return z, zf, false, err
	}
	post, err := readOOFSources2(tape2Path, labelPath)
	if err != nil {
		return z, zf, false, err
	}
	if !pre.equal(post) {
		return z, zf, false, fmt.Errorf("forecast: OOF matrix source identities changed during dataset build")
	}
	hdr, exported, err := assembleOOFMatrix(researchRowsFromV2(rows2), pre, plan)
	if err != nil {
		return z, zf, false, err
	}
	return commitOOFMatrix(outPath, hdr, exported)
}

func commitOOFMatrix(outPath string, hdr OOFHeader, exported []OOFRow) (OOFHeader, OOFFooter, bool, error) {
	var z OOFHeader
	var zf OOFFooter
	want := hashOOFMatrix(hdr, exported)
	if _, err := os.Stat(outPath); err == nil {
		gotH, gotRows, gotF, err := ReadOOFMatrix(outPath)
		if err != nil {
			return z, zf, false, fmt.Errorf("forecast: refuse existing oof-matrix %s: %w", outPath, err)
		}
		if gotF.ContentDigest == want && oofHeaderEqual(gotH, hdr) && len(gotRows) == len(exported) {
			return gotH, gotF, true, nil
		}
		return z, zf, false, fmt.Errorf("forecast: refuse overwrite of different oof-matrix %s", outPath)
	} else if !os.IsNotExist(err) {
		return z, zf, false, err
	}
	ft, err := writeOOFMatrix(outPath, hdr, exported, want)
	if err != nil {
		return z, zf, false, err
	}
	backH, _, backF, err := ReadOOFMatrix(outPath)
	if err != nil {
		return z, zf, false, err
	}
	if backH.FormatVersion != OOFMatrixFormatV1 || backH.AtUnit != OOFAtUnitUnixMs {
		return z, zf, false, fmt.Errorf("forecast: oof-matrix readback format/AtUnit mismatch")
	}
	if backF.ContentDigest != want || backF.ContentDigest != ft.ContentDigest || backH.DevRowCount != hdr.DevRowCount || !oofHeaderEqual(backH, hdr) {
		return z, zf, false, fmt.Errorf("forecast: oof-matrix readback mismatch")
	}
	return backH, backF, false, nil
}

func assembleOOFMatrix(rows []ResearchRow, src oofSourceSnap, plan ValidationPlan) (OOFHeader, []OOFRow, error) {
	var z OOFHeader
	ats := make([]int64, len(rows))
	for i := range rows {
		ats[i] = rows[i].At
	}
	tf := src.Market.Timeframe
	compiled, err := CompileValidationPlan(ats, tf, plan)
	if err != nil {
		return z, nil, err
	}
	devN := compiled.DevelopmentEndIndex
	if devN <= 0 || devN > len(rows) {
		return z, nil, fmt.Errorf("forecast: illegal DevelopmentEndIndex %d (rows=%d)", devN, len(rows))
	}
	if err := validateOOFFolds(compiled, devN); err != nil {
		return z, nil, err
	}
	exported := make([]OOFRow, devN)
	width := src.VectorLen
	ids := append([]FeatureID(nil), src.FeatureIDs...)
	if len(ids) != width {
		return z, nil, fmt.Errorf("forecast: tape FeatureIDs len %d != VectorLen %d", len(ids), width)
	}
	for i := 0; i < devN; i++ {
		r := rows[i]
		if r.At >= compiled.DevelopmentExclusiveEndAt {
			return z, nil, fmt.Errorf("forecast: development row At %d is not < DevelopmentExclusiveEndAt %d", r.At, compiled.DevelopmentExclusiveEndAt)
		}
		if err := validateOOFExportRow(width, r); err != nil {
			return z, nil, err
		}
		exported[i] = OOFRow{At: r.At, Features: cloneFeatureVector(r.Features), Outcome: r.Outcome}
	}
	if devN < len(rows) && rows[devN].At < compiled.DevelopmentExclusiveEndAt {
		return z, nil, fmt.Errorf("forecast: first omitted row At %d is still before development exclusive end", rows[devN].At)
	}
	id, err := plan.Identity()
	if err != nil {
		return z, nil, err
	}
	hdr := OOFHeader{
		FormatVersion: OOFMatrixFormatV1,
		AtUnit:        OOFAtUnitUnixMs,
		Market:        src.Market,
		FeatureIDs:    ids,
		PlanDigest:    src.PlanDigest,
		TapeSource:    src.TapeSource,
		TapeContent:   src.TapeContent,
		LabelContent:  src.labFt.ContentDigest,
		TargetDigest:  src.labHdr.TargetDigest,
		LabelSource:   src.labFt.LabelSourceRangeDigest,
		FinerSource:   src.labFt.FinerSourceDigest,
		FinerWindows:  src.labFt.FinerWindowCount,
		LabelTapePlan: src.labHdr.FeatureTapePlanDigest,
		LabelTapeSrc:  src.labHdr.FeatureTapeSourceRangeDigest,
		LabelTapeCnt:  src.labHdr.FeatureTapeContentDigest,
		LabelLogic:    src.labHdr.LabelLogicVersion,
		ValPlan:       id,
		ValPlanRules:  plan,
		DevExclusive:  compiled.DevelopmentExclusiveEndAt,
		DevRowCount:   devN,
		HoldoutStart:  compiled.HoldoutStartAt,
		Folds:         append([]CompiledFold(nil), compiled.Folds...),
	}
	return hdr, exported, nil
}

func validateOOFFolds(c CompiledValidationPlan, devN int) error {
	for i, f := range c.Folds {
		if f.TrainBegin < 0 || f.TrainBegin > f.TrainEnd || f.TrainEnd > devN {
			return fmt.Errorf("forecast: fold %d train range [%d,%d) not in matrix [0,%d)", i, f.TrainBegin, f.TrainEnd, devN)
		}
		if f.ValBegin < 0 || f.ValBegin > f.ValEnd || f.ValEnd > devN {
			return fmt.Errorf("forecast: fold %d val range [%d,%d) not in matrix [0,%d)", i, f.ValBegin, f.ValEnd, devN)
		}
		if f.ValEnd > c.DevelopmentEndIndex || f.TrainEnd > c.DevelopmentEndIndex {
			return fmt.Errorf("forecast: fold %d range reaches seam/holdout", i)
		}
		for j := i + 1; j < len(c.Folds); j++ {
			g := c.Folds[j]
			if f.ValEnd > g.ValBegin && g.ValEnd > f.ValBegin {
				return fmt.Errorf("forecast: fold %d and %d validation overlap", i, j)
			}
		}
	}
	return nil
}

func validateOOFExportRow(width int, r ResearchRow) error {
	if len(r.Features) != width {
		return fmt.Errorf("forecast: oof-matrix feature width %d != schema %d at %d", len(r.Features), width, r.At)
	}
	for _, v := range r.Features {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("forecast: oof-matrix nonfinite feature at %d", r.At)
		}
	}
	switch r.Outcome {
	case OutcomeUpFirst, OutcomeDownFirst, OutcomeTimeout:
	default:
		return fmt.Errorf("forecast: oof-matrix illegal outcome %q at %d", r.Outcome, r.At)
	}
	return nil
}

func oofHeaderEqual(a, b OOFHeader) bool {
	if a.FormatVersion != b.FormatVersion || a.AtUnit != b.AtUnit || a.Market != b.Market {
		return false
	}
	if a.PlanDigest != b.PlanDigest || a.TapeSource != b.TapeSource || a.TapeContent != b.TapeContent {
		return false
	}
	if a.LabelContent != b.LabelContent || a.TargetDigest != b.TargetDigest || a.LabelSource != b.LabelSource {
		return false
	}
	if a.FinerSource != b.FinerSource || a.FinerWindows != b.FinerWindows {
		return false
	}
	if a.LabelTapePlan != b.LabelTapePlan || a.LabelTapeSrc != b.LabelTapeSrc || a.LabelTapeCnt != b.LabelTapeCnt || a.LabelLogic != b.LabelLogic {
		return false
	}
	if a.ValPlan.Digest != b.ValPlan.Digest || a.ValPlanRules != b.ValPlanRules {
		return false
	}
	if a.DevExclusive != b.DevExclusive || a.DevRowCount != b.DevRowCount || a.HoldoutStart != b.HoldoutStart {
		return false
	}
	if len(a.FeatureIDs) != len(b.FeatureIDs) || len(a.Folds) != len(b.Folds) {
		return false
	}
	for i := range a.FeatureIDs {
		if a.FeatureIDs[i] != b.FeatureIDs[i] {
			return false
		}
	}
	for i := range a.Folds {
		if a.Folds[i] != b.Folds[i] {
			return false
		}
	}
	return true
}

func hashOOFMatrix(hdr OOFHeader, rows []OOFRow) Digest {
	h := sha256.New()
	hashPutString(h, "OM1C")
	hashOOFHeader(h, hdr)
	for i := range rows {
		hashPutI64(h, rows[i].At)
		hashPutU32(h, uint32(len(rows[i].Features)))
		for _, v := range rows[i].Features {
			hashPutF64(h, v)
		}
		hashPutString(h, string(rows[i].Outcome))
	}
	hashPutU32(h, uint32(len(rows)))
	if len(rows) > 0 {
		hashPutI64(h, rows[0].At)
		hashPutI64(h, rows[len(rows)-1].At)
	}
	var d Digest
	copy(d[:], h.Sum(nil))
	return d
}

func hashOOFHeader(h hash.Hash, hdr OOFHeader) {
	hashPutString(h, hdr.FormatVersion)
	hashPutString(h, hdr.AtUnit)
	hashPutMarket(h, hdr.Market)
	hashPutU32(h, uint32(len(hdr.FeatureIDs)))
	for _, id := range hdr.FeatureIDs {
		hashPutString(h, string(id))
	}
	hashPutDigest(h, hdr.PlanDigest)
	hashPutDigest(h, hdr.TapeSource)
	hashPutDigest(h, hdr.TapeContent)
	hashPutDigest(h, hdr.LabelContent)
	hashPutDigest(h, hdr.TargetDigest)
	hashPutDigest(h, hdr.LabelSource)
	hashPutDigest(h, hdr.FinerSource)
	hashPutU32(h, uint32(hdr.FinerWindows))
	hashPutDigest(h, hdr.LabelTapePlan)
	hashPutDigest(h, hdr.LabelTapeSrc)
	hashPutDigest(h, hdr.LabelTapeCnt)
	hashPutString(h, string(hdr.LabelLogic))
	hashPutDigest(h, hdr.ValPlan.Digest)
	hashPutString(h, string(hdr.ValPlanRules.Logic))
	hashPutString(h, hdr.ValPlanRules.Timeframe)
	hashPutI64(h, hdr.ValPlanRules.HoldoutStartAt)
	hashPutU32(h, uint32(hdr.ValPlanRules.ValidationSpanBars))
	hashPutU32(h, uint32(hdr.ValPlanRules.FoldCount))
	hashPutU32(h, uint32(hdr.ValPlanRules.TargetH))
	hashPutU32(h, uint32(hdr.ValPlanRules.ExtraGapBars))
	hashPutU32(h, uint32(hdr.ValPlanRules.MinTrainRows))
	hashPutI64(h, hdr.DevExclusive)
	hashPutU32(h, uint32(hdr.DevRowCount))
	hashPutI64(h, hdr.HoldoutStart)
	hashPutU32(h, uint32(len(hdr.Folds)))
	for _, f := range hdr.Folds {
		hashPutU32(h, uint32(f.TrainBegin))
		hashPutU32(h, uint32(f.TrainEnd))
		hashPutU32(h, uint32(f.ValBegin))
		hashPutU32(h, uint32(f.ValEnd))
		hashPutI64(h, f.ValBoundaryStartAt)
		hashPutI64(h, f.ValBoundaryEndAt)
		hashPutI64(h, f.TrainFirstAt)
		hashPutI64(h, f.TrainLastAt)
		hashPutI64(h, f.ValFirstAt)
		hashPutI64(h, f.ValLastAt)
	}
}
