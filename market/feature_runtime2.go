package market

import (
	"fmt"
	"math"

	"trading_bot/data"
	"trading_bot/forecast"
	"trading_bot/indicators"
)

type htfContext struct {
	at        int64
	closeTime int64
	ready     bool
	values    [11]float64
}

type patternPair struct {
	pred timedFact
	done timedFact
}

// FeatureRuntime2 is the Brain V2 closed-bar sensory state machine.
type FeatureRuntime2 struct {
	spec forecast.FeatureSpec2
	q    int
	h    int

	primary *nativeRSXContext
	htf1h   *nativeRSXContext
	htf4h   *nativeRSXContext

	atr       *indicators.ATR
	closes    []float64
	highs     []float64
	lows      []float64
	atrs      []float64
	rsxHist   []float64
	cross50Up timedFact
	cross50Dn timedFact
	prevRSX50 float64
	have50    bool

	tvToCross  [2]patternPair // 0 bull, 1 bear
	crossToTV  [2]patternPair
	pivotCross [2]patternPair
	sameBar    [2]timedFact

	ctx1h htfContext
	ctx4h htfContext
}

func NewFeatureRuntime2(spec forecast.FeatureSpec2) (*FeatureRuntime2, error) {
	resolved, err := forecast.ResolveFeatureSpec2(spec.Primary, spec.Target, spec.Analysis)
	if err != nil {
		return nil, err
	}
	want, err := resolved.Identity()
	if err != nil {
		return nil, err
	}
	got, err := spec.Identity()
	if err != nil {
		return nil, err
	}
	if want.Digest != got.Digest {
		return nil, fmt.Errorf("market: feature-runtime-2 FeatureSpec2 identity mismatch")
	}
	if !spec.Demand.IIRFromSourceStart {
		return nil, fmt.Errorf("market: feature-runtime-2 requires IIRFromSourceStart")
	}
	cfg := spec.Analysis.Config
	p, err := newNativeRSXContext(spec.Primary.Timeframe, cfg.RSXSource, cfg.RSXLength, cfg.RSXSignal, cfg.DivLookback,
		forecast.Spec2TVPivotAgePrimary, forecast.Spec2SimpleCrossAgePrim, true)
	if err != nil {
		return nil, err
	}
	h1, err := newNativeRSXContext(spec.HTF1h.Timeframe, cfg.RSXSource, cfg.RSXLength, cfg.RSXSignal, cfg.DivLookback,
		forecast.Spec2HTFFactAgeNative, forecast.Spec2HTFFactAgeNative, false)
	if err != nil {
		return nil, err
	}
	h4, err := newNativeRSXContext(spec.HTF4h.Timeframe, cfg.RSXSource, cfg.RSXLength, cfg.RSXSignal, cfg.DivLookback,
		forecast.Spec2HTFFactAgeNative, forecast.Spec2HTFFactAgeNative, false)
	if err != nil {
		return nil, err
	}
	atr, err := indicators.NewATRFromSpec(spec.Target.ATR)
	if err != nil {
		return nil, err
	}
	return &FeatureRuntime2{
		spec: spec, q: spec.Q, h: spec.Target.HorizonBars,
		primary: p, htf1h: h1, htf4h: h4, atr: atr,
	}, nil
}

func (r *FeatureRuntime2) Update4h(openTime int64, high, low, close float64) error {
	return r.updateHTF(r.htf4h, &r.ctx4h, r.spec.HTF4h.Timeframe, openTime, high, low, close)
}

func (r *FeatureRuntime2) Update1h(openTime int64, high, low, close float64) error {
	return r.updateHTF(r.htf1h, &r.ctx1h, r.spec.HTF1h.Timeframe, openTime, high, low, close)
}

func (r *FeatureRuntime2) updateHTF(ctx *nativeRSXContext, cache *htfContext, tf string, openTime int64, high, low, close float64) error {
	ct, err := data.BarCloseTimeMs(openTime, tf)
	if err != nil {
		return err
	}
	if err := ctx.update(openTime, ct, high, low, close); err != nil {
		return err
	}
	vals, ready := ctx.block11()
	*cache = htfContext{at: openTime, closeTime: ct, ready: ready, values: vals}
	return nil
}

// Update15m advances primary state and emits one FeatureRow2. Caller must have
// already applied all HTF bars whose CloseTime == this primary CloseTime.
func (r *FeatureRuntime2) Update15m(openTime int64, high, low, close float64) (forecast.FeatureRow2, error) {
	ct, err := data.BarCloseTimeMs(openTime, r.spec.Primary.Timeframe)
	if err != nil {
		return forecast.FeatureRow2{}, err
	}
	if r.ctx1h.at != 0 && r.ctx1h.closeTime > ct {
		return forecast.FeatureRow2{}, fmt.Errorf("market: feature-runtime-2 1h CloseTime after primary")
	}
	if r.ctx4h.at != 0 && r.ctx4h.closeTime > ct {
		return forecast.FeatureRow2{}, fmt.Errorf("market: feature-runtime-2 4h CloseTime after primary")
	}
	if err := r.primary.update(openTime, ct, high, low, close); err != nil {
		return forecast.FeatureRow2{}, err
	}
	atrV, err := r.atr.UpdateClosed(high, low, close)
	if err != nil {
		return forecast.FeatureRow2{}, err
	}
	r.closes = append(r.closes, close)
	r.highs = append(r.highs, high)
	r.lows = append(r.lows, low)
	r.atrs = append(r.atrs, atrV)
	r.rsxHist = append(r.rsxHist, r.primary.rsx)
	if r.have50 {
		if forecast.Midline50CrossUp(r.prevRSX50, r.primary.rsx) {
			r.cross50Up = timedFact{at: openTime, ord: r.primary.ord}
		}
		if forecast.Midline50CrossDown(r.prevRSX50, r.primary.rsx) {
			r.cross50Dn = timedFact{at: openTime, ord: r.primary.ord}
		}
	}
	r.prevRSX50, r.have50 = r.primary.rsx, true
	r.advancePatterns()
	return r.emit(openTime, ct)
}

func (r *FeatureRuntime2) advancePatterns() {
	p := r.primary
	ord, at := p.ord, p.lastAt
	complete := func(pair *patternPair, pred timedFact, secondNow bool) {
		if secondNow && pred.at != 0 && forecast.OrderedPatternOK(pred.at, at, pred.ord, ord) {
			pair.done = timedFact{at: at, ord: ord}
		}
	}
	if p.tvBullNow {
		r.tvToCross[0].pred = p.tvBull
		complete(&r.crossToTV[0], r.crossToTV[0].pred, true)
	}
	if p.tvBearNow {
		r.tvToCross[1].pred = p.tvBear
		complete(&r.crossToTV[1], r.crossToTV[1].pred, true)
	}
	if p.crossUpNow {
		r.crossToTV[0].pred = p.crossUp
		complete(&r.tvToCross[0], r.tvToCross[0].pred, true)
		complete(&r.pivotCross[0], r.pivotCross[0].pred, true)
	}
	if p.crossDnNow {
		r.crossToTV[1].pred = p.crossDown
		complete(&r.tvToCross[1], r.tvToCross[1].pred, true)
		complete(&r.pivotCross[1], r.pivotCross[1].pred, true)
	}
	if p.pivLoNow {
		r.pivotCross[0].pred = p.pivLow
	}
	if p.pivHiNow {
		r.pivotCross[1].pred = p.pivHigh
	}
	if p.tvBullNow && p.crossUpNow && forecast.SameBarConjunction(p.tvBull.at, p.crossUp.at) {
		r.sameBar[0] = timedFact{at: at, ord: ord}
	}
	if p.tvBearNow && p.crossDnNow && forecast.SameBarConjunction(p.tvBear.at, p.crossDown.at) {
		r.sameBar[1] = timedFact{at: at, ord: ord}
	}
}

func (r *FeatureRuntime2) emit(openTime, closeTime int64) (forecast.FeatureRow2, error) {
	row := forecast.FeatureRow2{At: openTime}
	if r.ctx1h.at == 0 || !r.ctx1h.ready {
		row.Reason = forecast.ReasonHTF1hWarmup
		return row, nil
	}
	if r.ctx1h.closeTime > closeTime {
		return row, fmt.Errorf("market: feature-runtime-2 illegal future 1h")
	}
	if r.ctx4h.at == 0 || !r.ctx4h.ready {
		row.Reason = forecast.ReasonHTF4hWarmup
		return row, nil
	}
	if r.ctx4h.closeTime > closeTime {
		return row, fmt.Errorf("market: feature-runtime-2 illegal future 4h")
	}
	p := r.primary
	t := len(r.closes) - 1
	if !p.ready || t < r.h || t < r.q || !r.atr.Ready() {
		row.Reason = forecast.ReasonPrimaryWarmup
		return row, nil
	}
	deltaQ := p.rsx
	if t >= r.q {
		deltaQ = p.rsx - r.rsxHist[t-r.q]
	} else {
		row.Reason = forecast.ReasonPrimaryWarmup
		return row, nil
	}
	dispQ, rd := forecast.PriceDisplacementATR(r.closes[t], r.closes[t-r.q], r.atrs[t])
	if rd != forecast.IsReady {
		row.Reason = forecast.ReasonPriceNotReady
		return row, nil
	}
	dispH, rd := forecast.PriceDisplacementATR(r.closes[t], r.closes[t-r.h], r.atrs[t])
	if rd != forecast.IsReady {
		row.Reason = forecast.ReasonPriceNotReady
		return row, nil
	}
	rng, rd := forecast.PriceRangePositionH(r.highs, r.lows, r.closes, t, r.h)
	if rd != forecast.IsReady {
		row.Reason = forecast.ReasonPriceNotReady
		return row, nil
	}
	path, rd := forecast.PricePathEfficiencyH(r.closes, t, r.h)
	if rd != forecast.IsReady {
		row.Reason = forecast.ReasonPriceNotReady
		return row, nil
	}
	atrP, rd := forecast.ATROverPrice(r.atrs[t], r.closes[t])
	if rd != forecast.IsReady {
		row.Reason = forecast.ReasonPriceNotReady
		return row, nil
	}
	atrQ, rd := forecast.ATRChangeQ(r.atrs[t], r.atrs[t-r.q])
	if rd != forecast.IsReady {
		row.Reason = forecast.ReasonPriceNotReady
		return row, nil
	}
	if math.IsNaN(deltaQ) || math.IsInf(deltaQ, 0) {
		row.Reason = forecast.ReasonPrimaryWarmup
		return row, nil
	}
	tvBP, tvBA := p.tvBull.presentAge(p.ord, p.tvMax)
	tvRP, tvRA := p.tvBear.presentAge(p.ord, p.tvMax)
	phP, phA := p.pivHigh.presentAge(p.ord, p.tvMax)
	plP, plA := p.pivLow.presentAge(p.ord, p.tvMax)
	cuP, cuA := p.crossUp.presentAge(p.ord, p.crossMax)
	cdP, cdA := p.crossDown.presentAge(p.ord, p.crossMax)
	f50uP, f50uA := r.cross50Up.presentAge(p.ord, forecast.Spec2SimpleCrossAgePrim)
	f50dP, f50dA := r.cross50Dn.presentAge(p.ord, forecast.Spec2SimpleCrossAgePrim)
	pat := func(done timedFact) (float64, float64) {
		return done.presentAge(p.ord, forecast.Spec2PatternActiveAge)
	}
	p1b, a1b := pat(r.tvToCross[0].done)
	p1s, a1s := pat(r.tvToCross[1].done)
	p2b, a2b := pat(r.crossToTV[0].done)
	p2s, a2s := pat(r.crossToTV[1].done)
	p3b, a3b := pat(r.pivotCross[0].done)
	p3s, a3s := pat(r.pivotCross[1].done)
	p4b, a4b := pat(r.sameBar[0])
	p4s, a4s := pat(r.sameBar[1])

	var v forecast.FeatureVector2
	n := 0
	put := func(xs ...float64) {
		for _, x := range xs {
			v[n] = x
			n++
		}
	}
	put(p.rsx, p.minus, p.delta1, deltaQ)
	put(tvBP, tvBA, tvRP, tvRA)
	put(phP, phA, plP, plA)
	put(cuP, cuA, cdP, cdA)
	put(f50uP, f50uA, f50dP, f50dA)
	put(dispQ, dispH, rng, path, atrP, atrQ)
	put(p1b, a1b, p1s, a1s)
	put(p2b, a2b, p2s, a2s)
	put(p3b, a3b, p3s, a3s)
	put(p4b, a4b, p4s, a4s)
	put(r.ctx1h.values[:]...)
	put(r.ctx4h.values[:]...)
	if n != forecast.Spec2FeatureWidth {
		return forecast.FeatureRow2{}, fmt.Errorf("market: feature-runtime-2 encoder offset %d != 64", n)
	}
	if err := forecast.Vector2Finite(v); err != nil {
		return forecast.FeatureRow2{}, err
	}
	row.Ready = forecast.IsReady
	row.Values = v
	return row, nil
}
