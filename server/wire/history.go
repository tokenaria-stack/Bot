package wire

import (
	"math"
	"strings"

	"trading_bot/core"
)

// HistoryAbsent is the columnar sentinel for missing / non-finite slot values.
// JSON cannot encode NaN; clients must skip this value during LWC hydration.
const HistoryAbsent = math.MaxFloat64

// BuildHistoryColumns projects committed HistoryBus values into columnar plot arrays.
// times is oldest-first; len(times) may exceed hist.Count() — leading bars receive HistoryAbsent.
func (p *Projector) BuildHistoryColumns(hist *core.HistoryBus, times []int64) map[string]any {
	plots, sentinel := p.BuildHistoryColumnsFiltered(hist, times, nil)
	return map[string]any{
		"times":    append([]int64(nil), times...),
		"plots":    plots,
		"sentinel": sentinel,
	}
}

// BuildHistoryColumnsFiltered projects only the requested component IDs into plot columns.
// When slotIDs is empty, all scalar manifest components are serialized.
func (p *Projector) BuildHistoryColumnsFiltered(hist *core.HistoryBus, times []int64, slotIDs []string) (map[string][]float64, float64) {
	n := len(times)
	if n == 0 {
		return map[string][]float64{}, HistoryAbsent
	}
	if p == nil || p.registry == nil {
		return map[string][]float64{}, HistoryAbsent
	}

	components := p.filterScalarComponents(slotIDs)
	plots := make(map[string][]float64, len(components))
	for _, c := range components {
		plots[c.ID] = make([]float64, n)
	}

	histCount := 0
	if hist != nil {
		histCount = hist.Count()
	}

	for i := 0; i < n; i++ {
		lookback := n - i
		for _, c := range components {
			col := plots[c.ID]
			if lookback > histCount || hist == nil {
				col[i] = HistoryAbsent
				continue
			}
			val := hist.Get(c.Slot, lookback)
			if !jsonSafeFloat(val) {
				col[i] = HistoryAbsent
			} else {
				col[i] = val
			}
		}
	}

	return plots, HistoryAbsent
}

func (p *Projector) filterScalarComponents(slotIDs []string) []core.UIComponent {
	all := p.scalarComponents()
	if len(slotIDs) == 0 {
		return all
	}
	allowed := make(map[string]struct{}, len(slotIDs))
	for _, id := range slotIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		allowed[id] = struct{}{}
	}
	if len(allowed) == 0 {
		return all
	}
	out := make([]core.UIComponent, 0, len(allowed))
	for _, c := range all {
		if _, ok := allowed[c.ID]; ok {
			out = append(out, c)
		}
	}
	return out
}

func (p *Projector) scalarComponents() []core.UIComponent {
	all := p.registry.Components()
	out := make([]core.UIComponent, 0, len(all))
	for _, c := range all {
		if c.DataMode == "scalar" {
			out = append(out, c)
		}
	}
	return out
}
