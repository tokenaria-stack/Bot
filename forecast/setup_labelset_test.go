package forecast

import (
	"strings"
	"testing"
	"time"
)

func TestFrozenSetupTarget1(t *testing.T) {
	t.Parallel()
	s := FrozenSetupTarget1()
	if err := s.validate(); err != nil {
		t.Fatal(err)
	}
	if s.Side != GeomSideLong || s.SuccessR != 2 || s.Horizon != 72 {
		t.Fatalf("%+v", s)
	}
}

func TestLabelSetupTarget1_MatchesMovePotential2R(t *testing.T) {
	t.Parallel()
	rep := StructuralStopReport{
		WickOwner: StopOwnerPriceK2,
		Assignments: []StructuralStopAssignment{
			{Status: StructuralStopStatusValid, Side: GeomSideLong, At: 1, Hit2RBeforeStop: true, Plus2R: 110},
			{Status: StructuralStopStatusValid, Side: GeomSideLong, At: 2, Hit1RBeforeStop: true},
			{Status: StructuralStopStatusValid, Side: GeomSideLong, At: 3, StopHit: true, StopHitAt: 30},
			{Status: StructuralStopStatusValid, Side: GeomSideLong, At: 4},
			{Status: StructuralStopStatusValid, Side: GeomSideLong, At: 5, IncompleteH: true, Hit2RBeforeStop: true},
			{Status: StructuralStopStatusValid, Side: GeomSideLong, At: 6, PathAmbiguous: true},
			{Status: StructuralStopStatusInvalid, Side: GeomSideLong, At: 7},
			{Status: StructuralStopStatusNone, Side: GeomSideLong, At: 8},
			{Status: StructuralStopStatusValid, Side: GeomSideShort, At: 9, Hit2RBeforeStop: true},
		},
	}
	ls, err := LabelSetupTarget1(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !ls.MatchOK || len(ls.Rows) != 8 {
		t.Fatalf("LONG only: %+v n=%d", ls, len(ls.Rows))
	}
	c := countSetupLabels(ls.Rows)
	if c.TP != 1 || c.Stop != 1 || c.Timeout != 2 || c.Skip != 2 || c.Invalid != 2 {
		t.Fatalf("%+v", c)
	}
	txt := ls.Text
	if !strings.Contains(txt, "MOVE-POTENTIAL-1 LONG +2R MATCH") {
		t.Fatal(txt)
	}
	if !strings.Contains(txt, "SHORT is not this target") {
		t.Fatal(txt)
	}
	a, err := LabelSetupTarget1(rep)
	if err != nil || a.Text != ls.Text {
		t.Fatal("determinism")
	}
}

func TestLabelSetupTarget1_RefusesWrongWickOwner(t *testing.T) {
	t.Parallel()
	_, err := LabelSetupTarget1(StructuralStopReport{WickOwner: StopOwnerRSXFractal})
	if err == nil {
		t.Fatal("must refuse STOP-1 fractal owner")
	}
}

func TestFormatSetupYearTable(t *testing.T) {
	t.Parallel()
	y2019 := time.Date(2019, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	y2020 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	txt := formatSetupYearTable([]SetupLabelRow{
		{At: y2019, Class: SetupClassTP},
		{At: y2019, Class: SetupClassStop},
		{At: y2020, Class: SetupClassTimeout},
	})
	if !strings.Contains(txt, "2019") || !strings.Contains(txt, "2020") {
		t.Fatal(txt)
	}
}
