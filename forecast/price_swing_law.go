package forecast

import (
	"fmt"
	"strings"
)

// PriceSwingLawRow is one LONG sample: STOP-1 wick vs P1 (k=1) vs P2 (k=2).
type PriceSwingLawRow struct {
	Ordinal          int              `json:"ordinal"`
	Disputed         bool             `json:"disputed"`
	At               int64            `json:"at"`
	Entry            float64          `json:"entry"`
	Stop1AnchorAt    int64            `json:"stop1_anchor_at"`
	Stop1Wick        float64          `json:"stop1_wick"`
	LaterR2AnchorAt  int64            `json:"later_r2_anchor_at,omitempty"`
	LaterR2Wick      float64          `json:"later_r2_wick,omitempty"`
	LaterR2Usable    bool             `json:"later_r2_usable,omitempty"`
	P1               CausalPriceSwing `json:"p1"`
	P2               CausalPriceSwing `json:"p2"`
	P1MatchesLaterR2 bool             `json:"p1_matches_later_r2"`
	P2MatchesLaterR2 bool             `json:"p2_matches_later_r2"`
	P1LaterThanStop1 bool             `json:"p1_later_than_stop1"`
	P2LaterThanStop1 bool             `json:"p2_later_than_stop1"`
}

func matchAnchor(sw CausalPriceSwing, at int64) bool {
	return sw.Found && at > 0 && sw.AnchorAt == at
}

// EvaluatePriceSwingLaw compares causal OHLC swings to STOP-1 and any later RSX fact.
func EvaluatePriceSwingLaw(a StructuralStopAssignment, ordinal int, disputed bool, bars []CanonicalClosedBar, laterR2 PivotSighting) (PriceSwingLawRow, error) {
	row := PriceSwingLawRow{
		Ordinal: ordinal, Disputed: disputed, At: a.At, Entry: a.Entry,
		Stop1AnchorAt: a.PivotAnchorAt, Stop1Wick: a.Stop,
	}
	if laterR2.AnchorAt > 0 {
		row.LaterR2AnchorAt = laterR2.AnchorAt
		row.LaterR2Wick = laterR2.Wick
		row.LaterR2Usable = laterR2.Usable
	}
	p1, err := LatestCausalPriceSwing(bars, a.At, 1, a.Side != GeomSideShort)
	if err != nil {
		return row, err
	}
	p2, err := LatestCausalPriceSwing(bars, a.At, 2, a.Side != GeomSideShort)
	if err != nil {
		return row, err
	}
	row.P1, row.P2 = p1, p2
	row.P1MatchesLaterR2 = matchAnchor(p1, laterR2.AnchorAt)
	row.P2MatchesLaterR2 = matchAnchor(p2, laterR2.AnchorAt)
	row.P1LaterThanStop1 = p1.Found && p1.AnchorAt > a.PivotAnchorAt
	row.P2LaterThanStop1 = p2.Found && p2.AnchorAt > a.PivotAnchorAt
	return row, nil
}

// FormatPriceSwingLaw is forensic. No PnL. No k freeze.
func FormatPriceSwingLaw(rows []PriceSwingLawRow) string {
	var b strings.Builder
	b.WriteString("PRICE-SWING-LAW (P1 k=1 vs P2 k=2; causal OHLC; no ATR; no STOP freeze)\n\n")
	n1, n2, n1L, n2L := 0, 0, 0, 0
	nd := 0
	for _, r := range rows {
		if !r.Disputed {
			continue
		}
		nd++
		if r.P1MatchesLaterR2 {
			n1++
		}
		if r.P2MatchesLaterR2 {
			n2++
		}
		if r.P1LaterThanStop1 {
			n1L++
		}
		if r.P2LaterThanStop1 {
			n2L++
		}
		b.WriteString(fmt.Sprintf("LONG %d/30  entry=%.2f  STOP-1 AnchorAt=%d wick=%.2f\n",
			r.Ordinal, r.Entry, r.Stop1AnchorAt, r.Stop1Wick))
		if r.LaterR2AnchorAt != 0 {
			b.WriteString(fmt.Sprintf("  later RSX r2 AnchorAt=%d wick=%.2f usable=%v\n",
				r.LaterR2AnchorAt, r.LaterR2Wick, r.LaterR2Usable))
		} else {
			b.WriteString("  later RSX r2: none (D-class)\n")
		}
		fmtSwing := func(name string, s CausalPriceSwing, match, later bool) {
			if !s.Found {
				b.WriteString(fmt.Sprintf("  %s: NONE\n", name))
				return
			}
			b.WriteString(fmt.Sprintf("  %s: AnchorAt=%d ConfirmedAt=%d wick=%.2f geo_ok=%v match_later_RSX=%v later_than_STOP1=%v\n",
				name, s.AnchorAt, s.ConfirmedAt, s.Wick, !s.InvalidGeo, match, later))
		}
		fmtSwing("P1 k=1", r.P1, r.P1MatchesLaterR2, r.P1LaterThanStop1)
		fmtSwing("P2 k=2", r.P2, r.P2MatchesLaterR2, r.P2LaterThanStop1)
		b.WriteByte('\n')
	}
	b.WriteString(fmt.Sprintf("disputed n=%d  P1 match later-RSX=%d  P2 match later-RSX=%d  P1 later than STOP-1=%d  P2 later than STOP-1=%d\n",
		nd, n1, n2, n1L, n2L))
	b.WriteString("NO K SELECTED. Human: which wick is the pullback you meant?\n")
	return b.String()
}
