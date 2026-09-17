package market

import (
	"sync/atomic"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

const dagShadowEpsilon = 1e-4

const dagHistoryCap = 1024

// testDAGRunnerBorn counts newDAGRunner allocations (SIGNAL-2A.1 single-walk proof).
var testDAGRunnerBorn atomic.Int64

func newDAGRunner(historyCap int, rsx RSXSettings) *core.DAGRunner {
	return newDAGRunnerMasked(historyCap, rsx, nodes.WozduhMaskAll)
}

func newDAGRunnerMasked(historyCap int, rsx RSXSettings, wozMask nodes.WozduhMask) *core.DAGRunner {
	testDAGRunnerBorn.Add(1)
	normalized := NormalizeRSXSettings(rsx)
	bus := core.NewBus(historyCap)
	runner := core.NewDAGRunner(bus)
	runner.AddNode(nodes.NewRSXNode(normalized.Length, normalized.SignalLength, normalized.Source))
	runner.AddNode(nodes.NewWozduhNodeMasked(wozMask))
	runner.AddNode(nodes.NewZigZagNode(nodes.ZigZagConfig{
		LeftBars:  2,
		RightBars: 2,
	}))
	return runner
}

// ClosedDAGReplay is one closed-bar DAG walk: scalar hist plus ZZ facts from that walk.
type ClosedDAGReplay struct {
	Hist    *core.HistoryBus
	ZZFacts []indicators.IndicatorFactEvent
}

// ReplayClosedBars is the single sequential closed-bar DAG walk (scalars + rsx_zz_div).
// Wozduh compute-all. Callers that know plot demand should use ReplayClosedBarsMasked.
func ReplayClosedBars(klines []exchange.Kline, rsx RSXSettings) ClosedDAGReplay {
	return ReplayClosedBarsMasked(klines, rsx, nodes.WozduhMaskAll)
}

// ReplayClosedBarsMasked is ReplayClosedBars with a fixed Wozduh compute mask for the whole walk.
// Zero mask skips all Wozduh streams. RSX and ZigZag are unchanged.
func ReplayClosedBarsMasked(klines []exchange.Kline, rsx RSXSettings, wozMask nodes.WozduhMask) ClosedDAGReplay {
	if len(klines) == 0 {
		return ClosedDAGReplay{}
	}
	return replayClosedBarsCap(klines, rsx, core.ValidateHistoryCap(len(klines)), wozMask)
}

func replayClosedBarsCap(klines []exchange.Kline, rsx RSXSettings, histCap int, wozMask nodes.WozduhMask) ClosedDAGReplay {
	if len(klines) == 0 {
		return ClosedDAGReplay{}
	}
	runner := newDAGRunnerMasked(histCap, rsx, wozMask)
	var col ZZDivFactCollector
	out := make([]indicators.IndicatorFactEvent, 0)
	for i, k := range klines {
		runner.TickUpdate(k.Open, k.High, k.Low, k.Close, k.Volume, i, true)
		if ev, ok := col.ObserveClosed(runner.Bus(), klines, i); ok {
			out = append(out, ev)
		}
	}
	bus := runner.Bus()
	if bus == nil {
		return ClosedDAGReplay{ZZFacts: out}
	}
	return ClosedDAGReplay{Hist: bus.Hist, ZZFacts: out}
}

// ReplayDAGKlines runs the DAG over closed klines and returns the populated history ring.
func ReplayDAGKlines(klines []exchange.Kline, rsx RSXSettings) *core.HistoryBus {
	return ReplayClosedBars(klines, rsx).Hist
}

func (a *Frame) initDAGShadowLocked() {
	wozBirth := a.wozduhDemand | wozduhInternalMask()
	a.dag = newDAGRunnerMasked(dagHistoryCap, a.effectiveRSXSettings(), wozBirth)
	rsxBirth := a.rsxUnionLocked()
	a.rsxDemand = rsxBirth
	a.rsxApplied = rsxBirth
	applyRSXBirthMask(a.dag, rsxBirth)
}

func (a *Frame) runDAGShadowLocked(k exchange.Kline, barIndex int, isClosed bool) {
	if a.dag == nil {
		return
	}
	a.dag.TickUpdate(k.Open, k.High, k.Low, k.Close, k.Volume, barIndex, isClosed)
}

// DAGTickFrame returns the current DAG bus frame for dual-write projection (read-only).
func (a *Frame) DAGTickFrame() *core.TickFrame {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.dag == nil {
		return nil
	}
	bus := a.dag.Bus()
	if bus == nil {
		return nil
	}
	return bus.Cur
}
