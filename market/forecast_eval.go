package market

import (
	"fmt"
	"math"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/exchange"
	"trading_bot/forecast"
	"trading_bot/indicators"
)

const analysisLogicV2 forecast.LogicVersion = "analysis:v2"

// AnalysisRecipeFromRSXSettings is the single conversion from the Frame's
// authoritative effective RSX settings into a resolved AnalysisRecipe.
// enableTV / enableFractal are bind-time capabilities (Frame has no such flags).
func AnalysisRecipeFromRSXSettings(s RSXSettings, enableTV, enableFractal bool, logic forecast.LogicVersion) (forecast.AnalysisRecipe, error) {
	n := NormalizeRSXSettings(s)
	return forecast.ResolveAnalysisRecipe("from-frame", forecast.AnalysisRecipeDraft{
		RSXLength:     n.Length,
		RSXSignal:     n.SignalLength,
		RSXSource:     n.Source,
		DivLookback:   n.DivLookback,
		EnableTV:      enableTV,
		EnableFractal: enableFractal,
	}, logic)
}

// RSXWorkFromFeaturePlan maps a compiled schema onto the existing RSXWorkMask.
func RSXWorkFromFeaturePlan(plan forecast.FeaturePlan) nodes.RSXWorkMask {
	var m nodes.RSXWorkMask
	for _, id := range plan.Schema {
		switch id {
		case forecast.FeatureRSXValue, forecast.FeatureRSXSignal, forecast.FeatureRSXSlope:
			m |= nodes.NeedRSXCore
		case forecast.FeatureTVBullPresent, forecast.FeatureTVBullAge:
			m |= nodes.NeedRSXCore | nodes.NeedRSTV
		case forecast.FeatureFractalClassA:
			m |= nodes.NeedRSXCore | nodes.NeedRSFractal
		}
	}
	return m
}

// FeatureEvaluator is the host binding: FeaturePlan schema + Frame truth/facts.
// Frame stays ignorant of FeatureID.
type FeatureEvaluator struct {
	frame  *Frame
	plan   forecast.FeaturePlan
	schema []forecast.FeatureID
	maxAge int
	dst    []float64
}

// BindFeatureEvaluator verifies AnalysisRecipe identity against this Frame's
// effective settings, compiles the schema, and installs forecast demand as a
// persistent contribution.
func BindFeatureEvaluator(frame *Frame, plan forecast.FeaturePlan) (*FeatureEvaluator, error) {
	if frame == nil {
		return nil, fmt.Errorf("market: feature evaluator requires a Frame")
	}
	if err := validateTape1ASchema(plan.Schema); err != nil {
		return nil, err
	}
	logic := analysisLogicV2
	if len(plan.Analysis.Logic) > 0 && plan.Analysis.Logic[0] != "" {
		logic = plan.Analysis.Logic[0]
	}
	needTV, needFractal := schemaCaps(plan.Schema)
	settings := frame.EffectiveRSXSettings()
	actual, err := AnalysisRecipeFromRSXSettings(settings, needTV, needFractal, logic)
	if err != nil {
		return nil, err
	}
	actualID, err := actual.Identity()
	if err != nil {
		return nil, err
	}
	if actualID.Digest != plan.Analysis.Digest {
		return nil, fmt.Errorf("market: refuse feature bind: AnalysisRecipe identity mismatch (frame %s vs plan %s)", actualID.Digest.Short(), plan.Analysis.Digest.Short())
	}
	maxAge := plan.FeatureHistoryBars
	if maxAge < 0 {
		maxAge = 0
	}
	ev := &FeatureEvaluator{
		frame:  frame,
		plan:   plan,
		schema: append([]forecast.FeatureID(nil), plan.Schema...),
		maxAge: maxAge,
		dst:    make([]float64, plan.VectorLen()),
	}
	frame.SetRSXForecastDemand(RSXWorkFromFeaturePlan(plan))
	return ev, nil
}

func (e *FeatureEvaluator) Unbind() {
	if e == nil || e.frame == nil {
		return
	}
	e.frame.SetRSXForecastDemand(0)
}

func (e *FeatureEvaluator) Plan() forecast.FeaturePlan {
	if e == nil {
		return forecast.FeaturePlan{}
	}
	return e.plan
}

func (e *FeatureEvaluator) VectorLen() int {
	if e == nil {
		return 0
	}
	return len(e.schema)
}

// LastBarOpenTime is the Frame tip OpenTime after the last closed ingest.
// Dump uses this to refuse At off-by-one; it is not a feature calculation.
func (e *FeatureEvaluator) LastBarOpenTime() int64 {
	if e == nil || e.frame == nil {
		return 0
	}
	e.frame.mu.RLock()
	defer e.frame.mu.RUnlock()
	if len(e.frame.klines) == 0 {
		return 0
	}
	return e.frame.klines[len(e.frame.klines)-1].OpenTime
}

// FillOwned overwrites the evaluator-owned vector. Steady-state Fill must not allocate.
func (e *FeatureEvaluator) FillOwned() (forecast.Ready, []float64, error) {
	if e == nil {
		return forecast.NotReady, nil, fmt.Errorf("market: nil feature evaluator")
	}
	ready, err := e.Fill(e.dst)
	return ready, e.dst, err
}

// Fill writes the compiled schema into dst. dst must already have VectorLen.
func (e *FeatureEvaluator) Fill(dst []float64) (forecast.Ready, error) {
	if e == nil || e.frame == nil {
		return forecast.NotReady, fmt.Errorf("market: nil feature evaluator")
	}
	if len(dst) != len(e.schema) {
		return forecast.NotReady, fmt.Errorf("market: feature dst len %d != VectorLen %d", len(dst), len(e.schema))
	}
	f := e.frame
	f.mu.RLock()
	defer f.mu.RUnlock()
	if len(f.klines) == 0 {
		return forecast.NotReady, nil
	}
	if f.dag == nil || f.dag.Bus() == nil || f.dag.Bus().Cur == nil {
		return forecast.NotReady, nil
	}
	if f.rsxApplied&nodes.NeedRSXCore == 0 {
		return forecast.NotReady, nil
	}
	cur := f.dag.Bus().Cur
	rsx := cur.Get(core.SlotJurikRSX)
	sig := cur.Get(core.SlotJurikSignal)
	if math.IsNaN(rsx) || math.IsInf(rsx, 0) || math.IsNaN(sig) || math.IsInf(sig, 0) {
		return forecast.NotReady, nil
	}
	needTV := false
	for _, id := range e.schema {
		if id == forecast.FeatureTVBullPresent || id == forecast.FeatureTVBullAge {
			needTV = true
			break
		}
	}
	if needTV && f.rsxApplied&nodes.NeedRSTV == 0 {
		return forecast.NotReady, nil
	}
	at := f.klines[len(f.klines)-1].OpenTime
	present, age := tvBullPresentAgeLocked(f, at, e.maxAge)
	for i, id := range e.schema {
		switch id {
		case forecast.FeatureRSXValue:
			dst[i] = rsx
		case forecast.FeatureRSXSignal:
			dst[i] = sig
		case forecast.FeatureTVBullPresent:
			if present {
				dst[i] = 1
			} else {
				dst[i] = 0
			}
		case forecast.FeatureTVBullAge:
			if present {
				dst[i] = float64(age)
			} else {
				dst[i] = 0
			}
		default:
			return forecast.NotReady, fmt.Errorf("market: unexpected feature %q in compiled schema", id)
		}
		if math.IsNaN(dst[i]) || math.IsInf(dst[i], 0) {
			return forecast.NotReady, nil
		}
	}
	return forecast.IsReady, nil
}

func validateTape1ASchema(schema []forecast.FeatureID) error {
	if len(schema) == 0 {
		return fmt.Errorf("market: feature plan schema is empty")
	}
	for _, id := range schema {
		switch id {
		case forecast.FeatureRSXValue, forecast.FeatureRSXSignal, forecast.FeatureTVBullPresent, forecast.FeatureTVBullAge:
		default:
			return fmt.Errorf("market: FEATURE-TAPE-1A refuses feature %q", id)
		}
	}
	return nil
}

func schemaCaps(schema []forecast.FeatureID) (tv, fractal bool) {
	for _, id := range schema {
		switch id {
		case forecast.FeatureTVBullPresent, forecast.FeatureTVBullAge:
			tv = true
		case forecast.FeatureFractalClassA:
			fractal = true
		}
	}
	return tv, fractal
}

func tvBullPresentAgeLocked(a *Frame, candidateAt int64, maxAge int) (present bool, age int) {
	if a == nil || candidateAt <= 0 || len(a.klines) == 0 {
		return false, 0
	}
	last := len(a.klines) - 1
	if a.klines[last].OpenTime != candidateAt {
		return false, 0
	}
	horizonIdx := 0
	if maxAge >= 0 && last > maxAge {
		horizonIdx = last - maxAge
	}
	horizonOpen := a.klines[horizonIdx].OpenTime
	facts := a.rsxTVFacts
	for i := len(facts) - 1; i >= 0; i-- {
		ev := facts[i]
		if ev.ConfirmedAt < horizonOpen {
			break
		}
		if ev.Source != indicators.FactSourceRSXTVDiv || ev.Direction != indicators.FactDirBullish {
			continue
		}
		if ev.ConfirmedAt > candidateAt {
			continue
		}
		bars, ok := closedBarAgeLocked(a.klines, last, ev.ConfirmedAt, maxAge)
		if !ok {
			continue
		}
		return true, bars
	}
	return false, 0
}

func closedBarAgeLocked(klines []exchange.Kline, lastIdx int, confirmedAt int64, maxAge int) (int, bool) {
	limit := 0
	if maxAge >= 0 && lastIdx-maxAge > 0 {
		limit = lastIdx - maxAge
	}
	for j := lastIdx; j >= limit; j-- {
		if klines[j].OpenTime == confirmedAt {
			return lastIdx - j, true
		}
	}
	return 0, false
}
