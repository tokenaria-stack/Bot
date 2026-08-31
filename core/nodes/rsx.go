package nodes

import (
	"fmt"
	"math"
	"strings"

	"trading_bot/core"
	"trading_bot/indicators"
)

// RSXNode computes Jurik RSX and its signal line into the data bus.
type RSXNode struct {
	bus           *core.Bus
	rsx           *indicators.JurikRSX
	signal        *indicators.RSXSignalLine
	source        string
	active        bool
	streamUpdates int
}

// NewRSXNode creates an RSX pipeline node with explicit length, signal, and price source.
func NewRSXNode(length, signalLength int, source string) *RSXNode {
	return &RSXNode{
		rsx:    indicators.NewJurikRSX(length),
		signal: indicators.NewRSXSignalLine(signalLength),
		source: normalizeRSXSource(source),
		active: true,
	}
}

func (n *RSXNode) Name() string { return "rsx" }

func (n *RSXNode) Init(bus *core.Bus) { n.bus = bus }

func (n *RSXNode) Update() {
	if n.bus == nil || n.bus.Cur == nil || n.rsx == nil || n.signal == nil || !n.active {
		return
	}
	high := n.bus.Cur.Get(core.SlotPriceHigh)
	low := n.bus.Cur.Get(core.SlotPriceLow)
	close := n.bus.Cur.Get(core.SlotPriceClose)
	price := rsxSourcePrice(high, low, close, n.source)
	jurik := n.rsx.Update(price)
	n.streamUpdates++
	n.bus.Cur.Set(core.SlotJurikRSX, jurik)
	n.bus.Cur.Set(core.SlotJurikSignal, n.signal.Update(jurik))
	n.streamUpdates++
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
func (n *RSXNode) ApplyActive(on bool) {
	if n == nil {
		return
	}
	n.active = on
	if !on && n.bus != nil && n.bus.Cur != nil {
		n.bus.Cur.Set(core.SlotJurikRSX, math.NaN())
		n.bus.Cur.Set(core.SlotJurikSignal, math.NaN())
	}
}

func (n *RSXNode) Active() bool {
	return n != nil && n.active
}

func (n *RSXNode) StreamUpdates() int {
	if n == nil {
		return 0
	}
	return n.streamUpdates
}

func (n *RSXNode) JurikPtr() *indicators.JurikRSX {
	if n == nil {
		return nil
	}
	return n.rsx
}

func (n *RSXNode) InstallWoken(src *RSXNode) {
	if n == nil || src == nil {
		return
	}
	n.rsx = src.rsx
	n.signal = src.signal
}

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
