package wire

import (
	"math"

	"trading_bot/core"
)

// HistoryAbsent is the columnar sentinel for missing / non-finite slot values.
// JSON cannot encode NaN; clients must skip this value during LWC hydration.
const HistoryAbsent = math.MaxFloat64

// BuildHistoryColumns projects committed HistoryBus values into columnar plot arrays.
// times is oldest-first; len(times) may exceed hist.Count() — leading bars receive HistoryAbsent.
func (p *Projector) BuildHistoryColumns(hist *core.HistoryBus, times []int64) map[string]any {
	n := len(times)
	if n == 0 {
		return map[string]any{
			"times":    []int64{},
			"plots":    map[string][]float64{},
			"sentinel": HistoryAbsent,
		}
	}
	if p == nil || p.registry == nil {
		return map[string]any{
			"times":    append([]int64(nil), times...),
			"plots":    map[string][]float64{},
			"sentinel": HistoryAbsent,
		}
	}

	components := p.scalarComponents()
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

	return map[string]any{
		"times":    append([]int64(nil), times...),
		"plots":    plots,
		"sentinel": HistoryAbsent,
	}
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
