package forecast

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

var errSetupTarget1Identity = fmt.Errorf("forecast: SETUP-TARGET-1 identity mismatch")

// SetupLabelRow is one LONG 50-cross under SETUP-TARGET-1. It is not LabelRow
// (UP_FIRST/DOWN_FIRST Target C).
type SetupLabelRow struct {
	At     int64   `json:"at"`
	Class  string  `json:"class"`
	Entry  float64 `json:"entry,omitempty"`
	Stop   float64 `json:"stop,omitempty"`
	Plus2R float64 `json:"plus_2r,omitempty"`
	HitAt  int64   `json:"hit_at,omitempty"`
	Status string  `json:"structure_status"`
}

// SetupLabelSet1 is the baseline label artifact. No dataset, no folds, no model.
type SetupLabelSet1 struct {
	Target      SetupTarget1    `json:"target"`
	Format      string          `json:"format"`
	Rows        []SetupLabelRow `json:"rows"`
	FirstAt     int64           `json:"first_at"`
	LastAt      int64           `json:"last_at"`
	Text        string          `json:"report"`
	MatchOK     bool            `json:"move_potential_2r_match"`
	MatchDetail string          `json:"move_potential_2r_match_detail,omitempty"`
}

type setupClassCounts struct {
	TP, Stop, Timeout, Invalid, Skip, All int
}

// LabelSetupTarget1 maps LONG STRUCTURAL-STOP-2 assignments onto SETUP-TARGET-1
// classes. Path truth is not recomputed; it is the same EvaluateBarriers result
// already stored on the assignment (+2R vs stop).
func LabelSetupTarget1(rep StructuralStopReport) (SetupLabelSet1, error) {
	tgt := FrozenSetupTarget1()
	if err := tgt.validate(); err != nil {
		return SetupLabelSet1{}, err
	}
	if rep.WickOwner != "" && rep.WickOwner != StopOwnerPriceK2 {
		return SetupLabelSet1{}, fmt.Errorf("forecast: SETUP-LABELSET-1 requires wick owner %s, got %s", StopOwnerPriceK2, rep.WickOwner)
	}
	var z SetupLabelSet1
	z.Target = tgt
	z.Format = SetupLabelSetFormat
	for _, a := range rep.Assignments {
		if a.Side != GeomSideLong {
			continue
		}
		row := SetupLabelRow{
			At: a.At, Entry: a.Entry, Stop: a.Stop, Plus2R: a.Plus2R,
			Status: a.Status,
		}
		if a.StopHit && a.StopHitAt > 0 && !a.Hit2RBeforeStop {
			row.HitAt = a.StopHitAt
		}
		row.Class = setupClassFromAssignment(a)
		z.Rows = append(z.Rows, row)
	}
	if len(z.Rows) > 0 {
		z.FirstAt = z.Rows[0].At
		z.LastAt = z.Rows[len(z.Rows)-1].At
	}
	if err := MatchMovePotentialLong2R(rep, z); err != nil {
		z.MatchOK = false
		z.MatchDetail = err.Error()
		return z, err
	}
	z.MatchOK = true
	z.MatchDetail = "MOVE-POTENTIAL-1 LONG +2R MATCH"
	z.Text = FormatSetupLabelSet1(z)
	return z, nil
}

func setupClassFromAssignment(a StructuralStopAssignment) string {
	switch a.Status {
	case StructuralStopStatusNone, StructuralStopStatusInvalid:
		return SetupClassInvalid
	case StructuralStopStatusValid:
		if a.IncompleteH || a.PathAmbiguous {
			return SetupClassSkip
		}
		if a.Hit2RBeforeStop {
			return SetupClassTP
		}
		if a.StopHit {
			return SetupClassStop
		}
		return SetupClassTimeout
	default:
		return SetupClassInvalid
	}
}

func countSetupLabels(rows []SetupLabelRow) setupClassCounts {
	var c setupClassCounts
	for _, r := range rows {
		c.All++
		switch r.Class {
		case SetupClassTP:
			c.TP++
		case SetupClassStop:
			c.Stop++
		case SetupClassTimeout:
			c.Timeout++
		case SetupClassInvalid:
			c.Invalid++
		case SetupClassSkip:
			c.Skip++
		}
	}
	return c
}

func countMovePotentialLong2R(as []StructuralStopAssignment) setupClassCounts {
	var c setupClassCounts
	for _, a := range as {
		if a.Side != GeomSideLong {
			continue
		}
		c.All++
		switch setupClassFromAssignment(a) {
		case SetupClassTP:
			c.TP++
		case SetupClassStop:
			c.Stop++
		case SetupClassTimeout:
			c.Timeout++
		case SetupClassInvalid:
			c.Invalid++
		case SetupClassSkip:
			c.Skip++
		}
	}
	return c
}

// MatchMovePotentialLong2R is the HARD STOP gate. Label classes must equal the
// MOVE-POTENTIAL-1 LONG +2R bucket counts on the same assignments.
func MatchMovePotentialLong2R(rep StructuralStopReport, labels SetupLabelSet1) error {
	want := countMovePotentialLong2R(rep.Assignments)
	got := countSetupLabels(labels.Rows)
	if want != got {
		return fmt.Errorf("forecast: SETUP-LABELSET-1 MOVE-POTENTIAL +2R mismatch want=%+v got=%+v", want, got)
	}
	return nil
}

// FormatSetupLabelSet1 is the chapter report. No EV, no model.
func FormatSetupLabelSet1(z SetupLabelSet1) string {
	c := countSetupLabels(z.Rows)
	usable := c.TP + c.Stop + c.Timeout
	var b strings.Builder
	b.WriteString("SETUP-TARGET-1 / SETUP-LABELSET-1 (LONG +2R; no CatBoost; not strategy EV)\n")
	b.WriteString(fmt.Sprintf("identity %s side=%s H=%d stop=%s buffer=%.2f×ATR15 success=+%.0fR\n",
		z.Target.ID, z.Target.Side, z.Target.Horizon, z.Target.StopOwner, z.Target.BufferATR, z.Target.SuccessR))
	b.WriteString(fmt.Sprintf("events=%d first_at=%d last_at=%d\n", c.All, z.FirstAt, z.LastAt))
	b.WriteString(fmt.Sprintf("  %s=%d (%.3f of usable)\n", SetupClassTP, c.TP, cellRate(c.TP, usable)))
	b.WriteString(fmt.Sprintf("  %s=%d (%.3f of usable)\n", SetupClassStop, c.Stop, cellRate(c.Stop, usable)))
	b.WriteString(fmt.Sprintf("  %s=%d (%.3f of usable)\n", SetupClassTimeout, c.Timeout, cellRate(c.Timeout, usable)))
	b.WriteString(fmt.Sprintf("  %s=%d\n", SetupClassInvalid, c.Invalid))
	b.WriteString(fmt.Sprintf("  %s=%d\n", SetupClassSkip, c.Skip))
	b.WriteString(fmt.Sprintf("usable=%d  resolved TP/(TP+STOP)=%.3f\n", usable, cellRate(c.TP, c.TP+c.Stop)))
	if z.MatchOK {
		b.WriteString("  " + z.MatchDetail + "\n")
	}
	b.WriteString("\ncalendar year UTC (all LONG events)\n")
	b.WriteString(formatSetupYearTable(z.Rows))
	b.WriteString("\nNOTES\n")
	b.WriteString("  - FeatureSpec2 join / ValidationPlan / CatBoost are later chapters\n")
	b.WriteString("  - SHORT is not this target\n")
	b.WriteString("  - overlapping H=72 events are kept; concurrency is later\n")
	b.WriteString("  - TIMEOUT is TIMEOUT_NEITHER, not truncated H (those are NOT_EVALUABLE)\n")
	return b.String()
}

func formatSetupYearTable(rows []SetupLabelRow) string {
	type yrow struct {
		TP, Stop, Timeout, Invalid, Skip, All int
	}
	by := map[int]*yrow{}
	var years []int
	for _, r := range rows {
		if r.At <= 0 {
			continue
		}
		y := time.UnixMilli(r.At).UTC().Year()
		if by[y] == nil {
			by[y] = &yrow{}
			years = append(years, y)
		}
		by[y].All++
		switch r.Class {
		case SetupClassTP:
			by[y].TP++
		case SetupClassStop:
			by[y].Stop++
		case SetupClassTimeout:
			by[y].Timeout++
		case SetupClassInvalid:
			by[y].Invalid++
		case SetupClassSkip:
			by[y].Skip++
		}
	}
	sort.Ints(years)
	var b strings.Builder
	b.WriteString("  year     n   TP_FIRST  STOP_FIRST  TIMEOUT  INVALID  NOT_EVAL\n")
	for _, y := range years {
		v := by[y]
		b.WriteString(fmt.Sprintf("  %d  %5d  %8d  %10d  %7d  %7d  %8d\n",
			y, v.All, v.TP, v.Stop, v.Timeout, v.Invalid, v.Skip))
	}
	return b.String()
}
