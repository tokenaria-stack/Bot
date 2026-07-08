package wire

import (
	"trading_bot/core"
)

// Projector maps TickFrame slot values to wire keys using the UI registry (no indicator names).
type Projector struct {
	registry *core.UIRegistry
}

// NewProjector binds a UI registry for tick JSON projection.
func NewProjector(r *core.UIRegistry) *Projector {
	return &Projector{registry: r}
}

// BuildTickJSON projects scalar slot values into a map keyed by component ID.
func (p *Projector) BuildTickJSON(frame *core.TickFrame) map[string]float64 {
	if p == nil || p.registry == nil || frame == nil {
		return map[string]float64{}
	}
	components := p.registry.Components()
	out := make(map[string]float64, len(components))
	for _, c := range components {
		if c.DataMode != "scalar" {
			continue
		}
		out[c.ID] = frame.Get(c.Slot)
	}
	return out
}
