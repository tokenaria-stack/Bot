package core

import (
	"math"
	"testing"
)

func TestValidateHistoryCap(t *testing.T) {
	if got := ValidateHistoryCap(0); got != minHistoryCap {
		t.Fatalf("expected min %d, got %d", minHistoryCap, got)
	}
	if got := ValidateHistoryCap(100); got != 128 {
		t.Fatalf("expected 128, got %d", got)
	}
	if got := ValidateHistoryCap(128); got != 128 {
		t.Fatalf("expected 128, got %d", got)
	}
}

func TestHistoryBusRing(t *testing.T) {
	h := NewHistoryBus(4)
	frame := &TickFrame{}
	frame.Set(SlotPriceClose, 100)
	h.PushFrame(frame)
	h.Advance()
	frame.Set(SlotPriceClose, 200)
	h.PushFrame(frame)
	h.Advance()

	if v := h.Get(SlotPriceClose, 1); v != 200 {
		t.Fatalf("lookback 1 want 200, got %v", v)
	}
	if v := h.Get(SlotPriceClose, 2); v != 100 {
		t.Fatalf("lookback 2 want 100, got %v", v)
	}
	if v := h.Get(SlotPriceClose, 99); !math.IsNaN(v) {
		t.Fatalf("out of range want NaN, got %v", v)
	}
}

type stubNode struct {
	name     string
	restored int
	updated  int
	saved    int
}

func (s *stubNode) Name() string             { return s.name }
func (s *stubNode) Init(*Bus)                {}
func (s *stubNode) Update()                  { s.updated++ }
func (s *stubNode) SaveState()               { s.saved++ }
func (s *stubNode) RestoreState()            { s.restored++ }
func (s *stubNode) OnConfigChange(any) error { return nil }

func TestDAGRunnerOpenBarNoHist(t *testing.T) {
	bus := NewBus(64)
	r := NewDAGRunner(bus)
	n := &stubNode{name: "stub"}
	r.AddNode(n)

	r.TickUpdate(1, 2, 0.5, 1.5, 10, 0, false)
	if n.restored != 1 || n.updated != 1 || n.saved != 0 {
		t.Fatalf("open bar: restored=%d updated=%d saved=%d", n.restored, n.updated, n.saved)
	}
	if !math.IsNaN(bus.Hist.Get(SlotPriceClose, 1)) {
		t.Fatalf("open bar should not commit history")
	}

	r.TickUpdate(1, 2, 0.5, 1.6, 11, 1, true)
	if n.saved != 1 {
		t.Fatalf("closed bar: saved=%d", n.saved)
	}
	if bus.Hist.Get(SlotPriceClose, 1) != 1.6 {
		t.Fatalf("hist close want 1.6, got %v", bus.Hist.Get(SlotPriceClose, 1))
	}
}
