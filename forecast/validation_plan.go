package forecast

import (
	"fmt"
	"math"
	"sort"

	"trading_bot/data"
)

const ValidationLogicWalkForwardV1 LogicVersion = "validation:walk-forward-v1"

// ValidationPlanDraft is mutable input. TargetH is NOT a draft field — it is
// copied from TargetSpec at resolve time.
type ValidationPlanDraft struct {
	Timeframe          string
	HoldoutStartAt     int64
	ValidationSpanBars int
	FoldCount          int
	ExtraGapBars       int
	MinTrainRows       int
}

// ValidationPlan is the resolved, immutable walk-forward rule set.
type ValidationPlan struct {
	Logic              LogicVersion
	Timeframe          string
	HoldoutStartAt     int64
	ValidationSpanBars int
	FoldCount          int
	TargetH            int
	ExtraGapBars       int
	MinTrainRows       int
}

// CompiledFold is one expanding walk-forward split as index ranges into At[].
type CompiledFold struct {
	TrainBegin int
	TrainEnd   int
	ValBegin   int
	ValEnd     int

	ValBoundaryStartAt int64
	ValBoundaryEndAt   int64
	TrainFirstAt       int64
	TrainLastAt        int64
	ValFirstAt         int64
	ValLastAt          int64
}

// CompiledValidationPlan is the deterministic geometry for one At[] + plan.
type CompiledValidationPlan struct {
	Plan                      Identity
	DevelopmentExclusiveEndAt int64
	DevelopmentEndIndex       int
	HoldoutStartAt            int64
	HoldoutBeginIndex         int
	Folds                     []CompiledFold
}

type validationIdentityPayload struct {
	Logic              LogicVersion
	Timeframe          string
	HoldoutStartAt     int64
	ValidationSpanBars int
	FoldCount          int
	TargetH            int
	ExtraGapBars       int
	MinTrainRows       int
}

// ResolveValidationPlan copies TargetH from spec and validates rule fields.
func ResolveValidationPlan(draft ValidationPlanDraft, spec TargetSpec, logic LogicVersion) (ValidationPlan, error) {
	if logic == "" {
		return ValidationPlan{}, fmt.Errorf("forecast: validation plan requires a LogicVersion")
	}
	p := ValidationPlan{
		Logic:              logic,
		Timeframe:          draft.Timeframe,
		HoldoutStartAt:     draft.HoldoutStartAt,
		ValidationSpanBars: draft.ValidationSpanBars,
		FoldCount:          draft.FoldCount,
		TargetH:            spec.HorizonBars,
		ExtraGapBars:       draft.ExtraGapBars,
		MinTrainRows:       draft.MinTrainRows,
	}
	if err := p.validateRules(); err != nil {
		return ValidationPlan{}, err
	}
	return p, nil
}

func (p ValidationPlan) TotalCausalBars() (int, error) {
	if p.TargetH <= 0 || p.ExtraGapBars < 0 {
		return 0, fmt.Errorf("forecast: invalid causal bars TargetH=%d ExtraGapBars=%d", p.TargetH, p.ExtraGapBars)
	}
	if p.ExtraGapBars > math.MaxInt-p.TargetH {
		return 0, fmt.Errorf("forecast: TargetH + ExtraGapBars overflow")
	}
	return p.TargetH + p.ExtraGapBars, nil
}

func (p ValidationPlan) Identity() (Identity, error) {
	if err := p.validateRules(); err != nil {
		return Identity{}, err
	}
	return NewIdentity(string(p.Logic)+"-"+p.Timeframe, validationIdentityPayload{
		Logic:              p.Logic,
		Timeframe:          p.Timeframe,
		HoldoutStartAt:     p.HoldoutStartAt,
		ValidationSpanBars: p.ValidationSpanBars,
		FoldCount:          p.FoldCount,
		TargetH:            p.TargetH,
		ExtraGapBars:       p.ExtraGapBars,
		MinTrainRows:       p.MinTrainRows,
	}, p.Logic)
}

func (p ValidationPlan) validateRules() error {
	if p.Logic == "" {
		return fmt.Errorf("forecast: validation plan LogicVersion is required")
	}
	if p.Timeframe == "" {
		return fmt.Errorf("forecast: validation plan Timeframe is required")
	}
	if _, err := data.CurrentBarOpen(p.HoldoutStartAt, p.Timeframe); err != nil {
		return fmt.Errorf("forecast: validation plan Timeframe %q: %w", p.Timeframe, err)
	}
	if p.HoldoutStartAt <= 0 {
		return fmt.Errorf("forecast: HoldoutStartAt must be > 0")
	}
	if p.ValidationSpanBars <= 0 {
		return fmt.Errorf("forecast: ValidationSpanBars must be > 0")
	}
	if p.FoldCount <= 0 {
		return fmt.Errorf("forecast: FoldCount must be > 0")
	}
	if p.TargetH <= 0 {
		return fmt.Errorf("forecast: TargetH must be > 0")
	}
	if p.ExtraGapBars < 0 {
		return fmt.Errorf("forecast: ExtraGapBars must be >= 0")
	}
	if p.MinTrainRows <= 0 {
		return fmt.Errorf("forecast: MinTrainRows must be > 0")
	}
	if _, err := p.TotalCausalBars(); err != nil {
		return err
	}
	return nil
}

// CompileValidationPlan is the sole walk-forward geometry owner.
// at must be strictly increasing trainable timestamps. It never sees y or X.
func CompileValidationPlan(at []int64, tf string, plan ValidationPlan) (CompiledValidationPlan, error) {
	var z CompiledValidationPlan
	if err := plan.validateRules(); err != nil {
		return z, err
	}
	if tf != plan.Timeframe {
		return z, fmt.Errorf("forecast: caller timeframe %q != ValidationPlan.Timeframe %q", tf, plan.Timeframe)
	}
	open, err := data.CurrentBarOpen(plan.HoldoutStartAt, tf)
	if err != nil {
		return z, err
	}
	if open != plan.HoldoutStartAt {
		return z, fmt.Errorf("forecast: HoldoutStartAt %d is off the %s grid (floor %d)", plan.HoldoutStartAt, tf, open)
	}
	if err := requireStrictlyIncreasingAt(at); err != nil {
		return z, err
	}
	causal, err := plan.TotalCausalBars()
	if err != nil {
		return z, err
	}
	id, err := plan.Identity()
	if err != nil {
		return z, err
	}
	devEnd, err := hopPrevOpen(plan.HoldoutStartAt, tf, causal)
	if err != nil {
		return z, err
	}
	valStart := make([]int64, plan.FoldCount)
	valEnd := make([]int64, plan.FoldCount)
	end := devEnd
	for i := plan.FoldCount - 1; i >= 0; i-- {
		start, err := hopPrevOpen(end, tf, plan.ValidationSpanBars)
		if err != nil {
			return z, err
		}
		if start >= end {
			return z, fmt.Errorf("forecast: fold %d empty market-time validation window", i)
		}
		valStart[i], valEnd[i] = start, end
		end = start
	}
	lb := func(b int64) int {
		return sort.Search(len(at), func(i int) bool { return at[i] >= b })
	}
	holdoutIdx := lb(plan.HoldoutStartAt)
	if holdoutIdx == len(at) {
		return z, fmt.Errorf("forecast: empty final holdout")
	}
	devIdx := lb(devEnd)
	illegalTrain := make([]int64, plan.FoldCount)
	for i := 0; i < plan.FoldCount; i++ {
		ill, err := hopPrevOpen(valStart[i], tf, causal)
		if err != nil {
			return z, err
		}
		illegalTrain[i] = ill
	}
	folds := make([]CompiledFold, plan.FoldCount)
	for i := 0; i < plan.FoldCount; i++ {
		v0, v1 := lb(valStart[i]), lb(valEnd[i])
		if v1 <= v0 {
			return z, fmt.Errorf("forecast: fold %d empty validation range", i)
		}
		tr1 := lb(illegalTrain[i])
		if tr1 <= 0 {
			return z, fmt.Errorf("forecast: fold %d empty train", i)
		}
		trainLastH, err := HorizonEnd(at[tr1-1], tf, causal)
		if err != nil {
			return z, err
		}
		if trainLastH >= valStart[i] {
			return z, fmt.Errorf("forecast: fold %d HorizonEnd(TrainLastAt)=%d is not < ValidationStart %d", i, trainLastH, valStart[i])
		}
		for j := v0; j < v1; j++ {
			if at[j] >= plan.HoldoutStartAt {
				return z, fmt.Errorf("forecast: fold %d validation includes holdout At=%d", i, at[j])
			}
			vh, err := HorizonEnd(at[j], tf, causal)
			if err != nil {
				return z, err
			}
			if vh >= plan.HoldoutStartAt {
				return z, fmt.Errorf("forecast: fold %d validation row At=%d crosses holdout wall", i, at[j])
			}
		}
		if tr1 > holdoutIdx {
			return z, fmt.Errorf("forecast: fold %d train reaches holdout", i)
		}
		folds[i] = CompiledFold{
			TrainBegin:         0,
			TrainEnd:           tr1,
			ValBegin:           v0,
			ValEnd:             v1,
			ValBoundaryStartAt: valStart[i],
			ValBoundaryEndAt:   valEnd[i],
			TrainFirstAt:       at[0],
			TrainLastAt:        at[tr1-1],
			ValFirstAt:         at[v0],
			ValLastAt:          at[v1-1],
		}
	}
	if folds[0].TrainEnd-folds[0].TrainBegin < plan.MinTrainRows {
		return z, fmt.Errorf("forecast: Fold0 train rows %d < MinTrainRows %d", folds[0].TrainEnd-folds[0].TrainBegin, plan.MinTrainRows)
	}
	for i := 0; i < plan.FoldCount; i++ {
		for j := i + 1; j < plan.FoldCount; j++ {
			if folds[i].ValEnd > folds[j].ValBegin && folds[j].ValEnd > folds[i].ValBegin {
				return z, fmt.Errorf("forecast: validation ranges overlap fold %d and %d", i, j)
			}
		}
		if i > 0 && folds[i-1].ValBoundaryEndAt != folds[i].ValBoundaryStartAt {
			return z, fmt.Errorf("forecast: validation windows are not contiguous in market time")
		}
		if folds[i].ValEnd > holdoutIdx || folds[i].ValBegin >= holdoutIdx {
			return z, fmt.Errorf("forecast: fold %d validation reaches holdout", i)
		}
	}
	if folds[plan.FoldCount-1].ValBoundaryEndAt != devEnd {
		return z, fmt.Errorf("forecast: last validation end %d != development exclusive end %d", folds[plan.FoldCount-1].ValBoundaryEndAt, devEnd)
	}
	return CompiledValidationPlan{
		Plan:                      id,
		DevelopmentExclusiveEndAt: devEnd,
		DevelopmentEndIndex:       devIdx,
		HoldoutStartAt:            plan.HoldoutStartAt,
		HoldoutBeginIndex:         holdoutIdx,
		Folds:                     folds,
	}, nil
}

func requireStrictlyIncreasingAt(at []int64) error {
	if len(at) == 0 {
		return fmt.Errorf("forecast: validation At[] is empty")
	}
	for i := 1; i < len(at); i++ {
		if at[i] <= at[i-1] {
			return fmt.Errorf("forecast: At not strictly increasing index=%d got %d after %d", i, at[i], at[i-1])
		}
	}
	return nil
}

func hopPrevOpen(at int64, tf string, n int) (int64, error) {
	if n < 0 {
		return 0, fmt.Errorf("forecast: hop count must be >= 0")
	}
	t := at
	for i := 0; i < n; i++ {
		p, err := data.PreviousBarOpen(t, tf)
		if err != nil {
			return 0, err
		}
		if p >= t {
			return 0, fmt.Errorf("forecast: PreviousBarOpen did not go backward from %d", t)
		}
		t = p
	}
	return t, nil
}
