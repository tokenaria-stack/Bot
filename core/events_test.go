package core

import "testing"

func TestEventRingSaveRestore(t *testing.T) {
	r := NewEventRing()
	r.Push(SwingEvent{BarIndex: 1, IsHigh: true, Price: 100})
	r.Push(SwingEvent{BarIndex: 2, IsHigh: false, Price: 90})
	r.SaveState()

	r.Push(SwingEvent{BarIndex: 3, IsHigh: true, Price: 200})
	if got := r.GetLast(1)[0].Price; got != 200 {
		t.Fatalf("expected open-bar push 200, got %v", got)
	}

	r.RestoreState()
	last := r.GetLast(1)
	if len(last) != 1 || last[0].Price != 90 {
		t.Fatalf("restore failed: got %+v want price 90", last)
	}
}

func TestHistValueAtBar(t *testing.T) {
	bus := NewBus(64)
	runner := NewDAGRunner(bus)

	for i := 1; i <= 5; i++ {
		p := float64(100 + i)
		bus.Cur.Set(SlotJurikRSX, p)
		runner.TickUpdate(p, p+1, p-1, p, 10, i, true)
	}

	v := HistValueAtBar(bus, SlotJurikRSX, 6, 3)
	if v != 103 {
		t.Fatalf("hist at bar 3 from bar 6: got %v want 103", v)
	}
}
