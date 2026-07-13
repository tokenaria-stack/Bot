package wire

import (
	"testing"

	"trading_bot/core"
	"trading_bot/ui_config"
)

func TestAnnotationFromDivState(t *testing.T) {
	t.Parallel()
	ann, ok := AnnotationFromDivState(1700000000, core.DivStateL, "rsx")
	if !ok {
		t.Fatal("expected bullish annotation")
	}
	if ann.Label != "L" || ann.Shape != "arrowUp" || ann.Position != "belowBar" {
		t.Fatalf("got %+v", ann)
	}
	ann, ok = AnnotationFromDivState(1700000000, core.DivStateS, "rsx")
	if !ok || ann.Label != "S" || ann.Shape != "arrowDown" {
		t.Fatalf("bearish: ok=%v %+v", ok, ann)
	}
	if _, ok := AnnotationFromDivState(1, core.DivStateNone, "rsx"); ok {
		t.Fatal("None must not emit")
	}
}

func TestBuildHistoryAnnotations_risingEdge(t *testing.T) {
	t.Parallel()
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	p := NewProjector(reg)

	hist := core.NewHistoryBus(8)
	// Oldest → newest push order: None, None, L, L, None, S
	states := []float64{
		core.DivStateNone,
		core.DivStateNone,
		core.DivStateL,
		core.DivStateL,
		core.DivStateNone,
		core.DivStateS,
	}
	for _, st := range states {
		frame := &core.TickFrame{}
		frame.Set(core.SlotDivState, st)
		hist.PushFrame(frame)
		hist.Advance()
	}
	times := []int64{100, 200, 300, 400, 500, 600}
	anns := p.BuildHistoryAnnotations(hist, times)
	if len(anns) != 2 {
		t.Fatalf("len=%d want 2 (rising edges only), got %+v", len(anns), anns)
	}
	if anns[0].Time != 300 || anns[0].Label != "L" {
		t.Fatalf("first %+v", anns[0])
	}
	if anns[1].Time != 600 || anns[1].Label != "S" {
		t.Fatalf("second %+v", anns[1])
	}
}

func TestBuildTickAnnotation(t *testing.T) {
	t.Parallel()
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	p := NewProjector(reg)
	frame := &core.TickFrame{}
	frame.Set(core.SlotDivState, core.DivStateLL)
	ann, ok := p.BuildTickAnnotation(frame, 42)
	if !ok || ann.Label != "LL" || ann.Time != 42 {
		t.Fatalf("ok=%v %+v", ok, ann)
	}
}
