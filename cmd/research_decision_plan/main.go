// Thin stdin/stdout bridge: validated At[] only → CompileValidationPlan.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"trading_bot/market"
)

type compileIn struct {
	At []int64 `json:"at"`
}

type foldOut struct {
	TrainBegin         int   `json:"train_begin"`
	TrainEnd           int   `json:"train_end"`
	ValBegin           int   `json:"val_begin"`
	ValEnd             int   `json:"val_end"`
	ValBoundaryStartAt int64 `json:"val_boundary_start_at"`
	ValBoundaryEndAt   int64 `json:"val_boundary_end_at"`
	TrainFirstAt       int64 `json:"train_first_at"`
	TrainLastAt        int64 `json:"train_last_at"`
	ValFirstAt         int64 `json:"val_first_at"`
	ValLastAt          int64 `json:"val_last_at"`
}

type compileOut struct {
	Logic                     string    `json:"logic"`
	Timeframe                 string    `json:"timeframe"`
	HoldoutStartAt            int64     `json:"holdout_start_at"`
	ValidationSpanBars        int       `json:"validation_span_bars"`
	FoldCount                 int       `json:"fold_count"`
	MinTrainRows              int       `json:"min_train_rows"`
	TargetH                   int       `json:"target_h"`
	ExtraGapBars              int       `json:"extra_gap_bars"`
	TotalCausalBars           int       `json:"total_causal_bars"`
	TargetDigest              string    `json:"target_digest"`
	DevelopmentExclusiveEndAt int64     `json:"development_exclusive_end_at"`
	DevelopmentEndIndex       int       `json:"development_end_index"`
	HoldoutBeginIndex         int       `json:"holdout_begin_index"`
	Folds                     []foldOut `json:"folds"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "research_decision_plan: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dec := json.NewDecoder(os.Stdin)
	dec.DisallowUnknownFields()
	var in compileIn
	if err := dec.Decode(&in); err != nil {
		return fmt.Errorf("stdin: %w", err)
	}
	compiled, plan, spec, err := market.CompileResearchDecisionValidation(in.At)
	if err != nil {
		return err
	}
	tid, err := spec.Identity()
	if err != nil {
		return err
	}
	causal, err := plan.TotalCausalBars()
	if err != nil {
		return err
	}
	out := compileOut{
		Logic:                     string(plan.Logic),
		Timeframe:                 plan.Timeframe,
		HoldoutStartAt:            plan.HoldoutStartAt,
		ValidationSpanBars:        plan.ValidationSpanBars,
		FoldCount:                 plan.FoldCount,
		MinTrainRows:              plan.MinTrainRows,
		TargetH:                   plan.TargetH,
		ExtraGapBars:              plan.ExtraGapBars,
		TotalCausalBars:           causal,
		TargetDigest:              tid.Digest.String(),
		DevelopmentExclusiveEndAt: compiled.DevelopmentExclusiveEndAt,
		DevelopmentEndIndex:       compiled.DevelopmentEndIndex,
		HoldoutBeginIndex:         compiled.HoldoutBeginIndex,
		Folds:                     make([]foldOut, len(compiled.Folds)),
	}
	for i, f := range compiled.Folds {
		out.Folds[i] = foldOut{
			TrainBegin:         f.TrainBegin,
			TrainEnd:           f.TrainEnd,
			ValBegin:           f.ValBegin,
			ValEnd:             f.ValEnd,
			ValBoundaryStartAt: f.ValBoundaryStartAt,
			ValBoundaryEndAt:   f.ValBoundaryEndAt,
			TrainFirstAt:       f.TrainFirstAt,
			TrainLastAt:        f.TrainLastAt,
			ValFirstAt:         f.ValFirstAt,
			ValLastAt:          f.ValLastAt,
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}
