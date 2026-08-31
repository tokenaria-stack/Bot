package market

import (
	"trading_bot/core/nodes"
	"trading_bot/indicators"
)

func (a *Frame) noteRSTZZFactLocked(isClosed bool, barIndex int) {
	if a == nil || !isClosed || a.dag == nil {
		return
	}
	if a.rsxApplied&nodes.NeedRSZZ == 0 {
		return
	}
	a.zzEvals++
	ev, ok := a.zzCollector.ObserveClosed(a.dag.Bus(), a.klines, barIndex)
	if !ok {
		return
	}
	a.rsxZZFacts = append(a.rsxZZFacts, ev)
}

func (a *Frame) trimRSTZZFactsLocked() {
	if len(a.klines) == 0 {
		a.rsxZZFacts = nil
		a.zzCollector = ZZDivFactCollector{}
		return
	}
	minOpen := a.klines[0].OpenTime
	if len(a.rsxZZFacts) == 0 {
		return
	}
	out := a.rsxZZFacts[:0]
	for _, ev := range a.rsxZZFacts {
		if ev.AnchorAt >= minOpen && ev.ConfirmedAt >= minOpen {
			out = append(out, ev)
		}
	}
	a.rsxZZFacts = out
}

// RSTZZFactsSnapshot copies published rsx_zz_div facts.
func (a *Frame) RSTZZFactsSnapshot() []indicators.IndicatorFactEvent {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.rsxZZFacts) == 0 {
		return nil
	}
	return append([]indicators.IndicatorFactEvent(nil), a.rsxZZFacts...)
}

func (a *Frame) RSTZZFactsConfirmedAt(openTimeMs int64) []indicators.IndicatorFactEvent {
	if a == nil || openTimeMs <= 0 {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []indicators.IndicatorFactEvent
	for _, ev := range a.rsxZZFacts {
		if ev.ConfirmedAt == openTimeMs {
			out = append(out, ev)
		}
	}
	return out
}

func dagRunnerBorn() int64 {
	return testDAGRunnerBorn.Load()
}
