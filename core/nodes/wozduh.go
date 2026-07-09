package nodes

import (
	"trading_bot/core"
	"trading_bot/indicators"
)

// Wozduh volume RSI periods — must match strategy/falcon.go (wozduhLenVol, wt11, wt22).
const (
	wozduhLenVol     = 24
	wozduhWt11Period = 12
	wozduhWt22Period = 5
)

// WozduhNode computes wt11/wt22 (volume RSI fast/slow) into the data bus.
// Pine: rsi11 = rsi(VWEMA(close), lenvol); wt11 = ema(rsi11, 12); wt22 = ema(rsi11, 5).
type WozduhNode struct {
	bus     *core.Bus
	volVwap *indicators.VolumeWeightedEMA
	volRsi  *indicators.RSI
	wt11Ema *indicators.EMA
	wt22Ema *indicators.EMA
}

// NewWozduhNode creates a Wozduh wt11/wt22 pipeline with Falcon-default periods.
func NewWozduhNode() *WozduhNode {
	return &WozduhNode{
		volVwap: indicators.NewVolumeWeightedEMA(wozduhLenVol),
		volRsi:  indicators.NewRSI(wozduhLenVol),
		wt11Ema: indicators.NewEMA(wozduhWt11Period),
		wt22Ema: indicators.NewEMA(wozduhWt22Period),
	}
}

func (n *WozduhNode) Name() string { return "wozduh" }

func (n *WozduhNode) Init(bus *core.Bus) { n.bus = bus }

func (n *WozduhNode) Update() {
	if n.bus == nil || n.bus.Cur == nil {
		return
	}
	close := n.bus.Cur.Get(core.SlotPriceClose)
	volume := n.bus.Cur.Get(core.SlotVolume)
	volPrice := n.volVwap.Update(close, volume)
	rsi11 := n.volRsi.Update(volPrice)
	wt11 := n.wt11Ema.Update(rsi11)
	wt22 := n.wt22Ema.Update(rsi11)
	n.bus.Cur.Set(core.SlotWozduhFast, wt11)
	n.bus.Cur.Set(core.SlotWozduhSlow, wt22)
}

func (n *WozduhNode) SaveState() {
	if n.volVwap != nil {
		n.volVwap.SaveState()
	}
	if n.volRsi != nil {
		n.volRsi.SaveState()
	}
	if n.wt11Ema != nil {
		n.wt11Ema.SaveState()
	}
	if n.wt22Ema != nil {
		n.wt22Ema.SaveState()
	}
}

func (n *WozduhNode) RestoreState() {
	if n.volVwap != nil {
		n.volVwap.RestoreState()
	}
	if n.volRsi != nil {
		n.volRsi.RestoreState()
	}
	if n.wt11Ema != nil {
		n.wt11Ema.RestoreState()
	}
	if n.wt22Ema != nil {
		n.wt22Ema.RestoreState()
	}
}

func (n *WozduhNode) OnConfigChange(any) error { return nil }

// Wt11Value exposes the wt11 EMA state (shadow validation / tests).
func (n *WozduhNode) Wt11Value() float64 {
	if n.wt11Ema == nil {
		return 0
	}
	return n.wt11Ema.Value()
}
