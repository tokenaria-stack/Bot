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
