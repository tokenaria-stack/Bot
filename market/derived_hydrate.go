package market

import (
	"log"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

func parentTipIsForming(klines []exchange.Kline, parentTF string) bool {
	if len(klines) == 0 {
		return false
	}
	cur, err := data.CurrentBarOpen(time.Now().UnixMilli(), parentTF)
	if err != nil {
		return false
	}
	return klines[len(klines)-1].OpenTime == cur
}

// HydrateDerivedFrames builds derived Frames from parent RAM after native boot.
func HydrateDerivedFrames(frames map[string]*Frame, chaos ChaosConfig) {
	if frames == nil {
		return
	}
	for _, e := range exchange.DerivedTime() {
		parent := frames[e.Parent]
		var parents []exchange.Kline
		if parent != nil {
			parents = parent.GetKlines()
		}
		formingTip := parentTipIsForming(parents, e.Parent)
		closed, forming, err := exchange.FoldClosedChildren(parents, e.Name, formingTip)
		if err != nil {
			log.Printf("[Init] derived %s fold: %v", e.Name, err)
		}
		f := NewFrame(closed, e.Name, chaos)
		if forming != nil {
			f.UpdateKlineTick(*forming, false)
		}
		f.UpdateIndicators()
		frames[e.Name] = f
		log.Printf("[Init] Frame [%s] derived from %s (%d closed)", e.Name, e.Parent, len(closed))
	}
}

func (m *Runtime) initDerivedAccumulators() {
	if m == nil {
		return
	}
	m.derivedAcc = make(map[string]*exchange.DerivedAccumulator)
	for _, e := range exchange.DerivedTime() {
		acc, err := exchange.NewDerivedAccumulator(e.Name)
		if err != nil {
			log.Printf("[Master] derived accumulator %s: %v", e.Name, err)
			continue
		}
		if parent := m.frames[e.Parent]; parent != nil {
			kl := parent.GetKlines()
			acc.ResetFromParents(kl, parentTipIsForming(kl, e.Parent))
		}
		m.derivedAcc[e.Name] = acc
	}
}

func (m *Runtime) rebuildDerivedFromParent(parentTF string) {
	if m == nil || !exchange.IsNativeBinance(parentTF) {
		return
	}
	m.mu.RLock()
	parent := m.frames[parentTF]
	children := exchange.ChildrenOf(parentTF)
	m.mu.RUnlock()
	if parent == nil {
		return
	}
	parents := parent.GetKlines()
	formingTip := parentTipIsForming(parents, parentTF)
	for _, childID := range children {
		closed, forming, err := exchange.FoldClosedChildren(parents, childID, formingTip)
		if err != nil {
			log.Printf("[Master] rebuild derived %s: %v", childID, err)
			continue
		}
		m.mu.RLock()
		child := m.frames[childID]
		acc := m.derivedAcc[childID]
		m.mu.RUnlock()
		if child != nil {
			child.ReplaceWorkingKlines(closed)
			if forming != nil {
				child.UpdateKlineTick(*forming, false)
			}
			child.UpdateIndicators()
		}
		if acc != nil {
			acc.ResetFromParents(parents, formingTip)
		}
	}
}

func (m *Runtime) fanoutDerived(parentTF string, k exchange.Kline, isClosed bool) {
	if m == nil || !exchange.IsNativeBinance(parentTF) {
		return
	}
	for _, childID := range exchange.ChildrenOf(parentTF) {
		m.mu.RLock()
		acc := m.derivedAcc[childID]
		frame := m.frames[childID]
		cb := m.onKlineBar
		m.mu.RUnlock()
		if acc == nil || frame == nil {
			continue
		}
		child, childClosed, ok := acc.OnParent(k, isClosed)
		if !ok {
			continue
		}
		frame.UpdateKlineTick(child, childClosed)
		if cb != nil {
			cb(childID, child, childClosed)
		}
		if childClosed {
			frame.UpdateIndicators()
		}
	}
}
