package wire

import (
	"encoding/json"
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/ui_config"
)

func TestBuildHistoryColumns_SentinelForMissingBars(t *testing.T) {
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	p := NewProjector(reg)

	h := core.NewHistoryBus(8)
	for i := 1; i <= 3; i++ {
		frame := &core.TickFrame{}
		frame.Set(core.SlotJurikRSX, float64(40+i))
		h.PushFrame(frame)
		h.Advance()
	}

	times := []int64{100, 200, 300, 400, 500}
	cols := p.BuildHistoryColumns(h, times)
	if cols["sentinel"] != HistoryAbsent {
		t.Fatalf("sentinel: got %v", cols["sentinel"])
	}
	plots, ok := cols["plots"].(map[string][]float64)
	if !ok {
		t.Fatal("expected plots map")
	}
	rsx := plots["line_rsx"]
	if len(rsx) != len(times) {
		t.Fatalf("column len %d want %d", len(rsx), len(times))
	}
	if rsx[0] != HistoryAbsent || rsx[1] != HistoryAbsent {
		t.Fatalf("leading bars should be sentinel: %v %v", rsx[0], rsx[1])
	}
	if rsx[4] != 43 {
		t.Fatalf("newest bar: got %v want 43", rsx[4])
	}

	_, err = json.Marshal(cols)
	if err != nil {
		t.Fatalf("marshal columns: %v", err)
	}
}

func TestBuildHistoryColumns_SentinelForNaN(t *testing.T) {
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	p := NewProjector(reg)
	h := core.NewHistoryBus(4)
	frame := &core.TickFrame{}
	frame.Set(core.SlotJurikRSX, math.NaN())
	h.PushFrame(frame)
	h.Advance()

	cols := p.BuildHistoryColumns(h, []int64{1})
	plots := cols["plots"].(map[string][]float64)
	if plots["line_rsx"][0] != HistoryAbsent {
		t.Fatalf("expected NaN mapped to sentinel, got %v", plots["line_rsx"][0])
	}
}
