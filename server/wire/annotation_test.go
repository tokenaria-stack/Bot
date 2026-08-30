package wire

import (
	"testing"

	"trading_bot/core"
	"trading_bot/indicators"
	"trading_bot/ui_config"
)

func TestAnnotationFromFact_BullBearOnAnchor(t *testing.T) {
	t.Parallel()
	bull := indicators.IndicatorFactEvent{
		Source:      indicators.FactSourceRSXTVDiv,
		Direction:   indicators.FactDirBullish,
		ConfirmedAt: 1_700_000_060_000,
		AnchorAt:    1_700_000_000_000,
		AnchorValue: 28,
		AnchorPrice: 100,
	}
	ann, ok := AnnotationFromFact(bull, "rsx")
	if !ok {
		t.Fatal("bull fact must project")
	}
	if ann.Time != 1_700_000_000 || ann.Pane != "rsx" || ann.Label != "" {
		t.Fatalf("bull ann=%+v", ann)
	}
	if ann.Position != "belowBar" || ann.Shape != "arrowUp" || ann.Color != rsxDivBullColor {
		t.Fatalf("bull style=%+v", ann)
	}
	if ann.Source != indicators.FactSourceRSXTVDiv {
		t.Fatalf("source %q", ann.Source)
	}

	bear := bull
	bear.Direction = indicators.FactDirBearish
	ann, ok = AnnotationFromFact(bear, "rsx")
	if !ok || ann.Label != "" || ann.Position != "aboveBar" || ann.Shape != "arrowDown" || ann.Color != rsxDivBearColor {
		t.Fatalf("bear ann=%+v ok=%v", ann, ok)
	}
}

func TestAnnotationFromFact_TVPivotArrows(t *testing.T) {
	t.Parallel()
	high := indicators.IndicatorFactEvent{
		Source:      indicators.FactSourceRSXTVPivot,
		Direction:   indicators.FactDirPivotHigh,
		ConfirmedAt: 1_700_000_120_000,
		AnchorAt:    1_700_000_000_000,
	}
	ann, ok := AnnotationFromFact(high, "rsx")
	if !ok || ann.Label != "" || ann.Shape != "arrowDown" || ann.Position != "aboveBar" || ann.Color != rsxPivotColor {
		t.Fatalf("pivot high=%+v ok=%v", ann, ok)
	}
	low := high
	low.Direction = indicators.FactDirPivotLow
	ann, ok = AnnotationFromFact(low, "rsx")
	if !ok || ann.Label != "" || ann.Shape != "arrowUp" || ann.Position != "belowBar" || ann.Color != rsxPivotColor {
		t.Fatalf("pivot low=%+v ok=%v", ann, ok)
	}
}

func TestAnnotationFromFact_RejectsLegacyAndZigZag(t *testing.T) {
	t.Parallel()
	if _, ok := AnnotationFromFact(indicators.IndicatorFactEvent{Source: "rsx_zz_div", Direction: indicators.FactDirBullish, AnchorAt: 1}, "rsx"); ok {
		t.Fatal("zz source must not use TV projector")
	}
	if _, ok := AnnotationFromDivState(1700000000, core.DivStateL, "rsx"); ok {
		t.Fatal("ZigZag DivState must not emit chart markers")
	}
}

func TestAnnotationsFromFacts_FiltersToTimes(t *testing.T) {
	t.Parallel()
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	p := NewProjector(reg)
	ev := indicators.IndicatorFactEvent{
		Source:      indicators.FactSourceRSXTVDiv,
		Direction:   indicators.FactDirBullish,
		ConfirmedAt: 1_700_000_060_000,
		AnchorAt:    1_700_000_000_000,
	}
	anns := p.AnnotationsFromFacts([]indicators.IndicatorFactEvent{ev}, []int64{1_700_000_000})
	if len(anns) != 1 || anns[0].Label != "" || anns[0].Shape != "arrowUp" {
		t.Fatalf("got %+v", anns)
	}
	empty := p.AnnotationsFromFacts([]indicators.IndicatorFactEvent{ev}, []int64{99})
	if len(empty) != 0 {
		t.Fatalf("out of window: %+v", empty)
	}
}

func TestBuildHistoryAnnotations_DivStateUnpublished(t *testing.T) {
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
