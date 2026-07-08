package core

// TickFrame holds scalar slot values for the current tick (zero-alloc hot path).
type TickFrame struct {
	Values [SlotCount]float64
}

// Get returns the value at slot s.
func (f *TickFrame) Get(s Slot) float64 {
	return f.Values[s]
}

// Set writes val into slot s.
func (f *TickFrame) Set(s Slot, val float64) {
	f.Values[s] = val
}

// Bus is the shared data plane for DAG nodes on the current tick.
type Bus struct {
	Cur  *TickFrame
	Hist *HistoryBus
}

// NewBus allocates the tick frame and history ring.
func NewBus(historyCap int) *Bus {
	return &Bus{
		Cur:  &TickFrame{},
		Hist: NewHistoryBus(historyCap),
	}
}
