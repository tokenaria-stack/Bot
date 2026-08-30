package wire

import (
	"math"

	"trading_bot/core"
	"trading_bot/indicators"
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

// AnnotationsFromFacts projects factual events whose AnchorAt (as chart seconds)
// falls in times. Pane comes from the annotation component HostID, not Slot values.
func (p *Projector) AnnotationsFromFacts(events []indicators.IndicatorFactEvent, times []int64) []Annotation {
	if p == nil || p.registry == nil || len(times) == 0 {
		return []Annotation{}
	}
	comp, ok := p.firstAnnotationComponent()
	if !ok {
		return []Annotation{}
	}
	pane := annotationPane(comp)
	allowed := make(map[int64]struct{}, len(times))
	for _, t := range times {
		allowed[t] = struct{}{}
	}
	out := make([]Annotation, 0)
	for _, ev := range events {
		ann, ok := AnnotationFromFact(ev, pane)
		if !ok {
			continue
		}
		if _, in := allowed[ann.Time]; !in {
			continue
		}
		out = append(out, ann)
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
