package nodes

import (
	"math"

	"trading_bot/core"
)

const (
	saucerRSIThreshold = 30.0
	saucerScore        = 15.0
	vSpikeScore        = 20.0
)

// MicroPatternNode detects saucer and V-spike micro patterns on any oscillator slot.
type MicroPatternNode struct {
	bus        *core.Bus
	targetSlot core.Slot

	current, prev, prevPrev float64
	tickCount               int

	snapCurrent, snapPrev, snapPrevPrev float64
	snapTickCount                       int
}

// NewMicroPatternNode creates a micro-pattern detector for the given slot.
func NewMicroPatternNode(targetSlot core.Slot) *MicroPatternNode {
	return &MicroPatternNode{targetSlot: targetSlot}
}

func (n *MicroPatternNode) Name() string { return "micro_pattern" }

func (n *MicroPatternNode) Init(bus *core.Bus) { n.bus = bus }

func (n *MicroPatternNode) Update() {
	if n.bus == nil || n.bus.Cur == nil {
		return
	}

	n.bus.Cur.Set(core.SlotMicroDivScore, 0)

	val := n.bus.Cur.Get(n.targetSlot)
	n.shift(val)

	if n.tickCount < 3 {
		return
	}

	n.bus.Cur.Set(core.SlotMicroDivScore, n.scoreMicro(n.current, n.prev, n.prevPrev))
}

func (n *MicroPatternNode) shift(val float64) {
	n.prevPrev = n.prev
	n.prev = n.current
	n.current = val
	if n.tickCount < 3 {
		n.tickCount++
	}
}

func (n *MicroPatternNode) scoreMicro(current, prev, prevPrev float64) float64 {
	score := 0.0
	if detectSaucer(current, prev, prevPrev) {
		score += saucerScore
	}
	if detectVSpike(current, prev, prevPrev) {
		score += vSpikeScore
	}
	return score
}

func detectSaucer(current, prev, prevPrev float64) bool {
	if current >= saucerRSIThreshold || prev >= saucerRSIThreshold || prevPrev >= saucerRSIThreshold {
		return false
	}
	v1 := prev - prevPrev
	v2 := current - prev
	a := v2 - v1
	return v1 < 0 && v2 > v1 && a > 0
}

func detectVSpike(current, prev, prevPrev float64) bool {
	vPrev := prev - prevPrev
	vCurr := current - prev
	a := vCurr - vPrev
	strongNeg := vPrev < -2
	strongPos := vCurr > 2
	extremeAccel := math.Abs(a) > 3
	return strongNeg && strongPos && extremeAccel && vPrev*vCurr < 0
}

func (n *MicroPatternNode) SaveState() {
	n.snapCurrent = n.current
	n.snapPrev = n.prev
	n.snapPrevPrev = n.prevPrev
	n.snapTickCount = n.tickCount
}

func (n *MicroPatternNode) RestoreState() {
	n.current = n.snapCurrent
	n.prev = n.snapPrev
	n.prevPrev = n.snapPrevPrev
	n.tickCount = n.snapTickCount
}

func (n *MicroPatternNode) OnConfigChange(cfg any) error {
	if slot, ok := cfg.(core.Slot); ok && slot < core.SlotCount {
		n.targetSlot = slot
	}
	return nil
}
