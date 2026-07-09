package wire

import (
	"math"

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
// Non-finite values (NaN, ±Inf) are omitted so encoding/json never panics on plots.
func (p *Projector) BuildTickJSON(frame *core.TickFrame) map[string]float64 {
	if p == nil || p.registry == nil || frame == nil {
		return nil
	}
	components := p.registry.Components()
	out := make(map[string]float64, len(components))
	for _, c := range components {
		if c.DataMode != "scalar" {
			continue
		}
		val := frame.Get(c.Slot)
		if !jsonSafeFloat(val) {
			continue
		}
		out[c.ID] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func jsonSafeFloat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
