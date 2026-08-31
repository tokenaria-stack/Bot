package market

import (
	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

func fractalScanConfigFromSettings(s RSXSettings) indicators.RSXScanConfig {
	lookback := s.DivLookback
	if lookback <= 0 {
		lookback = RSXLookbackDefault
	}
	pivotRadius := s.PivotRadius
	if pivotRadius <= 0 {
		pivotRadius = DefaultRSXPivotRadius
	}
	return indicators.NormalizeRSXScanConfig(indicators.RSXScanConfig{
		Mode:               indicators.RSXScanFractal,
		Lookback:           lookback,
		PivotRadius:        pivotRadius,
		MinPriceDeltaRatio: s.MinPriceDeltaRatio,
		MinOscDelta:        s.MinOscDelta,
	})
}

func fractalPricesFromKlines(klines []exchange.Kline, source string) []float64 {
	out := make([]float64, len(klines))
	for i, k := range klines {
		out[i] = RSXSourcePrice(k.High, k.Low, k.Close, source)
	}
	return out
}

func fractalOpensFromKlines(klines []exchange.Kline) []int64 {
	out := make([]int64, len(klines))
	for i, k := range klines {
		out[i] = k.OpenTime
	}
	return out
}

// RSTFractalFactsFromClosedSeries emits fractal RSX divergence and pivot facts.
func RSTFractalFactsFromClosedSeries(klines []exchange.Kline, rsx []float64, settings RSXSettings) []indicators.IndicatorFactEvent {
	n := len(klines)
	if n == 0 || len(rsx) != n {
		return nil
	}
	prices := fractalPricesFromKlines(klines, settings.Source)
	opens := fractalOpensFromKlines(klines)
	return indicators.FractalFacts(prices, rsx, opens, fractalScanConfigFromSettings(settings))
}

func RSTFractalFactsFromDAGHistory(klines []exchange.Kline, hist *core.HistoryBus, settings RSXSettings) []indicators.IndicatorFactEvent {
	rsx := historySlotSeries(hist, core.SlotJurikRSX)
	n := len(rsx)
	if n > len(klines) {
		n = len(klines)
	}
	if n < 5 {
		return nil
	}
	return RSTFractalFactsFromClosedSeries(klines[:n], rsx[:n], settings)
}

func (a *Frame) noteRSTFractalFactLocked(isClosed bool, barIndex int) {
	if a == nil || !isClosed || barIndex < 0 || barIndex >= len(a.klines) {
		return
	}
	if a.rsxApplied&nodes.NeedRSFractal == 0 {
		return
	}
	a.fractalEvals++
	bus := a.dag.Bus()
	if bus == nil || bus.Cur == nil {
		return
	}
	k := a.klines[barIndex]
	osc := bus.Cur.Get(core.SlotJurikRSX)
	price := RSXSourcePrice(k.High, k.Low, k.Close, a.effectiveRSXSettings().Source)
	if len(a.rsxFractalOpens) == barIndex+1 && a.rsxFractalOpens[barIndex] == k.OpenTime {
		a.rsxFractalPrices[barIndex] = price
		a.rsxFractalOsc[barIndex] = osc
	} else if len(a.rsxFractalOpens) == barIndex {
		a.rsxFractalPrices = append(a.rsxFractalPrices, price)
		a.rsxFractalOsc = append(a.rsxFractalOsc, osc)
		a.rsxFractalOpens = append(a.rsxFractalOpens, k.OpenTime)
	} else {
		hist := a.dagHistoryLocked()
		if hist != nil && hist.Count() == len(a.klines) {
			a.resyncRSTFractalSeriesLocked(hist, barIndex+1)
		} else {
			return
		}
	}
	if len(a.rsxFractalOpens) < 5 || barIndex != len(a.rsxFractalOpens)-1 {
		return
	}
	cfg := fractalScanConfigFromSettings(a.effectiveRSXSettings())
	events := indicators.FractalFactsAt(a.rsxFractalPrices, a.rsxFractalOsc, a.rsxFractalOpens, cfg, barIndex)
	confirmed := a.rsxFractalOpens[barIndex]
	kept := a.rsxFractalFacts[:0]
	for _, ev := range a.rsxFractalFacts {
		if ev.ConfirmedAt != confirmed {
			kept = append(kept, ev)
		}
	}
	a.rsxFractalFacts = append(kept, events...)
}

func (a *Frame) resyncRSTFractalSeriesLocked(hist *core.HistoryBus, n int) {
	rsx := historySlotSeries(hist, core.SlotJurikRSX)
	if len(rsx) < n {
		n = len(rsx)
	}
	if n > len(a.klines) {
		n = len(a.klines)
	}
	a.rsxFractalPrices = make([]float64, n)
	a.rsxFractalOsc = make([]float64, n)
	a.rsxFractalOpens = make([]int64, n)
	src := a.effectiveRSXSettings().Source
	for i := 0; i < n; i++ {
		k := a.klines[i]
		a.rsxFractalPrices[i] = RSXSourcePrice(k.High, k.Low, k.Close, src)
		a.rsxFractalOsc[i] = rsx[i]
		a.rsxFractalOpens[i] = k.OpenTime
	}
}

func (a *Frame) rebuildRSTFractalFactsLocked() {
	a.rsxFractalFacts = nil
	a.rsxFractalPrices = nil
	a.rsxFractalOsc = nil
	a.rsxFractalOpens = nil
	hist := a.dagHistoryLocked()
	if hist == nil {
		return
	}
	n := hist.Count()
	if n > len(a.klines) {
		n = len(a.klines)
	}
	if n < 5 {
		return
	}
	a.resyncRSTFractalSeriesLocked(hist, n)
	a.rsxFractalFacts = indicators.FractalFacts(
		a.rsxFractalPrices, a.rsxFractalOsc, a.rsxFractalOpens,
		fractalScanConfigFromSettings(a.effectiveRSXSettings()),
	)
}

func (a *Frame) trimRSTFractalFactsLocked() {
	if len(a.klines) == 0 {
		a.rsxFractalFacts = nil
		a.rsxFractalPrices = nil
		a.rsxFractalOsc = nil
		a.rsxFractalOpens = nil
		return
	}
	minOpen := a.klines[0].OpenTime
	drop := 0
	for drop < len(a.rsxFractalOpens) && a.rsxFractalOpens[drop] < minOpen {
		drop++
	}
	if drop > 0 {
		a.rsxFractalPrices = a.rsxFractalPrices[drop:]
		a.rsxFractalOsc = a.rsxFractalOsc[drop:]
		a.rsxFractalOpens = a.rsxFractalOpens[drop:]
	}
	if len(a.rsxFractalFacts) == 0 {
		return
	}
	out := a.rsxFractalFacts[:0]
	for _, ev := range a.rsxFractalFacts {
		if ev.AnchorAt >= minOpen && ev.ConfirmedAt >= minOpen {
			out = append(out, ev)
		}
	}
	a.rsxFractalFacts = out
}

// RSTFractalFactsSnapshot copies published fractal RSX facts.
func (a *Frame) RSTFractalFactsSnapshot() []indicators.IndicatorFactEvent {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.rsxFractalFacts) == 0 {
		return nil
	}
	return append([]indicators.IndicatorFactEvent(nil), a.rsxFractalFacts...)
}

func (a *Frame) RSTFractalFactsConfirmedAt(openTimeMs int64) []indicators.IndicatorFactEvent {
	if a == nil || openTimeMs <= 0 {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []indicators.IndicatorFactEvent
	for _, ev := range a.rsxFractalFacts {
		if ev.ConfirmedAt == openTimeMs {
			out = append(out, ev)
		}
	}
	return out
}
