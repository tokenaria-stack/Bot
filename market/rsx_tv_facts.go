package market

import (
	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

// RSTVFactsFromClosedSeries emits Pine TV divergence and TV pivot facts from aligned closed bars.
// rsx[i] must be SlotJurikRSX for klines[i]. Lookback follows RSX settings.
func RSTVFactsFromClosedSeries(klines []exchange.Kline, rsx []float64, lookback int) []indicators.IndicatorFactEvent {
	facts, _ := replayRSTV(klines, rsx, lookback)
	return facts
}

func replayRSTV(klines []exchange.Kline, rsx []float64, lookback int) ([]indicators.IndicatorFactEvent, *indicators.RSTVState) {
	n := len(klines)
	if n == 0 || len(rsx) != n {
		return nil, indicators.NewRSTVState(lookback)
	}
	closes := make([]float64, n)
	opens := make([]int64, n)
	for i, k := range klines {
		closes[i] = k.Close
		opens[i] = k.OpenTime
	}
	return indicators.ReplayRSTV(opens, closes, rsx, lookback)
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
	if a.rsxApplied&nodes.NeedRSTV == 0 {
		return
	}
	a.tvEvals++
	bus := a.dag.Bus()
	if bus == nil || bus.Cur == nil {
		return
	}
	k := a.klines[barIndex]
	osc := bus.Cur.Get(core.SlotJurikRSX)
	if a.rstv == nil {
		a.rstv = indicators.NewRSTVState(a.effectiveRSXSettings().DivLookback)
	}
	if a.rstv.Started() && a.rstv.LastOpenTime() == k.OpenTime {
		return
	}
	evs, err := a.rstv.UpdateClosed(k.OpenTime, k.Close, osc)
	if err != nil {
		return
	}
	confirmed := k.OpenTime
	kept := a.rsxTVFacts[:0]
	for _, ev := range a.rsxTVFacts {
		if ev.ConfirmedAt != confirmed {
			kept = append(kept, ev)
		}
	}
	a.rsxTVFacts = append(kept, evs.Facts()...)
}

func (a *Frame) rebuildRSTVFactsLocked() {
	a.rsxTVFacts = nil
	a.rstv = nil
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
	rsx := historySlotSeries(hist, core.SlotJurikRSX)
	if len(rsx) < n {
		n = len(rsx)
	}
	facts, st := replayRSTV(a.klines[:n], rsx[:n], a.effectiveRSXSettings().DivLookback)
	a.rsxTVFacts = facts
	a.rstv = st
}

func (a *Frame) trimRSTVFactsLocked() {
	if len(a.klines) == 0 {
		a.rsxTVFacts = nil
		a.rstv = nil
		return
	}
	minOpen := a.klines[0].OpenTime
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

// RSTVFactsSnapshot copies published rsx_tv_div / rsx_tv_pivot facts (read-only).
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
