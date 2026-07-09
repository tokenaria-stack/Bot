package core

import "math"

const maxSwingEvents = 16

// SwingEvent is a confirmed ZigZag structural pivot (bar-indexed for zero look-ahead).
type SwingEvent struct {
	BarIndex int
	IsHigh   bool
	Price    float64
}

// EventRing is a fixed-capacity ring of swing events with O(1) snapshot/restore.
type EventRing struct {
	buf   [maxSwingEvents]SwingEvent
	head  int
	count int

	snapBuf   [maxSwingEvents]SwingEvent
	snapHead  int
	snapCount int
}

// NewEventRing allocates an empty event ring.
func NewEventRing() *EventRing {
	return &EventRing{}
}

// Push appends a swing event (overwrites oldest when full).
func (r *EventRing) Push(e SwingEvent) {
	if r == nil {
		return
	}
	r.buf[r.head] = e
	r.head = (r.head + 1) % maxSwingEvents
	if r.count < maxSwingEvents {
		r.count++
	}
}

// GetLast returns up to n most recent events (index 0 = newest).
func (r *EventRing) GetLast(n int) []SwingEvent {
	if r == nil || n <= 0 || r.count == 0 {
		return nil
	}
	if n > r.count {
		n = r.count
	}
	out := make([]SwingEvent, n)
	for i := 0; i < n; i++ {
		idx := (r.head - 1 - i) & (maxSwingEvents - 1)
		out[i] = r.buf[idx]
	}
	return out
}

// SaveState stores ring head/count/buffer at the last closed bar boundary.
func (r *EventRing) SaveState() {
	if r == nil {
		return
	}
	r.snapHead = r.head
	r.snapCount = r.count
	r.snapBuf = r.buf
}

// RestoreState rolls back open-bar event mutations to the last SaveState snapshot.
func (r *EventRing) RestoreState() {
	if r == nil {
		return
	}
	r.head = r.snapHead
	r.count = r.snapCount
	r.buf = r.snapBuf
}

// DivState enum values stored in SlotDivState.
const (
	DivStateNone float64 = 0
	DivStateS    float64 = -1
	DivStateSS   float64 = -2
	DivStateL    float64 = 1
	DivStateLL   float64 = 2
)

// HistValueAtBar returns the oscillator value at barIndex using history lookback.
// barsAgo = currentBarIndex - barIndex; uses Cur for the current (uncommitted) bar.
// Returns NaN when lookback exceeds ring memory.
func HistValueAtBar(bus *Bus, slot Slot, currentBarIndex, eventBarIndex int) float64 {
	if bus == nil || bus.Cur == nil {
		return math.NaN()
	}
	barsAgo := currentBarIndex - eventBarIndex
	if barsAgo <= 0 {
		return bus.Cur.Get(slot)
	}
	if bus.Hist == nil {
		return math.NaN()
	}
	if barsAgo >= bus.Hist.Cap() || barsAgo > bus.Hist.Count() {
		return math.NaN()
	}
	return bus.Hist.Get(slot, barsAgo)
}
