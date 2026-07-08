package core

import "math"

const minHistoryCap = 64

// ValidateHistoryCap rounds requested up to the next power of two (minimum minHistoryCap).
// Required for ring indexing via (head - lookback) & (cap - 1).
func ValidateHistoryCap(requested int) int {
	if requested < minHistoryCap {
		return minHistoryCap
	}
	if requested > 0 && requested&(requested-1) == 0 {
		return requested
	}
	cap := 1
	for cap < requested {
		cap <<= 1
	}
	return cap
}

// HistoryBus stores per-slot ring buffers in one flat slice (SlotCount * cap).
type HistoryBus struct {
	cap   int
	head  int
	count int
	data  []float64
}

// NewHistoryBus allocates a ring buffer with validated power-of-two capacity.
func NewHistoryBus(requestedCap int) *HistoryBus {
	cap := ValidateHistoryCap(requestedCap)
	return &HistoryBus{
		cap:  cap,
		data: make([]float64, int(SlotCount)*cap),
	}
}

// Cap returns the validated ring capacity.
func (h *HistoryBus) Cap() int {
	if h == nil {
		return 0
	}
	return h.cap
}

// Push writes val for slot at the current head index (call before Advance on bar close).
func (h *HistoryBus) Push(slot Slot, val float64) {
	if h == nil || slot >= SlotCount {
		return
	}
	base := int(slot) * h.cap
	h.data[base+h.head] = val
}

// Get returns the value at lookback bars relative to head.
// lookback 1 = most recently committed bar after Advance.
// Invalid lookback or insufficient history returns NaN.
func (h *HistoryBus) Get(slot Slot, lookback int) float64 {
	if h == nil || slot >= SlotCount {
		return math.NaN()
	}
	if lookback < 1 || lookback >= h.cap || lookback > h.count {
		return math.NaN()
	}
	idx := (h.head - lookback) & (h.cap - 1)
	base := int(slot) * h.cap
	return h.data[base+idx]
}

// Advance moves head forward after all slots are pushed on bar close.
func (h *HistoryBus) Advance() {
	if h == nil || h.cap == 0 {
		return
	}
	h.head = (h.head + 1) & (h.cap - 1)
	if h.count < h.cap {
		h.count++
	}
}

// PushFrame commits every slot from frame at the current head.
func (h *HistoryBus) PushFrame(frame *TickFrame) {
	if h == nil || frame == nil {
		return
	}
	for slot := Slot(0); slot < SlotCount; slot++ {
		h.Push(slot, frame.Get(slot))
	}
}
