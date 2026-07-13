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

// BuildTickAnnotation projects SlotDivState on the current frame into at most one marker.
// Returns ok=false when DivState is idle or no annotations component is registered.
func (p *Projector) BuildTickAnnotation(frame *core.TickFrame, timeSec int64) (Annotation, bool) {
	if p == nil || p.registry == nil || frame == nil || timeSec <= 0 {
		return Annotation{}, false
	}
	comp, ok := p.firstAnnotationComponent()
	if !ok {
		return Annotation{}, false
	}
	return AnnotationFromDivState(timeSec, frame.Get(comp.Slot), annotationPane(comp))
}

// BuildHistoryAnnotations walks HistoryBus DivState and emits markers on rising edges
// (None → active or label change). DAG math stays in slots; this layer only packs visuals.
func (p *Projector) BuildHistoryAnnotations(hist *core.HistoryBus, times []int64) []Annotation {
	n := len(times)
	if n == 0 || p == nil || p.registry == nil {
		return []Annotation{}
	}
	comp, ok := p.firstAnnotationComponent()
	if !ok {
		return []Annotation{}
	}
	pane := annotationPane(comp)
	slot := comp.Slot

	histCount := 0
	if hist != nil {
		histCount = hist.Count()
	}

	out := make([]Annotation, 0, 16)
	var prev float64 = core.DivStateNone
	for i := 0; i < n; i++ {
		lookback := n - i
		state := core.DivStateNone
		if hist != nil && lookback <= histCount {
			state = hist.Get(slot, lookback)
			if !jsonSafeFloat(state) {
				state = core.DivStateNone
			}
		}
		if divStateActive(state) && state != prev {
			if ann, ok := AnnotationFromDivState(times[i], state, pane); ok {
				out = append(out, ann)
			}
		}
		prev = state
	}
	return out
}

func (p *Projector) firstAnnotationComponent() (core.UIComponent, bool) {
	if p == nil || p.registry == nil {
		return core.UIComponent{}, false
	}
	for _, c := range p.registry.Components() {
		if c.DataMode == "annotations" {
			return c, true
		}
	}
	return core.UIComponent{}, false
}

func annotationPane(c core.UIComponent) string {
	if c.HostID != "" {
		return c.HostID
	}
	return "rsx"
}

func jsonSafeFloat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
