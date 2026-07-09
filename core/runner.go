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

// NodeByName returns the first node with the given Name(), or nil.
func (r *DAGRunner) NodeByName(name string) Node {
	if r == nil {
		return nil
	}
	for _, n := range r.nodes {
		if n.Name() == name {
			return n
		}
	}
	return nil
}

// OnConfigChange forwards cfg to the named node.
func (r *DAGRunner) OnConfigChange(name string, cfg any) error {
	n := r.NodeByName(name)
	if n == nil {
		return nil
	}
	return n.OnConfigChange(cfg)
}

// TickUpdate runs the Jeweler tick protocol: Restore → OHLCV → Update → Save+Hist (if closed).
func (r *DAGRunner) TickUpdate(priceOpen, priceHigh, priceLow, priceClose, volume float64, barIndex int, isClosed bool) {
	if r == nil || r.bus == nil || r.bus.Cur == nil {
		return
	}

	if r.bus.Events != nil {
		r.bus.Events.RestoreState()
	}
	for _, n := range r.nodes {
		n.RestoreState()
	}

	cur := r.bus.Cur
	cur.BarIndex = barIndex
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
	if r.bus.Events != nil {
		r.bus.Events.SaveState()
	}

	if r.bus.Hist != nil {
		r.bus.Hist.PushFrame(cur)
		r.bus.Hist.Advance()
	}
}
