package market

import (
	"testing"
)

func TestExportAllAnnotations_NoRSXTradingLabels(t *testing.T) {
	t.Parallel()

	klines := makeSyntheticKlines(80)
	m := NewFrame(klines, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	m.mu.Lock()
	m.Annotations = append(m.Annotations, ChartAnnotation{
		Time:     klines[21].OpenTime / 1000,
		Pane:     "rsx",
		Label:    "L",
		Color:    "#26a69a",
		Position: "belowBar",
		Shape:    "arrowUp",
	})
	m.mu.Unlock()

	export := m.ExportAllAnnotations(0, len(klines)-1)
	for _, ann := range export {
		if ann.Pane == "rsx" && (ann.Label == "L" || ann.Label == "LL" || ann.Label == "S" || ann.Label == "SS") {
			t.Fatalf("Phase F: RSX trading label leaked: %+v", ann)
		}
	}
}
