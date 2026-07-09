package nodes

import (
	"fmt"
	"strings"

	"trading_bot/core"
	"trading_bot/indicators"
)

// RSXNode computes Jurik RSX and its signal line into the data bus.
type RSXNode struct {
	bus    *core.Bus
	rsx    *indicators.JurikRSX
	signal *indicators.RSXSignalLine
	source string
}

// NewRSXNode creates an RSX pipeline node with explicit length, signal, and price source.
func NewRSXNode(length, signalLength int, source string) *RSXNode {
	return &RSXNode{
		rsx:    indicators.NewJurikRSX(length),
		signal: indicators.NewRSXSignalLine(signalLength),
		source: normalizeRSXSource(source),
	}
}

func (n *RSXNode) Name() string { return "rsx" }

func (n *RSXNode) Init(bus *core.Bus) { n.bus = bus }

func (n *RSXNode) Update() {
	if n.bus == nil || n.bus.Cur == nil || n.rsx == nil || n.signal == nil {
		return
	}
	high := n.bus.Cur.Get(core.SlotPriceHigh)
	low := n.bus.Cur.Get(core.SlotPriceLow)
	close := n.bus.Cur.Get(core.SlotPriceClose)
	price := rsxSourcePrice(high, low, close, n.source)
	jurik := n.rsx.Update(price)
	n.bus.Cur.Set(core.SlotJurikRSX, jurik)
	n.bus.Cur.Set(core.SlotJurikSignal, n.signal.Update(jurik))
}

func (n *RSXNode) SaveState() {
	if n.rsx != nil {
		n.rsx.SaveState()
	}
	if n.signal != nil {
		n.signal.SaveState()
	}
}

func (n *RSXNode) RestoreState() {
	if n.rsx != nil {
		n.rsx.RestoreState()
	}
	if n.signal != nil {
		n.signal.RestoreState()
	}
}

// OnConfigChange reconfigures Jurik length, signal SMA, and price source.
func (n *RSXNode) OnConfigChange(cfg any) error {
	c, ok := cfg.(RSXNodeConfig)
	if !ok {
		return fmt.Errorf("rsx: expected RSXNodeConfig, got %T", cfg)
	}
	if n.rsx != nil && c.Length > 0 {
		n.rsx.Reconfigure(c.Length)
	}
	if n.signal != nil && c.SignalLength > 0 {
		n.signal.Reconfigure(c.SignalLength)
	}
	if strings.TrimSpace(c.Source) != "" {
		n.source = normalizeRSXSource(c.Source)
	}
	return nil
}

// JurikValue exposes the current Jurik RSX output (shadow validation / tests).
func (n *RSXNode) JurikValue() float64 {
	if n.rsx == nil {
		return 0
	}
	return n.rsx.Value()
}

func normalizeRSXSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "close":
		return "close"
	default:
		return "hlc3"
	}
}

func rsxSourcePrice(high, low, close float64, source string) float64 {
	if normalizeRSXSource(source) == "close" {
		return close
	}
	return (high + low + close) / 3.0
}
