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

func TestAnnotationFromFact_ZigZagHiddenLabel(t *testing.T) {
	t.Parallel()
	ev := indicators.IndicatorFactEvent{
		Source:      indicators.FactSourceRSXZZDiv,
		Direction:   indicators.FactDirBullish,
		Pattern:     indicators.FactPatternHidden,
		ConfirmedAt: 1_700_000_120_000,
		AnchorAt:    1_700_000_000_000,
	}
	ann, ok := AnnotationFromFact(ev, "rsx")
	if !ok || ann.Label != "H Bull" || ann.Shape != "arrowUp" || ann.Source != indicators.FactSourceRSXZZDiv {
		t.Fatalf("hidden bull=%+v ok=%v", ann, ok)
	}
	ev.Pattern = indicators.FactPatternRegular
	ann, ok = AnnotationFromFact(ev, "rsx")
	if !ok || ann.Label != "" || ann.Color != rsxDivBullColor {
		t.Fatalf("regular zz=%+v ok=%v", ann, ok)
	}
	ev.Direction = indicators.FactDirBearish
	ev.Pattern = indicators.FactPatternHidden
	ann, ok = AnnotationFromFact(ev, "rsx")
	if !ok || ann.Label != "H Bear" || ann.Shape != "arrowDown" {
		t.Fatalf("hidden bear=%+v ok=%v", ann, ok)
	}
}

func TestAnnotationFromFact_RejectsUnknown(t *testing.T) {
	t.Parallel()
	if _, ok := AnnotationFromFact(indicators.IndicatorFactEvent{Source: "rsx_fractal_div", Direction: indicators.FactDirBullish, AnchorAt: 1}, "rsx"); ok {
		t.Fatal("fractal source must not project")
	}
	if _, ok := AnnotationFromFact(indicators.IndicatorFactEvent{
		Source:    indicators.FactSourceRSXZZDiv,
		Direction: indicators.FactDirBullish,
		Pattern:   indicators.FactPatternRegular,
		AnchorAt:  1,
	}, "rsx"); !ok {
		t.Fatal("rsx_zz_div must project")
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

func TestAnnotationsFromFacts_RSXPaneNotDivStateSlot(t *testing.T) {
	t.Parallel()
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	var comp core.UIComponent
	found := false
	for _, c := range reg.Components() {
		if c.ID == "ann_rsx_div" {
			comp = c
			found = true
			break
		}
	}
	if !found || comp.Slot != core.SlotJurikRSX || comp.DataMode != "annotations" || comp.HostID != "rsx" {
		t.Fatalf("ann_rsx_div ownership %+v found=%v", comp, found)
	}
	p := NewProjector(reg)
	frame := &core.TickFrame{}
	frame.Set(core.SlotJurikRSX, 55)
	plots := p.BuildTickJSON(frame)
	if _, has := plots["ann_rsx_div"]; has {
		t.Fatal("annotation component must not pack Jurik RSX as a scalar plot")
	}
	if plots["line_rsx"] != 55 {
		t.Fatalf("line_rsx plot %+v", plots)
	}
	hist := core.NewHistoryBus(4)
	hist.PushFrame(frame)
	hist.Advance()
	cols := p.BuildHistoryColumns(hist, []int64{1})
	histPlots, _ := cols["plots"].(map[string][]float64)
	if _, has := histPlots["ann_rsx_div"]; has {
		t.Fatal("history columns must not pack ann_rsx_div from Jurik RSX")
	}
	ev := indicators.IndicatorFactEvent{
		Source:      indicators.FactSourceRSXZZDiv,
		Direction:   indicators.FactDirBullish,
		Pattern:     indicators.FactPatternHidden,
		ConfirmedAt: 1_700_000_060_000,
		AnchorAt:    1_700_000_000_000,
	}
	anns := p.AnnotationsFromFacts([]indicators.IndicatorFactEvent{ev}, []int64{1_700_000_000})
	if len(anns) != 1 || anns[0].Pane != "rsx" || anns[0].Label != "H Bull" {
		t.Fatalf("zz fact pane %+v", anns)
	}
}
