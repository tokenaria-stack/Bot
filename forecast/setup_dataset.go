package forecast

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

const (
	SetupDatasetFormat                     = "setup-dataset-v1"
	SetupDatasetLogicJoinV1   LogicVersion = "setup-dataset:join-v1"
	SetupDatasetFeatureWidth               = Spec2FeatureWidth + 2
	SetupFeatureRiskATR                    = "setup_risk_atr"
	SetupFeatureS0AnchorAge                = "s0_anchor_age_bars"
	SetupClassFeatureNotReady              = "FEATURE_NOT_READY"
	setupLocalLawCopyStop2                 = "copy StructuralStopAssignment.ROverATR15 and AnchorAgeBars; exact FeatureTape2 At join; no geometry"
)

// SetupDatasetRow is one model-eligible DEV setup. X is Spec2[64] + two STOP-2 facts.
type SetupDatasetRow struct {
	At      int64     `json:"at"`
	Y       string    `json:"y"`
	X       []float64 `json:"x"`
	OOFFold int       `json:"oof_fold"`
}

// SetupDataset1 is the join-only DEV matrix. It does not train, score, or open holdout.
type SetupDataset1 struct {
	Format               string                   `json:"format"`
	Width                int                      `json:"width"`
	FeatureNames         []string                 `json:"feature_names"`
	Rows                 []SetupDatasetRow        `json:"rows"`
	MinCandidateAt       int64                    `json:"min_candidate_at"`
	MaxCandidateAt       int64                    `json:"max_candidate_at"`
	HoldoutStartAt       int64                    `json:"holdout_start_at"`
	FeatureSpec2Digest   string                   `json:"feature_spec2_digest"`
	FeatureRecipeDigest  string                   `json:"feature_recipe_digest"`
	FeatureLogic         LogicVersion             `json:"feature_logic_version"`
	SetupTargetDigest    string                   `json:"setup_target_digest"`
	LabelContentDigest   string                   `json:"setup_labelset_content_digest"`
	ValidationPlanDigest string                   `json:"validation_plan_digest"`
	DataSplitDigest      string                   `json:"data_split_digest"`
	SetupLocalDigest     string                   `json:"setup_local_digest"`
	DatasetDigest        string                   `json:"dataset_identity_digest"`
	ContentDigest        string                   `json:"content_digest"`
	TapeContentDigest    string                   `json:"tape_content_digest,omitempty"`
	Accounting           SetupDatasetAccounting   `json:"accounting"`
	Folds                []SetupDatasetFoldCensus `json:"oof_folds"`
	RiskATR              SetupDatasetPercentiles  `json:"setup_risk_atr_dev"`
	S0Age                SetupDatasetPercentiles  `json:"s0_anchor_age_dev"`
	Text                 string                   `json:"report"`
}

type SetupDatasetPercentiles struct {
	N   int     `json:"n"`
	P10 float64 `json:"p10"`
	P50 float64 `json:"p50"`
	P90 float64 `json:"p90"`
}

type SetupDatasetAccounting struct {
	SourceDEV             int `json:"source_dev"`
	SourceHoldout         int `json:"source_holdout"`
	DEVTP                 int `json:"dev_tp_first"`
	DEVStop               int `json:"dev_stop_first"`
	DEVTimeout            int `json:"dev_timeout"`
	DEVInvalid            int `json:"dev_invalid_structure"`
	DEVSkip               int `json:"dev_not_evaluable"`
	DEVUsableLabels       int `json:"dev_usable_labels"`
	ExactJoins            int `json:"exact_feature_joins"`
	FeatureNotReady       int `json:"feature_not_ready"`
	MissingJoins          int `json:"missing_joins"`
	DuplicateTapeAt       int `json:"duplicate_tape_at"`
	DuplicateLabelAt      int `json:"duplicate_label_at"`
	DuplicateAssignmentAt int `json:"duplicate_assignment_at"`
	FinalRows             int `json:"final_dataset_rows"`
	FinalTP               int `json:"final_tp_first"`
	FinalStop             int `json:"final_stop_first"`
	FinalTimeout          int `json:"final_timeout"`
	HoldoutUsable         int `json:"holdout_usable_excluded"`
	TrainOnlyRows         int `json:"train_only_rows"`
}

type SetupDatasetFoldCensus struct {
	Index           int   `json:"index"`
	LabelValN       int   `json:"frozen_label_val_n"`
	Rows            int   `json:"dataset_n"`
	TP              int   `json:"tp_first"`
	Stop            int   `json:"stop_first"`
	Timeout         int   `json:"timeout"`
	FeatureNotReady int   `json:"feature_not_ready"`
	ValFirstAt      int64 `json:"val_first_at"`
	ValLastAt       int64 `json:"val_last_at"`
}

// SetupDataset1Input is the four frozen owners plus FeatureSpec2 identity.
// Dataset does not own market context or ticket geometry.
type SetupDataset1Input struct {
	Spec2       FeatureSpec2
	TapeHeader  Tape2Header
	TapeFooter  Tape2Footer
	Tape        []FeatureRow2
	Labels      SetupLabelSet1
	Assignments []StructuralStopAssignment
	Validation  SetupValidationReport
}

type setupLocalPayload struct {
	IDs   []string
	Width int
	Law   string
}

type setupDatasetIdentityPayload struct {
	Format         string
	Width          int
	FeatureNames   []string
	FeatureSpec2   Digest
	FeatureRecipe  Digest
	FeatureLogic   LogicVersion
	SetupTarget    Digest
	LabelContent   Digest
	ValidationPlan Digest
	DataSplit      Digest
	SetupLocal     Digest
	SetupLocalIDs  []string
}

// SetupDataset1FeatureNames is the frozen 66-column order.
func SetupDataset1FeatureNames() []string {
	ids := FeatureSpec2IDs()
	out := make([]string, SetupDatasetFeatureWidth)
	for i, id := range ids {
		out[i] = string(id)
	}
	out[Spec2FeatureWidth] = SetupFeatureRiskATR
	out[Spec2FeatureWidth+1] = SetupFeatureS0AnchorAge
	return out
}

func setupLocalIdentity() (Identity, error) {
	ids := []string{SetupFeatureRiskATR, SetupFeatureS0AnchorAge}
	return NewIdentity("setup-local-v1", setupLocalPayload{
		IDs: ids, Width: 2, Law: setupLocalLawCopyStop2,
	}, SetupDatasetLogicJoinV1)
}

func pinSetupDatasetValidation(v SetupValidationReport) (DataSplitPolicy, ValidationPlan, error) {
	wantSplit := DefaultDataSplitPolicy()
	wantSID, err := wantSplit.Identity()
	if err != nil {
		return DataSplitPolicy{}, ValidationPlan{}, err
	}
	wantPlan, err := FrozenSetupValidationPlan()
	if err != nil {
		return DataSplitPolicy{}, ValidationPlan{}, err
	}
	wantPID, err := wantPlan.Identity()
	if err != nil {
		return DataSplitPolicy{}, ValidationPlan{}, err
	}
	gotSID, err := v.Split.Identity()
	if err != nil {
		return DataSplitPolicy{}, ValidationPlan{}, err
	}
	gotPID, err := v.Plan.Identity()
	if err != nil {
		return DataSplitPolicy{}, ValidationPlan{}, err
	}
	if gotSID.Digest != wantSID.Digest || v.SplitDigest != wantSID.Digest.String() {
		return DataSplitPolicy{}, ValidationPlan{}, fmt.Errorf("forecast: SETUP-DATASET-1 DataSplitPolicy identity mismatch (wall must come from frozen SETUP-VALIDATION-1)")
	}
	if gotPID.Digest != wantPID.Digest || v.PlanDigest != wantPID.Digest.String() {
		return DataSplitPolicy{}, ValidationPlan{}, fmt.Errorf("forecast: SETUP-DATASET-1 ValidationPlan identity mismatch")
	}
	if v.Split.HoldoutStartAt != wantSplit.HoldoutStartAt {
		return DataSplitPolicy{}, ValidationPlan{}, fmt.Errorf("forecast: SETUP-DATASET-1 holdout wall mismatch")
	}
	if len(v.Folds) != SetupValidationFoldCount {
		return DataSplitPolicy{}, ValidationPlan{}, fmt.Errorf("forecast: SETUP-DATASET-1 requires %d frozen OOF folds", SetupValidationFoldCount)
	}
	return v.Split, v.Plan, nil
}

func indexTapeByAt(tape []FeatureRow2) (map[int64]FeatureRow2, int, error) {
	out := make(map[int64]FeatureRow2, len(tape))
	dups := 0
	for _, row := range tape {
		if _, ok := out[row.At]; ok {
			dups++
			continue
		}
		out[row.At] = row
	}
	if dups > 0 {
		return out, dups, fmt.Errorf("forecast: SETUP-DATASET-1 duplicate FeatureTape2 At n=%d", dups)
	}
	return out, 0, nil
}

func indexLongAssignments(as []StructuralStopAssignment) (map[int64]StructuralStopAssignment, int, error) {
	out := make(map[int64]StructuralStopAssignment)
	dups := 0
	for _, a := range as {
		if a.Side != GeomSideLong {
			continue
		}
		if _, ok := out[a.At]; ok {
			dups++
			continue
		}
		out[a.At] = a
	}
	if dups > 0 {
		return out, dups, fmt.Errorf("forecast: SETUP-DATASET-1 duplicate STOP-2 LONG At n=%d", dups)
	}
	return out, 0, nil
}

func oofFoldOf(at int64, folds []SetupValidationFold) (int, error) {
	hit := -1
	for _, f := range folds {
		if at >= f.ValFirstAt && at <= f.ValLastAt {
			if hit >= 0 {
				return -1, fmt.Errorf("forecast: SETUP-DATASET-1 At %d belongs to more than one OOF span", at)
			}
			hit = f.Index
		}
	}
	return hit, nil
}

func copySetupX(tape FeatureRow2, a StructuralStopAssignment, tf string) ([]float64, error) {
	if err := Vector2Finite(tape.Values); err != nil {
		return nil, err
	}
	if !isFinite(a.ROverATR15) || !(a.ROverATR15 > 0) {
		return nil, fmt.Errorf("forecast: SETUP-DATASET-1 At %d setup_risk_atr must be finite > 0 (owned ROverATR15)", a.At)
	}
	if a.PivotAnchorAt <= 0 {
		return nil, fmt.Errorf("forecast: SETUP-DATASET-1 At %d missing owned S0 PivotAnchorAt", a.At)
	}
	age, err := barAge(a.PivotAnchorAt, a.At, tf)
	if err != nil {
		return nil, err
	}
	if age < 0 {
		return nil, fmt.Errorf("forecast: SETUP-DATASET-1 At %d S0 AnchorAt is after candidate", a.At)
	}
	if a.AnchorAgeBars != age {
		return nil, fmt.Errorf("forecast: SETUP-DATASET-1 At %d owned AnchorAgeBars=%d != barAge(PivotAnchorAt)=%d", a.At, a.AnchorAgeBars, age)
	}
	x := make([]float64, SetupDatasetFeatureWidth)
	copy(x[:Spec2FeatureWidth], tape.Values[:])
	x[Spec2FeatureWidth] = a.ROverATR15
	x[Spec2FeatureWidth+1] = float64(a.AnchorAgeBars)
	for i, v := range x {
		if !isFinite(v) {
			return nil, fmt.Errorf("forecast: SETUP-DATASET-1 At %d X[%d] is not finite", a.At, i)
		}
	}
	return x, nil
}

// BuildSetupDataset1 joins FeatureTape2 + SETUP-LABELSET-1 + STOP-2 assignments
// under frozen SETUP-VALIDATION-1. It copies ticket facts; it does not find S0.
func BuildSetupDataset1(in SetupDataset1Input) (SetupDataset1, error) {
	var z SetupDataset1
	if !in.Labels.MatchOK {
		return z, fmt.Errorf("forecast: SETUP-DATASET-1 requires MOVE-POTENTIAL +2R MATCH labels")
	}
	split, plan, err := pinSetupDatasetValidation(in.Validation)
	if err != nil {
		return z, err
	}
	wall := split.HoldoutStartAt
	if err := in.Spec2.Primary.Validate(); err != nil {
		return z, err
	}
	specID, err := in.Spec2.Identity()
	if err != nil {
		return z, err
	}
	featID, err := in.Spec2.Features.Identity()
	if err != nil {
		return z, err
	}
	tgtID, err := FrozenSetupTarget1().Identity()
	if err != nil {
		return z, err
	}
	labID, err := in.Labels.Target.Identity()
	if err != nil {
		return z, err
	}
	if labID.Digest != tgtID.Digest {
		return z, fmt.Errorf("forecast: SETUP-DATASET-1 SetupTarget identity mismatch")
	}
	labelContent, err := in.Labels.ContentDigest()
	if err != nil {
		return z, err
	}
	localID, err := setupLocalIdentity()
	if err != nil {
		return z, err
	}
	planID, err := plan.Identity()
	if err != nil {
		return z, err
	}
	splitID, err := split.Identity()
	if err != nil {
		return z, err
	}
	if in.TapeHeader.FormatVersion != "" {
		if err := matchTape2ToSpec2(in.TapeHeader, in.Spec2); err != nil {
			return z, err
		}
	}
	names := SetupDataset1FeatureNames()
	if len(names) != SetupDatasetFeatureWidth {
		return z, fmt.Errorf("forecast: SETUP-DATASET-1 width")
	}
	wantIDs := FeatureSpec2IDs()
	for i, id := range wantIDs {
		if names[i] != string(id) {
			return z, fmt.Errorf("forecast: SETUP-DATASET-1 FeatureSpec2 order mismatch at %d", i)
		}
	}

	tapeByAt, tapeDups, tapeErr := indexTapeByAt(in.Tape)
	asgByAt, asgDups, asgErr := indexLongAssignments(in.Assignments)
	z.Accounting.DuplicateTapeAt = tapeDups
	z.Accounting.DuplicateAssignmentAt = asgDups
	if tapeErr != nil {
		return z, tapeErr
	}
	if asgErr != nil {
		return z, asgErr
	}

	seenLabel := map[int64]struct{}{}
	acc := z.Accounting
	foldNR := make([]int, len(in.Validation.Folds))
	var rows []SetupDatasetRow
	tf := plan.Timeframe
	if tf == "" {
		tf = Spec2PrimaryTF
	}

	for _, lab := range in.Labels.Rows {
		if lab.At <= 0 {
			return z, fmt.Errorf("forecast: SETUP-DATASET-1 label At is required")
		}
		if _, ok := seenLabel[lab.At]; ok {
			acc.DuplicateLabelAt++
			return z, fmt.Errorf("forecast: SETUP-DATASET-1 duplicate candidate At %d", lab.At)
		}
		seenLabel[lab.At] = struct{}{}
		role := split.Role(lab.At)
		if role == DataRoleHoldout {
			acc.SourceHoldout++
			switch lab.Class {
			case SetupClassTP, SetupClassStop, SetupClassTimeout:
				acc.HoldoutUsable++
			}
			continue
		}
		acc.SourceDEV++
		switch lab.Class {
		case SetupClassInvalid:
			acc.DEVInvalid++
			continue
		case SetupClassSkip:
			acc.DEVSkip++
			continue
		case SetupClassTP, SetupClassStop, SetupClassTimeout:
			acc.DEVUsableLabels++
			switch lab.Class {
			case SetupClassTP:
				acc.DEVTP++
			case SetupClassStop:
				acc.DEVStop++
			case SetupClassTimeout:
				acc.DEVTimeout++
			}
			tape, ok := tapeByAt[lab.At]
			if !ok {
				acc.MissingJoins++
				continue
			}
			if tape.Ready != IsReady {
				acc.FeatureNotReady++
				fi, err := oofFoldOf(lab.At, in.Validation.Folds)
				if err != nil {
					return z, err
				}
				if fi >= 0 {
					foldNR[fi]++
				}
				continue
			}
			a, ok := asgByAt[lab.At]
			if !ok {
				return z, fmt.Errorf("forecast: SETUP-DATASET-1 At %d missing STOP-2 assignment (dataset must not reconstruct S0)", lab.At)
			}
			x, err := copySetupX(tape, a, tf)
			if err != nil {
				return z, err
			}
			acc.ExactJoins++
			fi, err := oofFoldOf(lab.At, in.Validation.Folds)
			if err != nil {
				return z, err
			}
			row := SetupDatasetRow{At: lab.At, Y: lab.Class, X: x, OOFFold: fi}
			if err := split.RefuseHoldoutAt(row.At); err != nil {
				return z, err
			}
			if split.Role(row.At) == DataRoleHoldout {
				return z, fmt.Errorf("forecast: SETUP-DATASET-1 HOLDOUT row leaked into dataset At=%d", row.At)
			}
			rows = append(rows, row)
			switch lab.Class {
			case SetupClassTP:
				acc.FinalTP++
			case SetupClassStop:
				acc.FinalStop++
			case SetupClassTimeout:
				acc.FinalTimeout++
			}
		default:
			return z, fmt.Errorf("forecast: SETUP-DATASET-1 unknown class %q at %d", lab.Class, lab.At)
		}
	}
	if acc.MissingJoins > 0 {
		return z, fmt.Errorf("forecast: SETUP-DATASET-1 missing exact FeatureTape2 At n=%d (hard fail)", acc.MissingJoins)
	}
	acc.FinalRows = len(rows)
	if acc.FinalRows != acc.FinalTP+acc.FinalStop+acc.FinalTimeout {
		return z, fmt.Errorf("forecast: SETUP-DATASET-1 class partition does not close")
	}
	if acc.FinalRows != acc.DEVUsableLabels-acc.FeatureNotReady {
		return z, fmt.Errorf("forecast: SETUP-DATASET-1 final=%d usable=%d not_ready=%d", acc.FinalRows, acc.DEVUsableLabels, acc.FeatureNotReady)
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].At <= rows[i-1].At {
			return z, fmt.Errorf("forecast: SETUP-DATASET-1 rows are not strictly increasing At")
		}
	}

	folds := make([]SetupDatasetFoldCensus, len(in.Validation.Folds))
	for i, f := range in.Validation.Folds {
		sf := SetupDatasetFoldCensus{
			Index: f.Index, LabelValN: f.ValN, FeatureNotReady: foldNR[i],
			ValFirstAt: f.ValFirstAt, ValLastAt: f.ValLastAt,
		}
		for _, row := range rows {
			if row.OOFFold != f.Index {
				continue
			}
			sf.Rows++
			switch row.Y {
			case SetupClassTP:
				sf.TP++
			case SetupClassStop:
				sf.Stop++
			case SetupClassTimeout:
				sf.Timeout++
			}
		}
		if sf.Rows+sf.FeatureNotReady != sf.LabelValN {
			return z, fmt.Errorf("forecast: SETUP-DATASET-1 fold %d dataset_n=%d not_ready=%d != frozen val_n=%d", f.Index, sf.Rows, sf.FeatureNotReady, sf.LabelValN)
		}
		folds[i] = sf
	}
	trainOnly := 0
	for _, row := range rows {
		if row.OOFFold < 0 {
			trainOnly++
		}
	}
	acc.TrainOnlyRows = trainOnly

	var minAt, maxAt int64
	risk := make([]float64, 0, len(rows))
	age := make([]float64, 0, len(rows))
	for i, row := range rows {
		if len(row.X) != SetupDatasetFeatureWidth {
			return z, fmt.Errorf("forecast: SETUP-DATASET-1 row width")
		}
		if i == 0 || row.At < minAt {
			minAt = row.At
		}
		if row.At > maxAt {
			maxAt = row.At
		}
		risk = append(risk, row.X[Spec2FeatureWidth])
		age = append(age, row.X[Spec2FeatureWidth+1])
	}
	if len(rows) > 0 && maxAt >= wall {
		return z, fmt.Errorf("forecast: MaxCandidateAt %d is not < holdout wall %d", maxAt, wall)
	}

	idPayload := setupDatasetIdentityPayload{
		Format: SetupDatasetFormat, Width: SetupDatasetFeatureWidth, FeatureNames: names,
		FeatureSpec2: specID.Digest, FeatureRecipe: featID.Digest, FeatureLogic: FeaturesLogicV2,
		SetupTarget: tgtID.Digest, LabelContent: labelContent, ValidationPlan: planID.Digest,
		DataSplit: splitID.Digest, SetupLocal: localID.Digest,
		SetupLocalIDs: []string{SetupFeatureRiskATR, SetupFeatureS0AnchorAge},
	}
	dsID, err := NewIdentity("setup-dataset-1", idPayload, SetupDatasetLogicJoinV1, FeaturesLogicV2)
	if err != nil {
		return z, err
	}
	content := newSetupDatasetContentHasher()
	content.meta(dsID.Digest, names, wall, in.TapeFooter.ContentDigest)
	for _, row := range rows {
		content.row(row)
	}
	content.counts(len(rows), minAt, maxAt)
	cd := content.sum()

	z.Format = SetupDatasetFormat
	z.Width = SetupDatasetFeatureWidth
	z.FeatureNames = names
	z.Rows = rows
	z.MinCandidateAt = minAt
	z.MaxCandidateAt = maxAt
	z.HoldoutStartAt = wall
	z.FeatureSpec2Digest = specID.Digest.String()
	z.FeatureRecipeDigest = featID.Digest.String()
	z.FeatureLogic = FeaturesLogicV2
	z.SetupTargetDigest = tgtID.Digest.String()
	z.LabelContentDigest = labelContent.String()
	z.ValidationPlanDigest = planID.Digest.String()
	z.DataSplitDigest = splitID.Digest.String()
	z.SetupLocalDigest = localID.Digest.String()
	z.DatasetDigest = dsID.Digest.String()
	z.ContentDigest = cd.String()
	if in.TapeFooter.ContentDigest != (Digest{}) {
		z.TapeContentDigest = in.TapeFooter.ContentDigest.String()
	}
	z.Accounting = acc
	z.Folds = folds
	z.RiskATR = percentilesP10P50P90(risk)
	z.S0Age = percentilesP10P50P90(age)
	z.Text = FormatSetupDataset1(z)
	return z, nil
}

type setupDatasetHasher struct {
	c *contentHasher
}

func newSetupDatasetContentHasher() *setupDatasetHasher {
	c := &contentHasher{h: sha256.New()}
	hashPutString(c.h, "SD1C")
	return &setupDatasetHasher{c: c}
}

func (s *setupDatasetHasher) meta(id Digest, names []string, wall int64, tape Digest) {
	hashPutDigest(s.c.h, id)
	hashPutU32(s.c.h, uint32(len(names)))
	for _, n := range names {
		hashPutString(s.c.h, n)
	}
	hashPutI64(s.c.h, wall)
	hashPutDigest(s.c.h, tape)
}

func (s *setupDatasetHasher) row(row SetupDatasetRow) {
	hashPutI64(s.c.h, row.At)
	hashPutString(s.c.h, row.Y)
	hashPutU32(s.c.h, uint32(len(row.X)))
	for _, v := range row.X {
		hashPutF64(s.c.h, v)
	}
	hashPutI64(s.c.h, int64(row.OOFFold))
}

func (s *setupDatasetHasher) counts(n int, firstAt, lastAt int64) {
	hashPutU32(s.c.h, uint32(n))
	hashPutI64(s.c.h, firstAt)
	hashPutI64(s.c.h, lastAt)
}

func (s *setupDatasetHasher) sum() Digest {
	return s.c.sum()
}

func percentilesP10P50P90(xs []float64) SetupDatasetPercentiles {
	var v []float64
	for _, x := range xs {
		if isFinite(x) {
			v = append(v, x)
		}
	}
	if len(v) == 0 {
		return SetupDatasetPercentiles{}
	}
	sort.Float64s(v)
	return SetupDatasetPercentiles{
		N: len(v), P10: qtile(v, 0.10), P50: qtile(v, 0.50), P90: qtile(v, 0.90),
	}
}

func FormatSetupDataset1(z SetupDataset1) string {
	var b strings.Builder
	b.WriteString("SETUP-DATASET-1 (DEV join only; no CatBoost; holdout sealed)\n")
	b.WriteString(fmt.Sprintf("format=%s width=%d logic=%s holdout_start=%d\n", z.Format, z.Width, SetupDatasetLogicJoinV1, z.HoldoutStartAt))
	b.WriteString(fmt.Sprintf("min_at=%d max_at=%d\n", z.MinCandidateAt, z.MaxCandidateAt))
	b.WriteString(fmt.Sprintf("feature_spec2=%s\nfeature_recipe=%s\nfeature_logic=%s\n", z.FeatureSpec2Digest, z.FeatureRecipeDigest, z.FeatureLogic))
	b.WriteString(fmt.Sprintf("setup_target=%s\nlabel_content=%s\n", z.SetupTargetDigest, z.LabelContentDigest))
	b.WriteString(fmt.Sprintf("validation_plan=%s\ndata_split=%s\nsetup_local=%s\n", z.ValidationPlanDigest, z.DataSplitDigest, z.SetupLocalDigest))
	b.WriteString(fmt.Sprintf("dataset_identity=%s\ncontent_digest=%s\n", z.DatasetDigest, z.ContentDigest))
	if z.TapeContentDigest != "" {
		b.WriteString(fmt.Sprintf("tape_content=%s\n", z.TapeContentDigest))
	}
	a := z.Accounting
	b.WriteString("\nPOPULATION\n")
	b.WriteString(fmt.Sprintf("  source DEV=%d  source HOLDOUT=%d (excluded; no 2026 Y or X stats)\n", a.SourceDEV, a.SourceHoldout))
	b.WriteString(fmt.Sprintf("  DEV %s=%d %s=%d %s=%d\n", SetupClassTP, a.DEVTP, SetupClassStop, a.DEVStop, SetupClassTimeout, a.DEVTimeout))
	b.WriteString(fmt.Sprintf("  DEV %s=%d %s=%d usable_labels=%d\n", SetupClassInvalid, a.DEVInvalid, SetupClassSkip, a.DEVSkip, a.DEVUsableLabels))
	b.WriteString(fmt.Sprintf("  exact FeatureTape2 joins=%d  %s=%d  missing=%d  dup_tape=%d dup_label=%d dup_asg=%d\n",
		a.ExactJoins, SetupClassFeatureNotReady, a.FeatureNotReady, a.MissingJoins, a.DuplicateTapeAt, a.DuplicateLabelAt, a.DuplicateAssignmentAt))
	b.WriteString(fmt.Sprintf("  final rows=%d  TP=%d STOP=%d TIMEOUT=%d  train_only=%d  holdout_usable_excluded=%d\n",
		a.FinalRows, a.FinalTP, a.FinalStop, a.FinalTimeout, a.TrainOnlyRows, a.HoldoutUsable))
	b.WriteString("\nOOF (frozen SETUP-VALIDATION-1 spans; FEATURE_NOT_READY may reduce n)\n")
	for _, f := range z.Folds {
		b.WriteString(fmt.Sprintf("  fold %d  frozen_val_n=%d  dataset_n=%d  TP=%d STOP=%d TIMEOUT=%d  not_ready=%d\n",
			f.Index, f.LabelValN, f.Rows, f.TP, f.Stop, f.Timeout, f.FeatureNotReady))
		b.WriteString(fmt.Sprintf("         val=[%d,%d]\n", f.ValFirstAt, f.ValLastAt))
	}
	b.WriteString("\nSETUP FACTS (DEV rows only; not split by class)\n")
	b.WriteString(fmt.Sprintf("  %s p10/p50/p90=%.3f/%.3f/%.3f n=%d\n", SetupFeatureRiskATR, z.RiskATR.P10, z.RiskATR.P50, z.RiskATR.P90, z.RiskATR.N))
	b.WriteString(fmt.Sprintf("  %s p10/p50/p90=%.1f/%.1f/%.1f n=%d\n", SetupFeatureS0AnchorAge, z.S0Age.P10, z.S0Age.P50, z.S0Age.P90, z.S0Age.N))
	b.WriteString("\nNOTES\n")
	b.WriteString("  - FeatureTape2 owns market context; STOP-2 owns ticket geometry; dataset only joins\n")
	b.WriteString("  - setup_risk_atr is copied ROverATR15 (buffered R / ATR14-on-15m); not ATR period 15\n")
	b.WriteString("  - s0_anchor_age_bars is copied AnchorAgeBars; ConfirmedAt age is not a feature\n")
	b.WriteString("  - MaxCandidateAt < holdout wall is a hard assertion\n")
	b.WriteString("  - no CatBoost, no 2026 class rates, no FeatureSpec3\n")
	return b.String()
}
