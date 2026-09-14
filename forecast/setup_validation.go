package forecast

import (
	"fmt"
	"strings"
)

// SetupValidationLogicWalkForwardV1 is event-scale expanding walk-forward for
// SETUP-TARGET-1. Same CompileValidationPlan seam (HorizonEnd < val start).
const SetupValidationLogicWalkForwardV1 LogicVersion = "setup-validation:walk-forward-v1"

const (
	// SetupValidationSpanBars is 365 × 96 fifteen-minute bars (market clock).
	SetupValidationSpanBars       = 35040
	SetupValidationFoldCount      = 4
	SetupValidationMinTrainEvents = 1000
)

// SetupValidationHoldoutStartAt is the current book wall from DataSplitPolicy.
func SetupValidationHoldoutStartAt() int64 {
	return DefaultDataSplitPolicy().HoldoutStartAt
}

// FrozenSetupValidationPlan is the SETUP-VALIDATION-1 rule set.
func FrozenSetupValidationPlan() (ValidationPlan, error) {
	tgt := FrozenSetupTarget1()
	split := DefaultDataSplitPolicy()
	return ResolveSetupValidationPlan(ValidationPlanDraft{
		Timeframe:          tgt.PrimaryTF,
		HoldoutStartAt:     split.HoldoutStartAt,
		ValidationSpanBars: SetupValidationSpanBars,
		FoldCount:          SetupValidationFoldCount,
		ExtraGapBars:       0,
		MinTrainRows:       SetupValidationMinTrainEvents,
	}, tgt, SetupValidationLogicWalkForwardV1)
}

// ResolveSetupValidationPlan copies TargetH from SETUP-TARGET-1, not TargetSpec.
func ResolveSetupValidationPlan(draft ValidationPlanDraft, tgt SetupTarget1, logic LogicVersion) (ValidationPlan, error) {
	if err := tgt.validate(); err != nil {
		return ValidationPlan{}, err
	}
	if logic == "" {
		return ValidationPlan{}, fmt.Errorf("forecast: setup validation plan requires a LogicVersion")
	}
	p := ValidationPlan{
		Logic:              logic,
		Timeframe:          draft.Timeframe,
		HoldoutStartAt:     draft.HoldoutStartAt,
		ValidationSpanBars: draft.ValidationSpanBars,
		FoldCount:          draft.FoldCount,
		TargetH:            tgt.Horizon,
		ExtraGapBars:       draft.ExtraGapBars,
		MinTrainRows:       draft.MinTrainRows,
	}
	if err := p.validateRules(); err != nil {
		return ValidationPlan{}, err
	}
	if p.TargetH != StructuralStopHorizon {
		return ValidationPlan{}, fmt.Errorf("forecast: SETUP-VALIDATION-1 TargetH must be %d", StructuralStopHorizon)
	}
	if p.MinTrainRows >= 35040 {
		return ValidationPlan{}, fmt.Errorf("forecast: SETUP-VALIDATION-1 MinTrainRows is event-scale, not dense-bar 35040")
	}
	return p, nil
}

func setupTrainableAts(rows []SetupLabelRow) []int64 {
	var at []int64
	for _, r := range rows {
		switch r.Class {
		case SetupClassTP, SetupClassStop, SetupClassTimeout:
			at = append(at, r.At)
		}
	}
	return at
}

// SetupValidationReport is outcome-blind geometry plus a post-hoc class census
// of OOF folds. Holdout class rates are not printed (sealed).
type SetupValidationReport struct {
	Plan           ValidationPlan         `json:"plan"`
	Split          DataSplitPolicy        `json:"data_split"`
	SplitDigest    string                 `json:"data_split_digest"`
	PlanDigest     string                 `json:"validation_plan_digest"`
	Compiled       CompiledValidationPlan `json:"compiled"`
	Text           string                 `json:"report"`
	Folds          []SetupValidationFold  `json:"folds"`
	MaxOOFAt       int64                  `json:"max_oof_at"`
	HoldoutUsableN int                    `json:"holdout_usable_n"`
	HoldoutSourceN int                    `json:"holdout_source_n"`
	HoldoutN       int                    `json:"holdout_n"`
}

type SetupValidationFold struct {
	Index          int   `json:"index"`
	TrainN         int   `json:"train_n"`
	ValN           int   `json:"val_n"`
	ValTP          int   `json:"val_tp"`
	ValStop        int   `json:"val_stop"`
	ValTimeout     int   `json:"val_timeout"`
	TrainLastAt    int64 `json:"train_last_at"`
	ValFirstAt     int64 `json:"val_first_at"`
	ValLastAt      int64 `json:"val_last_at"`
	ValStartMarket int64 `json:"val_boundary_start_at"`
	ValEndMarket   int64 `json:"val_boundary_end_at"`
}

// CompileSetupValidation1 compiles H=72 walk-forward on trainable SETUP-LABELSET-1
// timestamps. INVALID / NOT_EVALUABLE are not fold rows.
func CompileSetupValidation1(ls SetupLabelSet1) (SetupValidationReport, error) {
	var z SetupValidationReport
	if !ls.MatchOK {
		return z, fmt.Errorf("forecast: SETUP-VALIDATION-1 requires MOVE-POTENTIAL +2R MATCH labels")
	}
	plan, err := FrozenSetupValidationPlan()
	if err != nil {
		return z, err
	}
	split := DefaultDataSplitPolicy()
	splitID, err := split.Identity()
	if err != nil {
		return z, err
	}
	planID, err := plan.Identity()
	if err != nil {
		return z, err
	}
	ats := setupTrainableAts(ls.Rows)
	compiled, err := CompileValidationPlan(ats, plan.Timeframe, plan)
	if err != nil {
		return z, err
	}
	byAt := map[int64]string{}
	for _, r := range ls.Rows {
		byAt[r.At] = r.Class
	}
	z.Plan = plan
	z.Split = split
	z.SplitDigest = splitID.Digest.String()
	z.PlanDigest = planID.Digest.String()
	z.Compiled = compiled
	z.HoldoutUsableN = len(ats) - compiled.HoldoutBeginIndex
	if z.HoldoutUsableN < 0 {
		z.HoldoutUsableN = 0
	}
	z.HoldoutN = z.HoldoutUsableN
	for _, r := range ls.Rows {
		if r.At >= split.HoldoutStartAt {
			z.HoldoutSourceN++
		}
	}
	z.Folds = make([]SetupValidationFold, len(compiled.Folds))
	for i, f := range compiled.Folds {
		if f.ValLastAt >= split.HoldoutStartAt {
			return z, fmt.Errorf("forecast: OOF fold %d val_last %d crosses holdout wall %d", i, f.ValLastAt, split.HoldoutStartAt)
		}
		sf := SetupValidationFold{
			Index: i, TrainN: f.TrainEnd - f.TrainBegin, ValN: f.ValEnd - f.ValBegin,
			TrainLastAt: f.TrainLastAt, ValFirstAt: f.ValFirstAt, ValLastAt: f.ValLastAt,
			ValStartMarket: f.ValBoundaryStartAt, ValEndMarket: f.ValBoundaryEndAt,
		}
		for j := f.ValBegin; j < f.ValEnd; j++ {
			if err := split.RefuseHoldoutAt(ats[j]); err != nil {
				return z, err
			}
			switch byAt[ats[j]] {
			case SetupClassTP:
				sf.ValTP++
			case SetupClassStop:
				sf.ValStop++
			case SetupClassTimeout:
				sf.ValTimeout++
			}
		}
		z.Folds[i] = sf
		if sf.ValLastAt > z.MaxOOFAt {
			z.MaxOOFAt = sf.ValLastAt
		}
	}
	if z.MaxOOFAt >= split.HoldoutStartAt {
		return z, fmt.Errorf("forecast: MaxOOFAt %d is not < holdout wall %d", z.MaxOOFAt, split.HoldoutStartAt)
	}
	z.Text = FormatSetupValidation1(z)
	return z, nil
}

// FormatSetupValidation1 is the chapter report. No model. No holdout TP rates.
func FormatSetupValidation1(z SetupValidationReport) string {
	var b strings.Builder
	b.WriteString("SETUP-VALIDATION-1 (event-scale walk-forward; no CatBoost)\n")
	b.WriteString(fmt.Sprintf("logic=%s H=%d span_bars=%d folds=%d min_train_events=%d holdout_start=%d\n",
		z.Plan.Logic, z.Plan.TargetH, z.Plan.ValidationSpanBars, z.Plan.FoldCount, z.Plan.MinTrainRows, z.Plan.HoldoutStartAt))
	b.WriteString(fmt.Sprintf("split_digest=%s\nplan_digest=%s\nmax_oof_at=%d\n", z.SplitDigest, z.PlanDigest, z.MaxOOFAt))
	b.WriteString("OOF is 2022–2025 inside DEV. 2026-to-today is HOLDOUT (sealed). Defaults mint new identities; they do not rewrite old artifacts.\n")
	b.WriteString("seam: HorizonEnd(train.At,15m,72) < validation_start. Overlapping events kept.\n\n")
	for _, f := range z.Folds {
		b.WriteString(fmt.Sprintf("fold %d  train_n=%d  val_n=%d  TP=%d STOP=%d TIMEOUT=%d  TP_rate=%.3f\n",
			f.Index, f.TrainN, f.ValN, f.ValTP, f.ValStop, f.ValTimeout, cellRate(f.ValTP, f.ValN)))
		b.WriteString(fmt.Sprintf("         train_last=%d  val_first=%d val_last=%d\n", f.TrainLastAt, f.ValFirstAt, f.ValLastAt))
		b.WriteString(fmt.Sprintf("         market_val=[%d,%d)\n", f.ValStartMarket, f.ValEndMarket))
	}
	b.WriteString(fmt.Sprintf("\nholdout_usable_n=%d  holdout_source_n=%d (usable = TP/STOP/TIMEOUT; source = all LONG events; no class rates)\n", z.HoldoutUsableN, z.HoldoutSourceN))
	b.WriteString("\nNOTES\n")
	b.WriteString("  - R/ATR15 and S0 age are SETUP-DATASET-1 ticket facts, not this chapter\n")
	b.WriteString("  - operating coverage later is 50/20/10/5%; 1% tail is diagnostic only\n")
	b.WriteString("  - one shared risk/outcome law; 50-cross is the first ignition, not a per-signal factory\n")
	return b.String()
}
