package nodes

import (
	"math"

	"trading_bot/core"
)

// DivergenceNodeConfig selects the oscillator slot and macro divergence weights.
type DivergenceNodeConfig struct {
	TargetSlot       core.Slot
	BullishWeight    float64
	BearishWeight    float64
	HiddenBullWeight float64
}

// DivergenceNode detects macro divergences from swing events vs any configured oscillator slot.
type DivergenceNode struct {
	bus              *core.Bus
	targetSlot       core.Slot
	bullishWeight    float64
	bearishWeight    float64
	hiddenBullWeight float64
}

// NewDivergenceNode creates a slot-agnostic macro divergence detector.
func NewDivergenceNode(cfg DivergenceNodeConfig) *DivergenceNode {
	wBull := cfg.BullishWeight
	if wBull <= 0 {
		wBull = 15
	}
	wBear := cfg.BearishWeight
	if wBear <= 0 {
		wBear = 15
	}
	wHidden := cfg.HiddenBullWeight
	if wHidden <= 0 {
		wHidden = 30
	}
	return &DivergenceNode{
		targetSlot:       cfg.TargetSlot,
		bullishWeight:    wBull,
		bearishWeight:    wBear,
		hiddenBullWeight: wHidden,
	}
}

func (n *DivergenceNode) Name() string { return "divergence" }

func (n *DivergenceNode) Init(bus *core.Bus) { n.bus = bus }

func (n *DivergenceNode) Update() {
	if n.bus == nil || n.bus.Cur == nil || n.bus.Events == nil {
		return
	}

	n.bus.Cur.Set(core.SlotDivScore, 0)
	n.bus.Cur.Set(core.SlotDivState, core.DivStateNone)

	events := n.bus.Events.GetLast(3)
	if len(events) < 2 {
		return
	}

	curBar := n.bus.Cur.BarIndex
	latest := events[0]
	prev := events[1]

	if !n.histBarsAvailable(curBar, latest.BarIndex) || !n.histBarsAvailable(curBar, prev.BarIndex) {
		return
	}

	oscLatest := core.HistValueAtBar(n.bus, n.targetSlot, curBar, latest.BarIndex)
	oscPrev := core.HistValueAtBar(n.bus, n.targetSlot, curBar, prev.BarIndex)
	if math.IsNaN(oscLatest) || math.IsNaN(oscPrev) {
		return
	}

	score, state := n.macroScore(latest, prev, oscLatest, oscPrev)
	n.bus.Cur.Set(core.SlotDivScore, score)
	n.bus.Cur.Set(core.SlotDivState, state)
}

func (n *DivergenceNode) histBarsAvailable(curBar, eventBar int) bool {
	if n.bus == nil || n.bus.Hist == nil {
		return false
	}
	barsAgo := curBar - eventBar
	if barsAgo <= 0 {
		return true
	}
	return barsAgo < n.bus.Hist.Cap() && barsAgo <= n.bus.Hist.Count()
}

func (n *DivergenceNode) macroScore(latest, prev core.SwingEvent, oscLatest, oscPrev float64) (float64, float64) {
	if latest.IsHigh && prev.IsHigh {
		if latest.Price > prev.Price && oscLatest < oscPrev {
			return n.bearishWeight, core.DivStateS
		}
		return 0, core.DivStateNone
	}
	if !latest.IsHigh && !prev.IsHigh {
		if latest.Price < prev.Price && oscLatest > oscPrev {
			return n.bullishWeight, core.DivStateL
		}
		if latest.Price > prev.Price && oscLatest < oscPrev {
			return n.hiddenBullWeight, core.DivStateLL
		}
	}
	return 0, core.DivStateNone
}

func (n *DivergenceNode) SaveState()    {}
func (n *DivergenceNode) RestoreState() {}

func (n *DivergenceNode) OnConfigChange(cfg any) error {
	c, ok := cfg.(DivergenceNodeConfig)
	if !ok {
		return nil
	}
	if c.TargetSlot < core.SlotCount {
		n.targetSlot = c.TargetSlot
	}
	if c.BullishWeight > 0 {
		n.bullishWeight = c.BullishWeight
	}
	if c.BearishWeight > 0 {
		n.bearishWeight = c.BearishWeight
	}
	if c.HiddenBullWeight > 0 {
		n.hiddenBullWeight = c.HiddenBullWeight
	}
	return nil
}
