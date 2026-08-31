package wire

import (
	"encoding/json"
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/ui_config"
)

func TestProjectorOmitsNonFiniteValues(t *testing.T) {
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	p := NewProjector(reg)
	frame := &core.TickFrame{}
	frame.Set(core.SlotJurikRSX, math.NaN())
	frame.Set(core.SlotJurikSignal, 42)

	plots := p.BuildTickJSON(frame)
	if _, ok := plots["line_rsx"]; ok {
		t.Fatal("expected NaN slot to be omitted")
	}
	if plots["line_rsx_signal"] != 42 {
		t.Fatalf("expected signal 42, got %v", plots["line_rsx_signal"])
	}

	_, err = json.Marshal(plots)
	if err != nil {
		t.Fatalf("marshal plots: %v", err)
	}
}

func TestBuildTickJSONFiltered_OnlyRequested(t *testing.T) {
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	p := NewProjector(reg)
	frame := &core.TickFrame{}
	frame.Set(core.SlotJurikRSX, 55)
	frame.Set(core.SlotJurikSignal, 44)
	frame.Set(core.SlotWozduhFast, 11)
	frame.Set(core.SlotWozduhSlow, 22)

	all := p.BuildTickJSON(frame)
	if _, ok := all["woz_fast"]; !ok {
		t.Fatal("unfiltered tick must include woz_fast")
	}
	filtered := p.BuildTickJSONFiltered(frame, []string{"line_rsx", "woz_slow"})
	if _, ok := filtered["woz_fast"]; ok {
		t.Fatal("hidden woz_fast must be omitted from filtered tick")
	}
	if filtered["line_rsx"] != 55 || filtered["woz_slow"] != 22 {
		t.Fatalf("filtered=%v", filtered)
	}
	if _, ok := filtered["woz_vol_chan"]; ok {
		t.Fatal("compose id must not be packed as a scalar")
	}
	if FilterPlotMap(all, nil)["woz_fast"] != 11 {
		t.Fatal("empty filter must keep plots")
	}
	for id, v := range filtered {
		if all[id] != v {
			t.Fatalf("parity %s: filtered=%v unfiltered=%v", id, v, all[id])
		}
	}
}

func TestWozduhScalarCount_Layout(t *testing.T) {
	n := 0
	for _, c := range ui_config.WozduhComponents() {
		if c.DataMode == "scalar" {
			n++
		}
	}
	if n != 15 {
		t.Fatalf("wozduh scalar plot ids = %d, want 15 (9 lines + 6 channel sources)", n)
	}
}
