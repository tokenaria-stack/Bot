package market

import (
	"math"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

func rsxInternalMask() nodes.RSXWorkMask {
	if !EngineAllowsStrategies() {
		return 0
	}
	return nodes.NeedRSXCore
}

func (a *Frame) rsxUnionLocked() nodes.RSXWorkMask {
	return a.rsxClientDemand | rsxInternalMask() | a.rsxForecastDemand
}

func (a *Frame) recomputeRSXDemandLocked() {
	required := a.rsxUnionLocked()
	a.rsxDemand = required
	a.applyRSXDemandLocked(required)
}

// SetRSXDemand stores the chart/client RSX contribution and recomputes
// applied = client | internal | forecast. It does not clear forecast demand.
func (a *Frame) SetRSXDemand(clientUnion nodes.RSXWorkMask) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rsxClientDemand = clientUnion
	a.recomputeRSXDemandLocked()
}

// SetRSXForecastDemand stores the Forecast RSX contribution and recomputes
// applied = client | internal | forecast. It does not clear client demand.
func (a *Frame) SetRSXForecastDemand(forecastMask nodes.RSXWorkMask) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rsxForecastDemand = forecastMask
	a.recomputeRSXDemandLocked()
}

func (a *Frame) applyRSXDemandLocked(required nodes.RSXWorkMask) {
	rsxN := rsxNodeFromDAG(a.dag)
	zzN := zigzagNodeFromDAG(a.dag)
	old := a.rsxApplied
	wake := required &^ old
	sleep := old &^ required
	if wake == 0 && sleep == 0 {
		return
	}

	closed := a.closedKlinesTailLocked(dagHistoryCap)
	var rsxSeries, sigs []float64
	coreWasOn := old&nodes.NeedRSXCore != 0

	if wake&nodes.NeedRSXCore != 0 && rsxN != nil {
		temp := nodes.NewRSXNode(a.effectiveRSXSettings().Length, a.effectiveRSXSettings().SignalLength, a.effectiveRSXSettings().Source)
		rsxSeries, sigs = replayRSXClosedBars(temp, closed)
		rsxN.InstallWoken(temp)
		rsxN.SaveState()
		a.rsxCoreRebuilds++
		if hist := a.dagHistoryLocked(); hist != nil {
			hist.PatchNewest(core.SlotJurikRSX, rsxSeries)
			hist.PatchNewest(core.SlotJurikSignal, sigs)
		}
	} else if coreWasOn {
		rsxSeries = a.closedJurikSeriesLocked(closed)
	}

	if wake&(nodes.NeedRSTV|nodes.NeedRSFractal|nodes.NeedRSZZ) != 0 && rsxSeries == nil && coreWasOn {
		rsxSeries = a.closedJurikSeriesLocked(closed)
	}

	if wake&nodes.NeedRSZZ != 0 && zzN != nil && len(closed) > 0 && len(rsxSeries) == len(closed) {
		tempZZ := nodes.NewZigZagNode(nodes.ZigZagConfig{LeftBars: 2, RightBars: 2})
		var col ZZDivFactCollector
		facts := replayZigZagWithRSX(tempZZ, closed, rsxSeries, &col)
		zzN.InstallWoken(tempZZ)
		zzN.SaveState()
		a.replaceZZFactsLocked(facts, closed)
		a.zzCollector = col
	}

	if wake&nodes.NeedRSTV != 0 {
		a.wakeTVFactsLocked(closed, rsxSeries)
	}
	if wake&nodes.NeedRSFractal != 0 {
		a.wakeFractalFactsLocked(closed, rsxSeries)
	}

	if rsxN != nil {
		rsxN.ApplyActive(required&nodes.NeedRSXCore != 0)
	}
	if zzN != nil {
		zzN.ApplyActive(required&nodes.NeedRSZZ != 0)
	}
	a.rsxApplied = required

	forming := false
	if len(a.klines) > 0 && a.lastCommittedOpenTime != 0 &&
		a.klines[len(a.klines)-1].OpenTime != a.lastCommittedOpenTime {
		forming = true
	}
	if wake&nodes.NeedRSXCore != 0 && rsxN != nil {
		rsxN.RestoreState()
		if forming {
			rsxN.Update()
		} else if bus := a.dag.Bus(); bus != nil && bus.Cur != nil && len(rsxSeries) > 0 {
			last := len(rsxSeries) - 1
			bus.Cur.Set(core.SlotJurikRSX, rsxSeries[last])
			if last < len(sigs) {
				bus.Cur.Set(core.SlotJurikSignal, sigs[last])
			}
		}
	}
	if wake&nodes.NeedRSZZ != 0 && zzN != nil && required&nodes.NeedRSZZ != 0 {
		zzN.RestoreState()
		if forming {
			zzN.Update()
		}
	}
}

func rsxNodeFromDAG(dag *core.DAGRunner) *nodes.RSXNode {
	if dag == nil {
		return nil
	}
	n, _ := dag.NodeByName("rsx").(*nodes.RSXNode)
	return n
}

func zigzagNodeFromDAG(dag *core.DAGRunner) *nodes.ZigZagNode {
	if dag == nil {
		return nil
	}
	n, _ := dag.NodeByName("zigzag").(*nodes.ZigZagNode)
	return n
}

func (a *Frame) closedJurikSeriesLocked(closed []exchange.Kline) []float64 {
	hist := a.dagHistoryLocked()
	if hist == nil || len(closed) == 0 {
		return nil
	}
	all := historySlotSeries(hist, core.SlotJurikRSX)
	if len(all) < len(closed) {
		return nil
	}
	return all[len(all)-len(closed):]
}

func replayRSXClosedBars(n *nodes.RSXNode, klines []exchange.Kline) (jurik, sigs []float64) {
	if n == nil {
		return nil, nil
	}
	capN := len(klines)
	if capN < 1 {
		capN = 1
	}
	bus := core.NewBus(core.ValidateHistoryCap(capN))
	n.Init(bus)
	n.ApplyActive(true)
	jurik = make([]float64, 0, len(klines))
	sigs = make([]float64, 0, len(klines))
	for _, k := range klines {
		cur := bus.Cur
		cur.Set(core.SlotPriceOpen, k.Open)
		cur.Set(core.SlotPriceHigh, k.High)
		cur.Set(core.SlotPriceLow, k.Low)
		cur.Set(core.SlotPriceClose, k.Close)
		n.Update()
		n.SaveState()
		jurik = append(jurik, cur.Get(core.SlotJurikRSX))
		sigs = append(sigs, cur.Get(core.SlotJurikSignal))
	}
	return jurik, sigs
}

func replayZigZagWithRSX(n *nodes.ZigZagNode, klines []exchange.Kline, rsx []float64, col *ZZDivFactCollector) []indicators.IndicatorFactEvent {
	if n == nil || len(klines) == 0 || len(rsx) != len(klines) {
		return nil
	}
	capN := len(klines)
	bus := core.NewBus(core.ValidateHistoryCap(capN))
	n.Init(bus)
	n.ApplyActive(true)
	var out []indicators.IndicatorFactEvent
	for i, k := range klines {
		cur := bus.Cur
		cur.BarIndex = i
		cur.Set(core.SlotPriceOpen, k.Open)
		cur.Set(core.SlotPriceHigh, k.High)
		cur.Set(core.SlotPriceLow, k.Low)
		cur.Set(core.SlotPriceClose, k.Close)
		cur.Set(core.SlotJurikRSX, rsx[i])
		n.Update()
		n.SaveState()
		if bus.Events != nil {
			bus.Events.SaveState()
		}
		if bus.Hist != nil {
			bus.Hist.PushFrame(cur)
			bus.Hist.Advance()
		}
		if col != nil {
			if ev, ok := col.ObserveClosed(bus, klines, i); ok {
				out = append(out, ev)
			}
		}
	}
	return out
}

func (a *Frame) wakeTVFactsLocked(closed []exchange.Kline, rsx []float64) {
	if len(closed) == 0 || len(rsx) != len(closed) {
		return
	}
	a.tvEvals++
	rebuilt := RSTVFactsFromClosedSeries(closed, rsx, a.effectiveRSXSettings().DivLookback)
	t0, t1 := windowOpenTimes(closed)
	next, changed := replaceFamilyFacts(a.rsxTVFacts, rebuilt, map[string]bool{
		indicators.FactSourceRSXTVDiv:   true,
		indicators.FactSourceRSXTVPivot: true,
	}, t0, t1)
	a.rsxTVFacts = next
	a.rsxTVCloses = make([]float64, len(closed))
	a.rsxTVOsc = append([]float64(nil), rsx...)
	a.rsxTVOpens = make([]int64, len(closed))
	for i, k := range closed {
		a.rsxTVCloses[i] = k.Close
		a.rsxTVOpens[i] = k.OpenTime
	}
	if changed {
		a.rsxFactRevision++
	}
}

func (a *Frame) wakeFractalFactsLocked(closed []exchange.Kline, rsx []float64) {
	if len(closed) == 0 || len(rsx) != len(closed) {
		return
	}
	a.fractalEvals++
	rebuilt := RSTFractalFactsFromClosedSeries(closed, rsx, a.effectiveRSXSettings())
	t0, t1 := windowOpenTimes(closed)
	next, changed := replaceFamilyFacts(a.rsxFractalFacts, rebuilt, map[string]bool{
		indicators.FactSourceRSXFractalDiv:   true,
		indicators.FactSourceRSXFractalPivot: true,
	}, t0, t1)
	a.rsxFractalFacts = next
	src := a.effectiveRSXSettings().Source
	a.rsxFractalPrices = fractalPricesFromKlines(closed, src)
	a.rsxFractalOsc = append([]float64(nil), rsx...)
	a.rsxFractalOpens = fractalOpensFromKlines(closed)
	if changed {
		a.rsxFactRevision++
	}
}

func (a *Frame) replaceZZFactsLocked(rebuilt []indicators.IndicatorFactEvent, closed []exchange.Kline) {
	t0, t1 := windowOpenTimes(closed)
	next, changed := replaceFamilyFacts(a.rsxZZFacts, rebuilt, map[string]bool{
		indicators.FactSourceRSXZZDiv: true,
	}, t0, t1)
	a.rsxZZFacts = next
	if changed {
		a.rsxFactRevision++
	}
}

func windowOpenTimes(closed []exchange.Kline) (t0, t1 int64) {
	if len(closed) == 0 {
		return 0, 0
	}
	return closed[0].OpenTime, closed[len(closed)-1].OpenTime
}

func factTupleEq(a, b indicators.IndicatorFactEvent) bool {
	return a.Source == b.Source && a.Direction == b.Direction && a.Pattern == b.Pattern &&
		a.ConfirmedAt == b.ConfirmedAt && a.AnchorAt == b.AnchorAt &&
		a.AnchorValue == b.AnchorValue && a.AnchorPrice == b.AnchorPrice
}

func replaceFamilyFacts(dst, rebuilt []indicators.IndicatorFactEvent, sources map[string]bool, t0, t1 int64) ([]indicators.IndicatorFactEvent, bool) {
	orig := append([]indicators.IndicatorFactEvent(nil), dst...)
	kept := make([]indicators.IndicatorFactEvent, 0, len(dst))
	for _, ev := range dst {
		inSrc := sources[ev.Source]
		inWin := ev.ConfirmedAt >= t0 && ev.ConfirmedAt <= t1
		if inSrc && inWin {
			continue
		}
		kept = append(kept, ev)
	}
	out := append(kept, rebuilt...)
	seen := make(map[string]struct{}, len(out))
	deduped := make([]indicators.IndicatorFactEvent, 0, len(out))
	for _, ev := range out {
		key := ev.Source + "|" + ev.Direction + "|" + ev.Pattern + "|" +
			itoa64(ev.ConfirmedAt) + "|" + itoa64(ev.AnchorAt)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, ev)
	}
	return deduped, !factSetsEqual(orig, deduped)
}

func factSetsEqual(a, b []indicators.IndicatorFactEvent) bool {
	if len(a) != len(b) {
		return false
	}
	type key struct {
		src, dir, pat string
		conf, anc     int64
		val, px       float64
	}
	counts := make(map[key]int, len(a))
	for _, ev := range a {
		counts[key{ev.Source, ev.Direction, ev.Pattern, ev.ConfirmedAt, ev.AnchorAt, ev.AnchorValue, ev.AnchorPrice}]++
	}
	for _, ev := range b {
		k := key{ev.Source, ev.Direction, ev.Pattern, ev.ConfirmedAt, ev.AnchorAt, ev.AnchorValue, ev.AnchorPrice}
		counts[k]--
		if counts[k] < 0 {
			return false
		}
	}
	return true
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func applyRSXBirthMask(dag *core.DAGRunner, mask nodes.RSXWorkMask) {
	if rsx := rsxNodeFromDAG(dag); rsx != nil {
		rsx.ApplyActive(mask&nodes.NeedRSXCore != 0)
	}
	if zz := zigzagNodeFromDAG(dag); zz != nil {
		zz.ApplyActive(mask&nodes.NeedRSZZ != 0)
	}
}

func (a *Frame) RSXLiveStats() (mask nodes.RSXWorkMask, rsxUpdates, zzUpdates, tvEvals, fractalEvals, zzEvals, factRev, coreRebuilds int) {
	if a == nil {
		return 0, 0, 0, 0, 0, 0, 0, 0
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	rsxN := rsxNodeFromDAG(a.dag)
	zzN := zigzagNodeFromDAG(a.dag)
	mask = a.rsxDemand
	if rsxN != nil {
		rsxUpdates = rsxN.StreamUpdates()
	}
	if zzN != nil {
		zzUpdates = zzN.StreamUpdates()
	}
	return mask, rsxUpdates, zzUpdates, a.tvEvals, a.fractalEvals, a.zzEvals, a.rsxFactRevision, a.rsxCoreRebuilds
}

func (a *Frame) RSXJurikPtr() *indicators.JurikRSX {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	n := rsxNodeFromDAG(a.dag)
	if n == nil {
		return nil
	}
	return n.JurikPtr()
}

func (a *Frame) RSXSlot(slot core.Slot) float64 {
	if a == nil {
		return math.NaN()
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.dag == nil || a.dag.Bus() == nil || a.dag.Bus().Cur == nil {
		return math.NaN()
	}
	return a.dag.Bus().Cur.Get(slot)
}

func (a *Frame) FrameZigZagPtr() *indicators.ZigZag {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.zigzag
}
