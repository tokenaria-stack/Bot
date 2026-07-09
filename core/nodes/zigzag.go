package nodes

import (
	"trading_bot/core"
	"trading_bot/indicators"
)

const defaultZigZagSensitivity = 0.5

// ZigZagConfig parametrizes fractal lag for swing event bar indexing.
type ZigZagConfig struct {
	LeftBars    int
	RightBars   int
	Sensitivity float64
}

// ZigZagNode runs adaptive ZigZag with O(1) snapshot/restore for open-bar immunity.
type ZigZagNode struct {
	bus            *core.Bus
	cfg            ZigZagConfig
	zigzag         *indicators.ZigZag
	last           indicators.ZigZagUpdate
	prevZig        indicators.ZigZagNode
	prevZigHas     bool
	snapPrevZig    indicators.ZigZagNode
	snapPrevZigHas bool
}

// NewZigZagNode creates a ZigZag node with explicit fractal and sensitivity config.
func NewZigZagNode(cfg ZigZagConfig) *ZigZagNode {
	sens := cfg.Sensitivity
	if sens <= 0 {
		sens = defaultZigZagSensitivity
	}
	zz := indicators.NewZigZag(indicators.DefaultATRPeriod)
	zz.SetSensitivity(sens)
	zz.SetDynamicFractal(cfg.LeftBars, cfg.RightBars)
	return &ZigZagNode{
		cfg: ZigZagConfig{
			LeftBars:    cfg.LeftBars,
			RightBars:   cfg.RightBars,
			Sensitivity: sens,
		},
		zigzag: zz,
	}
}

func (n *ZigZagNode) Name() string { return "zigzag" }

func (n *ZigZagNode) Init(bus *core.Bus) {
	n.bus = bus
	n.normalizeConfig()
}

func (n *ZigZagNode) normalizeConfig() {
	if n.cfg.LeftBars <= 0 {
		n.cfg.LeftBars = 2
	}
	if n.cfg.RightBars <= 0 {
		n.cfg.RightBars = 2
	}
	if n.cfg.Sensitivity <= 0 {
		n.cfg.Sensitivity = defaultZigZagSensitivity
	}
}

func (n *ZigZagNode) Update() {
	if n.bus == nil || n.bus.Cur == nil || n.zigzag == nil {
		return
	}
	high := n.bus.Cur.Get(core.SlotPriceHigh)
	low := n.bus.Cur.Get(core.SlotPriceLow)
	close := n.bus.Cur.Get(core.SlotPriceClose)
	rsi := n.bus.Cur.Get(core.SlotJurikRSX)
	n.last = n.zigzag.UpdateCandle(high, low, close, rsi)

	if n.isNewZigZagNode(n.last) && n.bus.Events != nil {
		actualBarIndex := n.bus.Cur.BarIndex - n.cfg.RightBars
		if actualBarIndex < 0 {
			actualBarIndex = 0
		}
		n.bus.Events.Push(core.SwingEvent{
			BarIndex: actualBarIndex,
			IsHigh:   n.last.Node.IsHigh,
			Price:    n.last.Node.Price,
		})
	}
	if n.last.Node.Confirmed {
		n.prevZig = n.last.Node
		n.prevZigHas = true
	}
}

func (n *ZigZagNode) isNewZigZagNode(upd indicators.ZigZagUpdate) bool {
	if !upd.Node.Confirmed {
		return false
	}
	if !n.prevZigHas {
		return true
	}
	return upd.Node.Price != n.prevZig.Price || upd.Node.IsHigh != n.prevZig.IsHigh
}

func (n *ZigZagNode) SaveState() {
	if n.zigzag != nil {
		n.zigzag.SaveState()
	}
	n.snapPrevZig = n.prevZig
	n.snapPrevZigHas = n.prevZigHas
}

func (n *ZigZagNode) RestoreState() {
	if n.zigzag != nil {
		n.zigzag.RestoreState()
	}
	n.prevZig = n.snapPrevZig
	n.prevZigHas = n.snapPrevZigHas
}

func (n *ZigZagNode) OnConfigChange(cfg any) error {
	c, ok := cfg.(ZigZagConfig)
	if !ok {
		return nil
	}
	if c.LeftBars > 0 {
		n.cfg.LeftBars = c.LeftBars
	}
	if c.RightBars > 0 {
		n.cfg.RightBars = c.RightBars
	}
	if c.Sensitivity > 0 {
		n.cfg.Sensitivity = c.Sensitivity
		if n.zigzag != nil {
			n.zigzag.SetSensitivity(c.Sensitivity)
		}
	}
	n.normalizeConfig()
	if n.zigzag != nil {
		n.zigzag.SetDynamicFractal(n.cfg.LeftBars, n.cfg.RightBars)
	}
	return nil
}

// LastUpdate returns the latest ZigZag structural update (for shadow validation).
func (n *ZigZagNode) LastUpdate() indicators.ZigZagUpdate {
	return n.last
}

// HasConfirmedNode reports whether a swing node has been confirmed.
func (n *ZigZagNode) HasConfirmedNode() bool {
	return n.last.Node.Confirmed
}

// Config returns the active fractal configuration.
func (n *ZigZagNode) Config() ZigZagConfig {
	return n.cfg
}
