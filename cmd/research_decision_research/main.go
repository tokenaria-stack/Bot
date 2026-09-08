// Thin stdin/stdout bridge: ForecastEvidence + Outcome + fold ranges → selector.
// No At. Outcome is evaluator truth only and is never passed to ApplyDecision.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"trading_bot/decision"
	"trading_bot/decisionresearch"
	"trading_bot/forecast"
	"trading_bot/market"
)

type foldIn struct {
	TrainBegin int `json:"train_begin"`
	TrainEnd   int `json:"train_end"`
	ValBegin   int `json:"val_begin"`
	ValEnd     int `json:"val_end"`
}

type researchIn struct {
	TargetDigest string    `json:"target_digest"`
	PUp          []float64 `json:"p_up"`
	PDown        []float64 `json:"p_down"`
	PTimeout     []float64 `json:"p_timeout"`
	Rank         []float64 `json:"rank"`
	Outcome      []string  `json:"outcome"`
	Folds        []foldIn  `json:"folds"`
}

type specOut struct {
	Logic                    string   `json:"logic"`
	TargetDigest             string   `json:"target_digest"`
	UpperBarrierUtility      float64  `json:"upper_barrier_utility"`
	LowerBarrierUtility      float64  `json:"lower_barrier_utility"`
	MinExpectedTargetUtility float64  `json:"min_expected_target_utility"`
	MinAbsDirectionalRank    float64  `json:"min_abs_directional_rank"`
	ClassOrder               []string `json:"class_order"`
}

type auditOut struct {
	N            int     `json:"n"`
	UpIntent     int     `json:"up_intent"`
	DownIntent   int     `json:"down_intent"`
	Abstain      int     `json:"abstain"`
	UpUp         int     `json:"up_intent_up_first"`
	UpDown       int     `json:"up_intent_down_first"`
	UpTO         int     `json:"up_intent_timeout"`
	DownUp       int     `json:"down_intent_up_first"`
	DownDown     int     `json:"down_intent_down_first"`
	DownTO       int     `json:"down_intent_timeout"`
	AbsUp        int     `json:"abstain_up_first"`
	AbsDown      int     `json:"abstain_down_first"`
	AbsTO        int     `json:"abstain_timeout"`
	TotalUtility float64 `json:"total_utility"`
}

type selOut struct {
	Kind          string   `json:"kind"`
	UtilityDecile int      `json:"utility_decile,omitempty"`
	RankDecile    int      `json:"rank_decile,omitempty"`
	Spec          *specOut `json:"spec,omitempty"`
}

type foldOut struct {
	FoldIndex            int      `json:"fold_index"`
	Selection            selOut   `json:"selection"`
	Train                auditOut `json:"train_audit"`
	Validation           auditOut `json:"validation_audit"`
	ValidationPositive   bool     `json:"validation_positive"`
	ValidationApplyCalls int      `json:"validation_apply_calls"`
}

type researchOut struct {
	Logic                   string    `json:"decision_research_logic_version"`
	DecisionLogic           string    `json:"decision_logic_version"`
	TargetDigest            string    `json:"target_digest"`
	UpperBarrierUtility     float64   `json:"upper_barrier_utility"`
	LowerBarrierUtility     float64   `json:"lower_barrier_utility"`
	ClassOrder              []string  `json:"class_order"`
	ApplyCalls              int       `json:"apply_calls"`
	Folds                   []foldOut `json:"folds"`
	Pooled                  auditOut  `json:"pooled_validation"`
	PositiveFoldCount       int       `json:"positive_fold_count"`
	FoldCount               int       `json:"fold_count"`
	EligibleForFinalization bool      `json:"eligible_for_finalization"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "research_decision_research: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dec := json.NewDecoder(os.Stdin)
	dec.DisallowUnknownFields()
	var in researchIn
	if err := dec.Decode(&in); err != nil {
		return fmt.Errorf("stdin: %w", err)
	}
	spec, err := market.ResearchTargetSpec()
	if err != nil {
		return err
	}
	id, err := spec.Identity()
	if err != nil {
		return err
	}
	want, err := forecast.ParseDigestHex(in.TargetDigest)
	if err != nil {
		return err
	}
	if id.Digest != want {
		return fmt.Errorf("target_digest mismatch")
	}
	n := len(in.Outcome)
	if len(in.PUp) != n || len(in.PDown) != n || len(in.PTimeout) != n || len(in.Rank) != n {
		return fmt.Errorf("array length")
	}
	ev := make([]decisionresearch.EvidenceRow, n)
	for i := 0; i < n; i++ {
		ev[i] = decisionresearch.EvidenceRow{Evidence: decision.ForecastEvidence{
			Probabilities:   [3]float64{in.PUp[i], in.PDown[i], in.PTimeout[i]},
			DirectionalRank: in.Rank[i],
		}}
	}
	folds := make([]decisionresearch.FoldRange, len(in.Folds))
	for i, f := range in.Folds {
		folds[i] = decisionresearch.FoldRange{
			TrainBegin: f.TrainBegin, TrainEnd: f.TrainEnd,
			ValBegin: f.ValBegin, ValEnd: f.ValEnd,
		}
	}
	co := [3]string{string(forecast.OutcomeUpFirst), string(forecast.OutcomeDownFirst), string(forecast.OutcomeTimeout)}
	decisionresearch.ApplyCalls = 0
	res, err := decisionresearch.Run(ev, in.Outcome, folds, decisionresearch.World{
		TargetDigest: want,
		U:            spec.UpperATRMultiple,
		L:            spec.LowerATRMultiple,
		ClassOrder:   co,
		Logic:        decision.DecisionLogicTargetUtilityRankGateV1,
	})
	if err != nil {
		return err
	}
	out := researchOut{
		Logic:                   decisionresearch.LogicV1,
		DecisionLogic:           string(decision.DecisionLogicTargetUtilityRankGateV1),
		TargetDigest:            id.Digest.String(),
		UpperBarrierUtility:     spec.UpperATRMultiple,
		LowerBarrierUtility:     spec.LowerATRMultiple,
		ClassOrder:              co[:],
		ApplyCalls:              decisionresearch.ApplyCalls,
		Folds:                   make([]foldOut, len(res.Folds)),
		Pooled:                  auditJSON(res.PooledVal),
		PositiveFoldCount:       res.PositiveFoldCount,
		FoldCount:               res.FoldCount,
		EligibleForFinalization: res.EligibleForFinalization,
	}
	for i, f := range res.Folds {
		out.Folds[i] = foldOut{
			FoldIndex:            f.Index,
			Selection:            selJSON(f.Sel),
			Train:                auditJSON(f.Train),
			Validation:           auditJSON(f.Val),
			ValidationPositive:   f.ValidationPositive,
			ValidationApplyCalls: f.ValApplyCalls,
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}

func auditJSON(a decisionresearch.Audit) auditOut {
	return auditOut{
		N: a.N, UpIntent: a.UpIntent, DownIntent: a.DownIntent, Abstain: a.Abstain,
		UpUp: a.UpUp, UpDown: a.UpDown, UpTO: a.UpTO,
		DownUp: a.DownUp, DownDown: a.DownDown, DownTO: a.DownTO,
		AbsUp: a.AbsUp, AbsDown: a.AbsDown, AbsTO: a.AbsTO,
		TotalUtility: a.TotalUtility,
	}
}

func selJSON(s decisionresearch.Selection) selOut {
	o := selOut{Kind: s.Kind}
	if s.Kind != decisionresearch.KindSpec {
		return o
	}
	o.UtilityDecile = s.UtilityDecile
	o.RankDecile = s.RankDecile
	sp := s.Spec
	o.Spec = &specOut{
		Logic:                    string(sp.Logic),
		TargetDigest:             sp.TargetDigest.String(),
		UpperBarrierUtility:      sp.UpperBarrierUtility,
		LowerBarrierUtility:      sp.LowerBarrierUtility,
		MinExpectedTargetUtility: sp.MinExpectedTargetUtility,
		MinAbsDirectionalRank:    sp.MinAbsDirectionalRank,
		ClassOrder:               sp.ClassOrder[:],
	}
	return o
}
