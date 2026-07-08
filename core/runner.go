package core

// DAGRunner executes nodes in registration order (explicit topological sort).
type DAGRunner struct {
	bus   *Bus
	nodes []Node
}

// NewDAGRunner binds a bus for tick orchestration.
func NewDAGRunner(bus *Bus) *DAGRunner {
	return &DAGRunner{bus: bus}
}

// AddNode appends a node and initializes it against the runner bus.
// Call order defines execution topology.
func (r *DAGRunner) AddNode(n Node) {
	if r == nil || n == nil {
		return
	}
	n.Init(r.bus)
	r.nodes = append(r.nodes, n)
}

// Bus returns the runner data bus.
func (r *DAGRunner) Bus() *Bus {
	if r == nil {
		return nil
	}
	return r.bus
}

// TickUpdate runs the Jeweler tick protocol: Restore → OHLCV → Update → Save+Hist (if closed).
func (r *DAGRunner) TickUpdate(priceOpen, priceHigh, priceLow, priceClose, volume float64, isClosed bool) {
	if r == nil || r.bus == nil || r.bus.Cur == nil {
		return
	}

	for _, n := range r.nodes {
		n.RestoreState()
	}

	cur := r.bus.Cur
	cur.Set(SlotPriceOpen, priceOpen)
	cur.Set(SlotPriceHigh, priceHigh)
	cur.Set(SlotPriceLow, priceLow)
	cur.Set(SlotPriceClose, priceClose)
	cur.Set(SlotVolume, volume)

	for _, n := range r.nodes {
		n.Update()
	}

	if !isClosed {
		return
	}

	for _, n := range r.nodes {
		n.SaveState()
	}

	if r.bus.Hist != nil {
		r.bus.Hist.PushFrame(cur)
		r.bus.Hist.Advance()
	}
}
