package market

import (
	"fmt"
	"math"

	"trading_bot/forecast"
	"trading_bot/indicators"
)

type timedFact struct {
	at  int64
	ord int
}

func (f timedFact) presentAge(nowOrd, maxAge int) (present, age float64) {
	if f.at == 0 {
		return 0, 0
	}
	return forecast.NativePresentAge(nowOrd, f.ord, maxAge)
}

// nativeRSXContext is one Tape-2 RSX/TV/signal-cross owner wrapping canonical Jurik+RSTV.
type nativeRSXContext struct {
	tf         string
	source     string
	jurik      *indicators.JurikRSX
	signal     *indicators.RSXSignalLine
	rstv       *indicators.RSTVState
	signalNeed int
	signalN    int
	ord        int
	havePrev   bool
	prevRSX    float64
	prevMinus  float64
	rsx        float64
	sig        float64
	minus      float64
	delta1     float64
	tvBull     timedFact
	tvBear     timedFact
	crossUp    timedFact
	crossDown  timedFact
	tvMax      int
	crossMax   int
	lastAt     int64
	closeTime  int64
	ready      bool
	crossUpNow bool
	crossDnNow bool
	tvBullNow  bool
	tvBearNow  bool
	pivHiNow   bool
	pivLoNow   bool
	pivHigh    timedFact
	pivLow     timedFact
	trackPivot bool
}

func newNativeRSXContext(tf, source string, rsxLen, sigLen, tvLookback, tvMax, crossMax int, trackPivot bool) (*nativeRSXContext, error) {
	if source != forecast.Spec2RSXSource {
		return nil, fmt.Errorf("market: feature-runtime-2 RSX source must be hlc3")
	}
	return &nativeRSXContext{
		tf: tf, source: source,
		jurik:      indicators.NewJurikRSX(rsxLen),
		signal:     indicators.NewRSXSignalLine(sigLen),
		rstv:       indicators.NewRSTVState(tvLookback),
		signalNeed: sigLen, tvMax: tvMax, crossMax: crossMax, trackPivot: trackPivot,
		ord: -1,
	}, nil
}

func hlc3(high, low, close float64) float64 { return (high + low + close) / 3 }

func (c *nativeRSXContext) update(openTime int64, closeTime int64, high, low, close float64) error {
	if c.lastAt != 0 && openTime <= c.lastAt {
		return fmt.Errorf("market: feature-runtime-2 %s bar not forward", c.tf)
	}
	px := hlc3(high, low, close)
	if math.IsNaN(px) || math.IsInf(px, 0) {
		return fmt.Errorf("market: feature-runtime-2 %s hlc3 not finite", c.tf)
	}
	rsx := c.jurik.Update(px)
	sig := c.signal.Update(rsx)
	c.signalN++
	minus := rsx - sig
	c.crossUpNow, c.crossDnNow = false, false
	c.tvBullNow, c.tvBearNow, c.pivHiNow, c.pivLoNow = false, false, false, false
	if c.havePrev {
		c.delta1 = rsx - c.prevRSX
		if forecast.SignalCrossUp(c.prevMinus, minus) {
			c.crossUpNow = true
			c.crossUp = timedFact{at: openTime, ord: c.ord + 1}
		}
		if forecast.SignalCrossDown(c.prevMinus, minus) {
			c.crossDnNow = true
			c.crossDown = timedFact{at: openTime, ord: c.ord + 1}
		}
	}
	evs, err := c.rstv.UpdateClosed(openTime, close, rsx)
	if err != nil {
		return err
	}
	c.ord++
	for i := 0; i < int(evs.Count); i++ {
		ev := evs.Events[i]
		fact := timedFact{at: ev.ConfirmedAt, ord: c.ord}
		switch {
		case ev.Family == indicators.RSTVFamilyDiv && ev.Direction == indicators.FactDirBullish:
			c.tvBull, c.tvBullNow = fact, true
		case ev.Family == indicators.RSTVFamilyDiv && ev.Direction == indicators.FactDirBearish:
			c.tvBear, c.tvBearNow = fact, true
		case c.trackPivot && ev.Family == indicators.RSTVFamilyPivot && ev.Direction == indicators.FactDirPivotHigh:
			c.pivHigh, c.pivHiNow = fact, true
		case c.trackPivot && ev.Family == indicators.RSTVFamilyPivot && ev.Direction == indicators.FactDirPivotLow:
			c.pivLow, c.pivLoNow = fact, true
		}
	}
	c.rsx, c.sig, c.minus = rsx, sig, minus
	c.prevRSX, c.prevMinus, c.havePrev = rsx, minus, true
	c.lastAt, c.closeTime = openTime, closeTime
	c.ready = c.ord >= 1 && c.signalN >= c.signalNeed &&
		!math.IsNaN(rsx) && !math.IsInf(rsx, 0) &&
		!math.IsNaN(sig) && !math.IsInf(sig, 0) &&
		!math.IsNaN(c.delta1) && !math.IsInf(c.delta1, 0)
	return nil
}

func (c *nativeRSXContext) block11() (vals [11]float64, ready bool) {
	if !c.ready {
		return vals, false
	}
	tbP, tbA := c.tvBull.presentAge(c.ord, c.tvMax)
	trP, trA := c.tvBear.presentAge(c.ord, c.tvMax)
	cuP, cuA := c.crossUp.presentAge(c.ord, c.crossMax)
	cdP, cdA := c.crossDown.presentAge(c.ord, c.crossMax)
	vals = [11]float64{
		c.rsx, c.minus, c.delta1,
		tbP, tbA, trP, trA,
		cuP, cuA, cdP, cdA,
	}
	return vals, true
}
