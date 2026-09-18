package core

import "testing"

func TestSlotLayout_CountAndIndependence(t *testing.T) {
	t.Parallel()
	if SlotAO != SlotWozduhVolRsiEma5+1 {
		t.Fatalf("SlotAO=%d want SlotWozduhVolRsiEma5+1=%d", SlotAO, SlotWozduhVolRsiEma5+1)
	}
	if SlotCount != SlotWozduhRsiCloseChanDn+1 {
		t.Fatalf("SlotCount=%d want last live+1=%d", SlotCount, SlotWozduhRsiCloseChanDn+1)
	}

	frame := &TickFrame{}
	frame.Set(SlotPriceOpen, 1)
	frame.Set(SlotWozduhRsiCloseChanDn, 2)
	if frame.Get(SlotPriceOpen) != 1 || frame.Get(SlotWozduhRsiCloseChanDn) != 2 {
		t.Fatal("first/last live slots must be independent")
	}
	if frame.Get(SlotAO) != 0 {
		t.Fatal("untouched middle slot must stay 0")
	}

	h := NewHistoryBus(4)
	if len(h.data) != int(SlotCount)*h.Cap() {
		t.Fatalf("hist stripes %d want SlotCount*cap=%d", len(h.data), int(SlotCount)*h.Cap())
	}
	frame.Set(SlotPriceOpen, 10)
	frame.Set(SlotWozduhRsiCloseChanDn, 20)
	h.PushFrame(frame)
	h.Advance()
	if h.Get(SlotPriceOpen, 1) != 10 || h.Get(SlotWozduhRsiCloseChanDn, 1) != 20 {
		t.Fatal("hist first/last live slots must be independent")
	}
}
