package decisionresearch

import (
	"fmt"
	"math"

	"trading_bot/decision"
	"trading_bot/forecast"
)

const (
	LogicV1             = "decision-research:fixed-grid-v1"
	KindBaseline        = "ABSTAIN_BASELINE"
	KindSpec            = "DECISION_SPEC"
	OutcomeUp           = "UP_FIRST"
	OutcomeDown         = "DOWN_FIRST"
	OutcomeTimeout      = "TIMEOUT"
	ThresholdResolution = "base=min(U,L); min_EU=base*(float64(du)/10.0); min_rank=float64(dr)/10.0"
	ObjectiveLaw        = "maximize_train_TotalUtility_all_rows"
	BaselineLaw         = "ABSTAIN_BASELINE TotalUtility=0; replace iff candidate TotalUtility>incumbent"
	TieLaw              = "equal_positive_TotalUtility: higher utility_decile then higher rank_decile"
	ProtocolLaw         = "train_select_one; validation_evaluate_selected_only"
)

// ApplyCalls counts decision.ApplyDecision invocations in this process (tests/spies).
var ApplyCalls int

type EvidenceRow struct {
	Evidence decision.ForecastEvidence
}

type FoldRange struct {
	TrainBegin, TrainEnd int
	ValBegin, ValEnd     int
}

type World struct {
	TargetDigest forecast.Digest
	U, L         float64
	ClassOrder   [3]string
	Logic        forecast.LogicVersion
}

type Audit struct {
	N                             int
	UpIntent, DownIntent, Abstain int
	UpUp, UpDown, UpTO            int
	DownUp, DownDown, DownTO      int
	AbsUp, AbsDown, AbsTO         int
	TotalUtility                  float64
}

type Selection struct {
	Kind          string
	UtilityDecile int
	RankDecile    int
	Spec          decision.DecisionSpec
}

type FoldResult struct {
	Index              int
	Sel                Selection
	Train, Val         Audit
	ValidationPositive bool
	ValApplyCalls      int
}

type Result struct {
	Folds                   []FoldResult
	PooledVal               Audit
	PositiveFoldCount       int
	FoldCount               int
	EligibleForFinalization bool
}

func classOrder() [3]string {
	return [3]string{OutcomeUp, OutcomeDown, OutcomeTimeout}
}

func ResolveThresholds(u, l float64, du, dr int) (minEU, minRank float64) {
	base := math.Min(u, l)
	minEU = base * (float64(du) / 10.0)
	minRank = float64(dr) / 10.0
	return minEU, minRank
}

func materializeSpec(w World, du, dr int) (decision.DecisionSpec, error) {
	minEU, minRank := ResolveThresholds(w.U, w.L, du, dr)
	s := decision.DecisionSpec{
		Logic:                    w.Logic,
		TargetDigest:             w.TargetDigest,
		UpperBarrierUtility:      w.U,
		LowerBarrierUtility:      w.L,
		MinExpectedTargetUtility: minEU,
		MinAbsDirectionalRank:    minRank,
		ClassOrder:               w.ClassOrder,
	}
	if err := decision.ValidateDecisionSpec(s); err != nil {
		return decision.DecisionSpec{}, err
	}
	return s, nil
}

func apply(e decision.ForecastEvidence, s decision.DecisionSpec) (decision.DirectionalIntent, error) {
	ApplyCalls++
	return decision.ApplyDecision(e, s)
}

func TotalUtility(u, l float64, a Audit) float64 {
	return u*float64(a.UpUp-a.DownUp) + l*float64(a.DownDown-a.UpDown)
}

func addCell(a *Audit, intent decision.DirectionalIntent, outcome string) error {
	a.N++
	switch intent {
	case decision.IntentUp:
		a.UpIntent++
		switch outcome {
		case OutcomeUp:
			a.UpUp++
		case OutcomeDown:
			a.UpDown++
		case OutcomeTimeout:
			a.UpTO++
		default:
			return fmt.Errorf("decisionresearch: unknown outcome %q", outcome)
		}
	case decision.IntentDown:
		a.DownIntent++
		switch outcome {
		case OutcomeUp:
			a.DownUp++
		case OutcomeDown:
			a.DownDown++
		case OutcomeTimeout:
			a.DownTO++
		default:
			return fmt.Errorf("decisionresearch: unknown outcome %q", outcome)
		}
	case decision.IntentAbstain:
		a.Abstain++
		switch outcome {
		case OutcomeUp:
			a.AbsUp++
		case OutcomeDown:
			a.AbsDown++
		case OutcomeTimeout:
			a.AbsTO++
		default:
			return fmt.Errorf("decisionresearch: unknown outcome %q", outcome)
		}
	default:
		return fmt.Errorf("decisionresearch: unknown intent %q", intent)
	}
	return nil
}

func (a Audit) check() error {
	if a.UpIntent+a.DownIntent+a.Abstain != a.N {
		return fmt.Errorf("decisionresearch: intent counts != N")
	}
	cells := a.UpUp + a.UpDown + a.UpTO + a.DownUp + a.DownDown + a.DownTO + a.AbsUp + a.AbsDown + a.AbsTO
	if cells != a.N {
		return fmt.Errorf("decisionresearch: contingency sum != N")
	}
	return nil
}

func evaluateSpec(ev []EvidenceRow, out []string, begin, end int, spec decision.DecisionSpec, u, l float64) (Audit, error) {
	var a Audit
	if begin < 0 || end > len(ev) || begin > end || len(ev) != len(out) {
		return a, fmt.Errorf("decisionresearch: bad range")
	}
	for i := begin; i < end; i++ {
		intent, err := apply(ev[i].Evidence, spec)
		if err != nil {
			return Audit{}, err
		}
		if err := addCell(&a, intent, out[i]); err != nil {
			return Audit{}, err
		}
	}
	a.TotalUtility = TotalUtility(u, l, a)
	if err := a.check(); err != nil {
		return Audit{}, err
	}
	return a, nil
}

func baselineAudit(out []string, begin, end int) (Audit, error) {
	var a Audit
	if begin < 0 || end > len(out) || begin > end {
		return a, fmt.Errorf("decisionresearch: bad baseline range")
	}
	for i := begin; i < end; i++ {
		if err := addCell(&a, decision.IntentAbstain, out[i]); err != nil {
			return Audit{}, err
		}
	}
	a.TotalUtility = 0
	if err := a.check(); err != nil {
		return Audit{}, err
	}
	return a, nil
}

func selectTrain(ev []EvidenceRow, out []string, fr FoldRange, w World, reverse bool) (Selection, Audit, error) {
	trainN := fr.TrainEnd - fr.TrainBegin
	baseAudit, err := baselineAudit(out, fr.TrainBegin, fr.TrainEnd)
	if err != nil {
		return Selection{}, Audit{}, err
	}
	if baseAudit.N != trainN {
		return Selection{}, Audit{}, fmt.Errorf("decisionresearch: baseline N")
	}
	kind := KindBaseline
	bestU, bestDU, bestDR := 0.0, 0, 0
	var bestSpec decision.DecisionSpec
	bestAudit := baseAudit

	type pair struct{ du, dr int }
	pairs := make([]pair, 0, 81)
	for du := 1; du <= 9; du++ {
		for dr := 1; dr <= 9; dr++ {
			pairs = append(pairs, pair{du, dr})
		}
	}
	if reverse {
		for i, j := 0, len(pairs)-1; i < j; i, j = i+1, j-1 {
			pairs[i], pairs[j] = pairs[j], pairs[i]
		}
	}
	for _, p := range pairs {
		spec, err := materializeSpec(w, p.du, p.dr)
		if err != nil {
			return Selection{}, Audit{}, err
		}
		au, err := evaluateSpec(ev, out, fr.TrainBegin, fr.TrainEnd, spec, w.U, w.L)
		if err != nil {
			return Selection{}, Audit{}, err
		}
		if au.TotalUtility > bestU {
			kind = KindSpec
			bestU, bestDU, bestDR = au.TotalUtility, p.du, p.dr
			bestSpec, bestAudit = spec, au
			continue
		}
		if au.TotalUtility == bestU && kind == KindSpec {
			if p.du > bestDU || (p.du == bestDU && p.dr > bestDR) {
				bestDU, bestDR = p.du, p.dr
				bestSpec, bestAudit = spec, au
			}
		}
	}
	sel := Selection{Kind: kind}
	if kind == KindSpec {
		sel.UtilityDecile, sel.RankDecile, sel.Spec = bestDU, bestDR, bestSpec
	}
	return sel, bestAudit, nil
}

func evaluateSelected(ev []EvidenceRow, out []string, fr FoldRange, w World, sel Selection) (Audit, int, error) {
	before := ApplyCalls
	if sel.Kind == KindBaseline {
		au, err := baselineAudit(out, fr.ValBegin, fr.ValEnd)
		return au, ApplyCalls - before, err
	}
	if sel.Kind != KindSpec {
		return Audit{}, 0, fmt.Errorf("decisionresearch: unknown selection kind")
	}
	au, err := evaluateSpec(ev, out, fr.ValBegin, fr.ValEnd, sel.Spec, w.U, w.L)
	return au, ApplyCalls - before, err
}

func Eligible(pooled float64, positive, foldCount int) bool {
	if !(pooled > 0) {
		return false
	}
	return positive*4 >= foldCount*3
}

func Run(ev []EvidenceRow, out []string, folds []FoldRange, w World) (Result, error) {
	return run(ev, out, folds, w, false)
}

func RunReverseGrid(ev []EvidenceRow, out []string, folds []FoldRange, w World) (Result, error) {
	return run(ev, out, folds, w, true)
}

func run(ev []EvidenceRow, out []string, folds []FoldRange, w World, reverse bool) (Result, error) {
	if w.Logic != decision.DecisionLogicTargetUtilityRankGateV1 {
		return Result{}, fmt.Errorf("decisionresearch: DecisionLogicVersion")
	}
	if w.ClassOrder != classOrder() {
		return Result{}, fmt.Errorf("decisionresearch: class_order")
	}
	if len(ev) != len(out) {
		return Result{}, fmt.Errorf("decisionresearch: evidence/outcome length")
	}
	nCand := 0
	for du := 1; du <= 9; du++ {
		for dr := 1; dr <= 9; dr++ {
			if _, err := materializeSpec(w, du, dr); err != nil {
				return Result{}, err
			}
			nCand++
		}
	}
	if nCand != 81 {
		return Result{}, fmt.Errorf("decisionresearch: candidate_count")
	}
	var res Result
	res.FoldCount = len(folds)
	var pooled Audit
	for i, fr := range folds {
		sel, trainAu, err := selectTrain(ev, out, fr, w, reverse)
		if err != nil {
			return Result{}, err
		}
		valAu, valCalls, err := evaluateSelected(ev, out, fr, w, sel)
		if err != nil {
			return Result{}, err
		}
		valN := fr.ValEnd - fr.ValBegin
		if sel.Kind == KindSpec && valCalls != valN {
			return Result{}, fmt.Errorf("decisionresearch: validation ApplyDecision count")
		}
		if sel.Kind == KindBaseline && valCalls != 0 {
			return Result{}, fmt.Errorf("decisionresearch: baseline validation ApplyDecision count")
		}
		pos := valAu.TotalUtility > 0
		if pos {
			res.PositiveFoldCount++
		}
		res.Folds = append(res.Folds, FoldResult{
			Index: i, Sel: sel, Train: trainAu, Val: valAu, ValidationPositive: pos, ValApplyCalls: valCalls,
		})
		addPooled(&pooled, valAu)
	}
	pooled.TotalUtility = TotalUtility(w.U, w.L, pooled)
	if err := pooled.check(); err != nil {
		return Result{}, err
	}
	res.PooledVal = pooled
	res.EligibleForFinalization = Eligible(pooled.TotalUtility, res.PositiveFoldCount, res.FoldCount)
	return res, nil
}

func addPooled(dst *Audit, src Audit) {
	dst.N += src.N
	dst.UpIntent += src.UpIntent
	dst.DownIntent += src.DownIntent
	dst.Abstain += src.Abstain
	dst.UpUp += src.UpUp
	dst.UpDown += src.UpDown
	dst.UpTO += src.UpTO
	dst.DownUp += src.DownUp
	dst.DownDown += src.DownDown
	dst.DownTO += src.DownTO
	dst.AbsUp += src.AbsUp
	dst.AbsDown += src.AbsDown
	dst.AbsTO += src.AbsTO
}
