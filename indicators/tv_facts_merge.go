package indicators

import "sort"

// MergeTVFacts concatenates TV divergence and pivot facts in a stable order
// (ConfirmedAt, AnchorAt, Source, Direction) so replay and live compare equal.
func MergeTVFacts(divs, pivots []IndicatorFactEvent) []IndicatorFactEvent {
	n := len(divs) + len(pivots)
	if n == 0 {
		return nil
	}
	out := make([]IndicatorFactEvent, 0, n)
	out = append(out, divs...)
	out = append(out, pivots...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ConfirmedAt != out[j].ConfirmedAt {
			return out[i].ConfirmedAt < out[j].ConfirmedAt
		}
		if out[i].AnchorAt != out[j].AnchorAt {
			return out[i].AnchorAt < out[j].AnchorAt
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Direction < out[j].Direction
	})
	return out
}
