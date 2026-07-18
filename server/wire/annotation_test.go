package wire

import (
	"testing"

	"trading_bot/core"
	"trading_bot/ui_config"
)

func TestAnnotationFromDivState_PhaseFPurged(t *testing.T) {
	t.Parallel()
	if _, ok := AnnotationFromDivState(1700000000, core.DivStateL, "rsx"); ok {
		t.Fatal("Phase F: DivState must not emit chart markers")
	}
	if _, ok := AnnotationFromDivState(1700000000, core.DivStateS, "rsx"); ok {
		t.Fatal("Phase F: DivState must not emit chart markers")
	}
	if _, ok := AnnotationFromDivState(1, core.DivStateNone, "rsx"); ok {
		t.Fatal("None must not emit")
	}
}

func TestBuildHistoryAnnotations_PhaseFEmpty(t *testing.T) {
	t.Parallel()
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	p := NewProjector(reg)

	hist := core.NewHistoryBus(8)
	states := []float64{
		core.DivStateNone,
		core.DivStateL,
		core.DivStateS,
		core.DivStateLL,
	}
	for _, st := range states {
		frame := &core.TickFrame{}
		frame.Set(core.SlotDivState, st)
		hist.PushFrame(frame)
		hist.Advance()
	}
	times := []int64{100, 200, 300, 400}
	anns := p.BuildHistoryAnnotations(hist, times)
	if len(anns) != 0 {
		t.Fatalf("Phase F: want 0 annotations, got %+v", anns)
	}
}

func TestBuildTickAnnotation_PhaseFEmpty(t *testing.T) {
	t.Parallel()
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	p := NewProjector(reg)
	frame := &core.TickFrame{}
	frame.Set(core.SlotDivState, core.DivStateLL)
	if _, ok := p.BuildTickAnnotation(frame, 42); ok {
		t.Fatal("Phase F: tick annotation must not emit")
	}
}

func TestDivStateLabel_StillMapsEnums(t *testing.T) {
	t.Parallel()
	if DivStateLabel(core.DivStateL) != "L" || DivStateLabel(core.DivStateSS) != "SS" {
		t.Fatalf("label map broken: L=%q SS=%q", DivStateLabel(core.DivStateL), DivStateLabel(core.DivStateSS))
	}
}
