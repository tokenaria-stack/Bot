package market

import (
	"trading_bot/core"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

// RSTVFactsFromClosedSeries emits Pine TV divergence and TV pivot facts from aligned closed bars.
// rsx[i] must be SlotJurikRSX for klines[i]. Lookback follows RSX settings.
func RSTVFactsFromClosedSeries(klines []exchange.Kline, rsx []float64, lookback int) []indicators.IndicatorFactEvent {
	n := len(klines)
	if n == 0 || len(rsx) != n {
		return nil
	}
	closes := make([]float64, n)
	opens := make([]int64, n)
	for i, k := range klines {
		closes[i] = k.Close
		opens[i] = k.OpenTime
	}
	return indicators.MergeTVFacts(
		indicators.TVDivergenceFacts(closes, rsx, opens, lookback),
		indicators.TVPivotFacts(closes, rsx, opens, lookback),
	)
}

func RSTVFactsFromDAGHistory(klines []exchange.Kline, hist *core.HistoryBus, settings RSXSettings) []indicators.IndicatorFactEvent {
	rsx := historySlotSeries(hist, core.SlotJurikRSX)
	n := len(rsx)
	if n > len(klines) {
		n = len(klines)
	}
	if n < 3 {
		return nil
	}
	return RSTVFactsFromClosedSeries(klines[:n], rsx[:n], settings.DivLookback)
}

func historySlotSeries(hist *core.HistoryBus, slot core.Slot) []float64 {
	if hist == nil {
		return nil
	}
	n := hist.Count()
	if n <= 0 {
		return nil
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = hist.Get(slot, n-i)
	}
	return out
}

func (a *Frame) noteRSTVFactLocked(isClosed bool, barIndex int) {
	if a == nil || !isClosed || barIndex < 0 || barIndex >= len(a.klines) {
		return
	}
	bus := a.dag.Bus()
	if bus == nil || bus.Cur == nil {
		return
	}
	k := a.klines[barIndex]
	osc := bus.Cur.Get(core.SlotJurikRSX)
	if len(a.rsxTVOpens) == barIndex+1 && a.rsxTVOpens[barIndex] == k.OpenTime {
		a.rsxTVCloses[barIndex] = k.Close
		a.rsxTVOsc[barIndex] = osc
	} else if len(a.rsxTVOpens) == barIndex {
		a.rsxTVCloses = append(a.rsxTVCloses, k.Close)
		a.rsxTVOsc = append(a.rsxTVOsc, osc)
		a.rsxTVOpens = append(a.rsxTVOpens, k.OpenTime)
	} else {
		hist := a.dagHistoryLocked()
		if hist != nil && hist.Count() == len(a.klines) {
			a.resyncRSTVSeriesLocked(hist, barIndex+1)
		} else {
			return
		}
	}
	if len(a.rsxTVOpens) < 3 || barIndex != len(a.rsxTVOpens)-1 {
		return
	}
	lookback := a.effectiveRSXSettings().DivLookback
	events := indicators.TVFactsAt(a.rsxTVCloses, a.rsxTVOsc, a.rsxTVOpens, barIndex, lookback)
	confirmed := a.rsxTVOpens[barIndex]
	kept := a.rsxTVFacts[:0]
	for _, ev := range a.rsxTVFacts {
		if ev.ConfirmedAt != confirmed {
			kept = append(kept, ev)
		}
	}
	a.rsxTVFacts = append(kept, events...)
}

func (a *Frame) resyncRSTVSeriesLocked(hist *core.HistoryBus, n int) {
	rsx := historySlotSeries(hist, core.SlotJurikRSX)
	if len(rsx) < n {
		n = len(rsx)
	}
	if n > len(a.klines) {
		n = len(a.klines)
	}
	a.rsxTVCloses = make([]float64, n)
	a.rsxTVOsc = make([]float64, n)
	a.rsxTVOpens = make([]int64, n)
	for i := 0; i < n; i++ {
		a.rsxTVCloses[i] = a.klines[i].Close
		a.rsxTVOsc[i] = rsx[i]
		a.rsxTVOpens[i] = a.klines[i].OpenTime
	}
}

func (a *Frame) rebuildRSTVFactsLocked() {
	a.rsxTVFacts = nil
	a.rsxTVCloses = nil
	a.rsxTVOsc = nil
	a.rsxTVOpens = nil
	hist := a.dagHistoryLocked()
	if hist == nil {
		return
	}
	n := hist.Count()
	if n > len(a.klines) {
		n = len(a.klines)
	}
	if n < 3 {
		return
	}
	a.resyncRSTVSeriesLocked(hist, n)
	lookback := a.effectiveRSXSettings().DivLookback
	a.rsxTVFacts = indicators.MergeTVFacts(
		indicators.TVDivergenceFacts(a.rsxTVCloses, a.rsxTVOsc, a.rsxTVOpens, lookback),
		indicators.TVPivotFacts(a.rsxTVCloses, a.rsxTVOsc, a.rsxTVOpens, lookback),
	)
}

func (a *Frame) trimRSTVFactsLocked() {
	if len(a.klines) == 0 {
		a.rsxTVFacts = nil
		a.rsxTVCloses = nil
		a.rsxTVOsc = nil
		a.rsxTVOpens = nil
		return
	}
	minOpen := a.klines[0].OpenTime
	drop := 0
	for drop < len(a.rsxTVOpens) && a.rsxTVOpens[drop] < minOpen {
		drop++
	}
	if drop > 0 {
		a.rsxTVCloses = a.rsxTVCloses[drop:]
		a.rsxTVOsc = a.rsxTVOsc[drop:]
		a.rsxTVOpens = a.rsxTVOpens[drop:]
	}
	if len(a.rsxTVFacts) == 0 {
		return
	}
	out := a.rsxTVFacts[:0]
	for _, ev := range a.rsxTVFacts {
		if ev.AnchorAt >= minOpen && ev.ConfirmedAt >= minOpen {
			out = append(out, ev)
		}
	}
	a.rsxTVFacts = out
}

func (a *Frame) dagHistoryLocked() *core.HistoryBus {
	if a == nil || a.dag == nil {
		return nil
	}
	bus := a.dag.Bus()
	if bus == nil {
		return nil
	}
	return bus.Hist
}

// RSTVFactsSnapshot copies published rsx_tv_div facts (read-only).
func (a *Frame) RSTVFactsSnapshot() []indicators.IndicatorFactEvent {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.rsxTVFacts) == 0 {
		return nil
	}
	return append([]indicators.IndicatorFactEvent(nil), a.rsxTVFacts...)
}

// RSTVFactsConfirmedAt returns TV facts whose ConfirmedAt equals openTimeMs.
func (a *Frame) RSTVFactsConfirmedAt(openTimeMs int64) []indicators.IndicatorFactEvent {
	if a == nil || openTimeMs <= 0 {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []indicators.IndicatorFactEvent
	for _, ev := range a.rsxTVFacts {
		if ev.ConfirmedAt == openTimeMs {
			out = append(out, ev)
		}
	}
	return out
}
