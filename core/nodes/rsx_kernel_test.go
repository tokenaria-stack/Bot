package nodes_test

import (
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/core/nodes"
)

func tickRSX(n *nodes.RSXNode, bus *core.Bus, high, low, close float64) (jurik, sig float64) {
	bus.Cur.Set(core.SlotPriceHigh, high)
	bus.Cur.Set(core.SlotPriceLow, low)
	bus.Cur.Set(core.SlotPriceClose, close)
	n.Update()
	return bus.Cur.Get(core.SlotJurikRSX), bus.Cur.Get(core.SlotJurikSignal)
}

func TestRSXNode_LengthChangesTip(t *testing.T) {
	t.Parallel()
	fastBus, slowBus := core.NewBus(64), core.NewBus(64)
	fast := nodes.NewRSXNode(7, 9, "hlc3")
	slow := nodes.NewRSXNode(21, 9, "hlc3")
	fast.Init(fastBus)
	slow.Init(slowBus)
	var vFast, vSlow float64
	for i := 0; i < 50; i++ {
		p := 100 + math.Sin(float64(i)*0.35)*5
		vFast, _ = tickRSX(fast, fastBus, p+1, p+2, p+0.5)
		vSlow, _ = tickRSX(slow, slowBus, p+1, p+2, p+0.5)
	}
	if math.Abs(vFast-vSlow) < 0.005 {
		t.Fatalf("expected different RSX with length 7 vs 21, fast=%f slow=%f", vFast, vSlow)
	}
}

func TestRSXNode_SourceCloseVsHLC3(t *testing.T) {
	t.Parallel()
	closeBus, hlc3Bus := core.NewBus(64), core.NewBus(64)
	closeN := nodes.NewRSXNode(14, 9, "close")
	hlc3N := nodes.NewRSXNode(14, 9, "hlc3")
	closeN.Init(closeBus)
	hlc3N.Init(hlc3Bus)
	var vClose, vHlc3 float64
	for i := 0; i < 80; i++ {
		h := 120.0 + float64(i%5)
		l := 100.0
		c := 118.0 + float64(i%3)
		vClose, _ = tickRSX(closeN, closeBus, h, l, c)
		vHlc3, _ = tickRSX(hlc3N, hlc3Bus, h, l, c)
	}
	if vClose <= 0 || vHlc3 <= 0 {
		t.Fatalf("RSX not warmed up: close=%f hlc3=%f", vClose, vHlc3)
	}
	if math.Abs(vClose-vHlc3) < 0.01 {
		t.Fatalf("close vs hlc3 RSX should differ, got %f vs %f", vClose, vHlc3)
	}
}

func TestRSXNode_SignalReconfigureInRange(t *testing.T) {
	t.Parallel()
	bus := core.NewBus(64)
	n := nodes.NewRSXNode(14, 9, "hlc3")
	n.Init(bus)
	for i := 0; i < 20; i++ {
		p := 100 + float64(i)
		tickRSX(n, bus, p+1, p, p+0.5)
	}
	if err := n.OnConfigChange(nodes.RSXNodeConfig{Length: 14, SignalLength: 3, Source: "hlc3"}); err != nil {
		t.Fatal(err)
	}
	_, sig := tickRSX(n, bus, 131, 130, 130.5)
	if sig <= 0 || sig > 100 {
		t.Fatalf("signal line out of range: %f", sig)
	}
}
