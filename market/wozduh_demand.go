package market

import (
	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

func wozduhInternalMask() nodes.WozduhMask {
	if !EngineAllowsStrategies() {
		return 0
	}
	// validateDAGShadowLocked reads woz_fast / woz_slow. /api/state packing,
	// Navigator ReplayClosedBars, and chart_cache hist rows are not demand.
	return nodes.WozduhBitVolBase | nodes.WozduhBitWt11 | nodes.WozduhBitWt22
}

// SetWozduhDemand sets this Frame's live Wozduh compute mask to clientUnion OR
// the proven internal consumer mask. Sleep/wake run under Frame.mu.
func (a *Frame) SetWozduhDemand(clientUnion nodes.WozduhMask) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	required := clientUnion | wozduhInternalMask()
	a.wozduhDemand = required
	a.applyWozduhDemandLocked(required)
}

func (a *Frame) applyWozduhDemandLocked(required nodes.WozduhMask) {
	woz := wozduhNodeFromDAG(a.dag)
	if woz == nil {
		return
	}
	old := woz.Mask()
	wake := required &^ old
	sleep := old &^ required
	if wake == 0 && sleep == 0 {
		return
	}
	if wake != 0 {
		closed := a.closedKlinesTailLocked(dagHistoryCap)
		temp := nodes.NewWozduhNodeMasked(nodes.WozduhWakeReplayMask(wake))
		replayWozduhClosedBars(temp, closed)
		woz.InstallWokenFields(temp, wake)
		woz.SaveState()
	}
	woz.ApplyMask(required)
	if wake != 0 {
		woz.RestoreState()
		woz.Update()
	}
}

func wozduhNodeFromDAG(dag *core.DAGRunner) *nodes.WozduhNode {
	if dag == nil {
		return nil
	}
	n, _ := dag.NodeByName("wozduh").(*nodes.WozduhNode)
	return n
}

func (a *Frame) closedKlinesTailLocked(maxBars int) []exchange.Kline {
	ks := a.klines
	if len(ks) == 0 {
		return nil
	}
	if a.lastCommittedOpenTime > 0 && ks[len(ks)-1].OpenTime != a.lastCommittedOpenTime {
		ks = ks[:len(ks)-1]
	}
	if maxBars > 0 && len(ks) > maxBars {
		ks = ks[len(ks)-maxBars:]
	}
	out := make([]exchange.Kline, len(ks))
	copy(out, ks)
	return out
}

func replayWozduhClosedBars(n *nodes.WozduhNode, klines []exchange.Kline) {
	if n == nil {
		return
	}
	capN := len(klines)
	if capN < 1 {
		capN = 1
	}
	bus := core.NewBus(core.ValidateHistoryCap(capN))
	n.Init(bus)
	for _, k := range klines {
		cur := bus.Cur
		cur.Set(core.SlotPriceOpen, k.Open)
		cur.Set(core.SlotPriceHigh, k.High)
		cur.Set(core.SlotPriceLow, k.Low)
		cur.Set(core.SlotPriceClose, k.Close)
		cur.Set(core.SlotVolume, k.Volume)
		n.Update()
		n.SaveState()
	}
}

// WozduhLiveStats is test/measure access to the persistent Wozduh node.
func (a *Frame) WozduhLiveStats() (mask nodes.WozduhMask, streams, wakes int) {
	if a == nil {
		return 0, 0, 0
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	woz := wozduhNodeFromDAG(a.dag)
	if woz == nil {
		return 0, 0, 0
	}
	return woz.Mask(), woz.StreamUpdates(), woz.WakeInstalls()
}

// WozduhOrangePtr is a test hook for shared-base identity.
func (a *Frame) WozduhOrangePtr() *indicators.RSI {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	woz := wozduhNodeFromDAG(a.dag)
	if woz == nil {
		return nil
	}
	return woz.OrangeRsiPtr()
}

// WozduhWt11Ptr is a test hook for vol-base identity.
func (a *Frame) WozduhWt11Ptr() *indicators.EMA {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	woz := wozduhNodeFromDAG(a.dag)
	if woz == nil {
		return nil
	}
	return woz.Wt11EmaPtr()
}

// WozduhSlot is a test hook for current TickFrame Wozduh values.
func (a *Frame) WozduhSlot(slot core.Slot) float64 {
	if a == nil {
		return 0
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.dag == nil {
		return 0
	}
	bus := a.dag.Bus()
	if bus == nil || bus.Cur == nil {
		return 0
	}
	return bus.Cur.Get(slot)
}
