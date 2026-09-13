package forecast

import (
	"fmt"
	"sort"

	"trading_bot/indicators"
)

// PivotDisagreement cases for STRUCTURAL-STOP-1 visual misses.
const (
	PivotCaseALaterR2LostToConfirmRank = "A_later_r2_usable_but_not_selected"
	PivotCaseBLaterR2Unconfirmed       = "B_later_r2_unconfirmed_at_entry"
	PivotCaseCLaterR6Only              = "C_later_r6_usable_no_later_r2"
	PivotCaseDNoLaterFractalFact       = "D_no_later_rsx_fractal_in_window"
)

// PivotSighting is one causal pivot in the selected-anchor → ENTRY window.
type PivotSighting struct {
	Family      string
	Radius      int
	AnchorAt    int64
	ConfirmedAt int64
	Usable      bool
	LaterAnchor bool
	Wick        float64
	ConfirmLag  int64
}

// PivotDisagreementSample is one TOO_TIGHT / WRONG_PIVOT chart.
type PivotDisagreementSample struct {
	Ordinal     int
	Assignment  StructuralStopAssignment
	Selected    PivotSighting
	R2          []PivotSighting
	R6          []PivotSighting
	TV          []PivotSighting
	Verdict     string
	Explanation string
}

func wickAt(openIdx map[int64]int, bars []CanonicalClosedBar, at int64, long bool) float64 {
	i, ok := openIdx[at]
	if !ok || i < 0 || i >= len(bars) {
		return 0
	}
	if long {
		return bars[i].Low
	}
	return bars[i].High
}

func collectWindowPivots(facts []indicators.IndicatorFactEvent, source, dir string, radius int, selectedAnchor, entry int64, openIdx map[int64]int, bars []CanonicalClosedBar, long bool) []PivotSighting {
	var out []PivotSighting
	seen := map[[2]int64]bool{}
	for _, ev := range facts {
		if ev.Source != source || ev.Direction != dir {
			continue
		}
		if ev.AnchorAt <= selectedAnchor || ev.AnchorAt > entry {
			continue
		}
		key := [2]int64{ev.AnchorAt, ev.ConfirmedAt}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, PivotSighting{
			Family:      source,
			Radius:      radius,
			AnchorAt:    ev.AnchorAt,
			ConfirmedAt: ev.ConfirmedAt,
			Usable:      ev.ConfirmedAt > 0 && ev.ConfirmedAt <= entry,
			LaterAnchor: true,
			Wick:        wickAt(openIdx, bars, ev.AnchorAt, long),
			ConfirmLag:  ev.ConfirmedAt - ev.AnchorAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AnchorAt != out[j].AnchorAt {
			return out[i].AnchorAt < out[j].AnchorAt
		}
		return out[i].ConfirmedAt < out[j].ConfirmedAt
	})
	return out
}

func anyUsable(rows []PivotSighting) bool {
	for _, r := range rows {
		if r.Usable {
			return true
		}
	}
	return false
}

func anyUnconfirmed(rows []PivotSighting) bool {
	for _, r := range rows {
		if !r.Usable {
			return true
		}
	}
	return false
}

func classifyPivotDisagreement(r2, r6, tv []PivotSighting) (verdict, why string) {
	switch {
	case anyUsable(r2):
		return PivotCaseALaterR2LostToConfirmRank,
			"research r=2 has a later usable LOW in the window; STOP-1 ranks max ConfirmedAt, not nearest AnchorAt"
	case len(r2) > 0 && anyUnconfirmed(r2):
		return PivotCaseBLaterR2Unconfirmed,
			"later research r=2 exists but ConfirmedAt is after ENTRY (hindsight on the chart)"
	case anyUsable(r6):
		return PivotCaseCLaterR6Only,
			"no later usable research r=2; a later radius-6 fractal is confirmed at ENTRY"
	case len(r6) > 0 && anyUnconfirmed(r6):
		return PivotCaseCLaterR6Only,
			"no later research r=2; a later radius-6 fractal exists but is unconfirmed at ENTRY"
	case len(tv) > 0:
		return PivotCaseDNoLaterFractalFact,
			"no later rsx_fractal_pivot r=2 or r=6 in the window; TV pivot(s) exist (different family)"
	default:
		return PivotCaseDNoLaterFractalFact,
			"no later rsx_fractal_pivot r=2/r=6 and no TV pivot in the window — likely a price swing only"
	}
}

// AuditPivotDisagreement enumerates later pivots between STOP-1 AnchorAt and ENTRY.
func AuditPivotDisagreement(
	sample StructuralStopAssignment,
	ordinal int,
	bars []CanonicalClosedBar,
	r2, r6, tv []indicators.IndicatorFactEvent,
) PivotDisagreementSample {
	openIdx := map[int64]int{}
	for i, b := range bars {
		openIdx[b.OpenTime] = i
	}
	long := sample.Side != GeomSideShort
	dir := indicators.FactDirPivotLow
	if !long {
		dir = indicators.FactDirPivotHigh
	}
	out := PivotDisagreementSample{Ordinal: ordinal, Assignment: sample}
	out.Selected = PivotSighting{
		Family:      indicators.FactSourceRSXFractalPivot,
		Radius:      2,
		AnchorAt:    sample.PivotAnchorAt,
		ConfirmedAt: sample.PivotConfirmedAt,
		Usable:      true,
		Wick:        sample.Stop,
		ConfirmLag:  sample.PivotConfirmedAt - sample.PivotAnchorAt,
	}
	if sample.PivotAnchorAt <= 0 {
		out.Verdict = PivotCaseDNoLaterFractalFact
		out.Explanation = "STOP-1 had no selected pivot"
		return out
	}
	out.R2 = collectWindowPivots(r2, indicators.FactSourceRSXFractalPivot, dir, 2, sample.PivotAnchorAt, sample.At, openIdx, bars, long)
	out.R6 = collectWindowPivots(r6, indicators.FactSourceRSXFractalPivot, dir, 6, sample.PivotAnchorAt, sample.At, openIdx, bars, long)
	out.TV = collectWindowPivots(tv, indicators.FactSourceRSXTVPivot, dir, 0, sample.PivotAnchorAt, sample.At, openIdx, bars, long)
	out.Verdict, out.Explanation = classifyPivotDisagreement(out.R2, out.R6, out.TV)
	return out
}

// FormatPivotDisagreementAudit is forensic text. No PnL, no target freeze.
func FormatPivotDisagreementAudit(rows []PivotDisagreementSample) string {
	s := "PIVOT-DISAGREEMENT-AUDIT (TOO_TIGHT / WRONG_PIVOT only; no STOP-2)\n\n"
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Verdict]++
		s += fmt.Sprintf("LONG %d/30  At=%d  selected AnchorAt=%d ConfirmedAt=%d wick=%.4f lag_ms=%d\n",
			r.Ordinal, r.Assignment.At, r.Selected.AnchorAt, r.Selected.ConfirmedAt, r.Selected.Wick, r.Selected.ConfirmLag)
		s += fmt.Sprintf("  verdict=%s\n  %s\n", r.Verdict, r.Explanation)
		dump := func(title string, xs []PivotSighting) {
			s += fmt.Sprintf("  %s n=%d\n", title, len(xs))
			for _, x := range xs {
				s += fmt.Sprintf("    AnchorAt=%d ConfirmedAt=%d usable=%v wick=%.4f lag_ms=%d\n",
					x.AnchorAt, x.ConfirmedAt, x.Usable, x.Wick, x.ConfirmLag)
			}
		}
		dump("research r=2 later", r.R2)
		dump("visual r=6 later", r.R6)
		dump("tv pivot later", r.TV)
		s += "\n"
	}
	s += fmt.Sprintf("counts A=%d B=%d C=%d D=%d n=%d\n",
		counts[PivotCaseALaterR2LostToConfirmRank],
		counts[PivotCaseBLaterR2Unconfirmed],
		counts[PivotCaseCLaterR6Only],
		counts[PivotCaseDNoLaterFractalFact],
		len(rows))
	return s
}
