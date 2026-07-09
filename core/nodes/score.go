package nodes

import (
	"math"

	"trading_bot/core"
)

// ScoreConfig maps input slots to their aggregation weights.
type ScoreConfig struct {
	Weights map[core.Slot]float64
}

// ScoreNode aggregates weighted slot values into SlotTotalScore.
type ScoreNode struct {
	bus     *core.Bus
	weights map[core.Slot]float64
}

// NewScoreNode creates a stateless score aggregator from the given config.
func NewScoreNode(cfg ScoreConfig) *ScoreNode {
	weights := make(map[core.Slot]float64, len(cfg.Weights))
	for slot, weight := range cfg.Weights {
		if slot < core.SlotCount {
			weights[slot] = weight
		}
	}
	return &ScoreNode{weights: weights}
}

func (n *ScoreNode) Name() string { return "score" }

func (n *ScoreNode) Init(bus *core.Bus) { n.bus = bus }

func (n *ScoreNode) Update() {
	if n.bus == nil || n.bus.Cur == nil {
		return
	}

	total := 0.0
	for slot, weight := range n.weights {
		val := n.bus.Cur.Get(slot)
		if math.IsNaN(val) {
			val = 0
		}
		total += val * weight
	}
	n.bus.Cur.Set(core.SlotTotalScore, total)
}

func (n *ScoreNode) SaveState()    {}
func (n *ScoreNode) RestoreState() {}

func (n *ScoreNode) OnConfigChange(cfg any) error {
	c, ok := cfg.(ScoreConfig)
	if !ok {
		return nil
	}
	for slot, weight := range c.Weights {
		if slot < core.SlotCount {
			n.weights[slot] = weight
		}
	}
	return nil
}
