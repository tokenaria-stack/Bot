package forecast

import (
	"fmt"
	"math"
	"strings"

	"trading_bot/indicators"
)

const StructuralSignificanceLastN = 4

// StructuralSwingView is one causal k=2 swing with diagnostic scalars.
// Prominence uses only High/Low on bars with OpenTime <= entry. No τ freeze.
type StructuralSwingView struct {
	ID            string  `json:"id"`
	K             int     `json:"k"`
	AnchorAt      int64   `json:"anchor_at"`
	ConfirmedAt   int64   `json:"confirmed_at"`
	Wick          float64 `json:"wick"`
	AgeBars       int     `json:"age_bars"`
	DistATR       float64 `json:"dist_atr"`
	ProminenceATR float64 `json:"prominence_atr"`
	FavorExtreme  float64 `json:"favor_extreme"`
	InvalidGeo    bool    `json:"invalid_geometry,omitempty"`
}

// StructuralSignificanceRow is one elicitation chart.
type StructuralSignificanceRow struct {
	Ordinal  int                   `json:"ordinal"`
	Disputed bool                  `json:"disputed"`
	Control  bool                  `json:"control"`
	At       int64                 `json:"at"`
	Entry    float64               `json:"entry"`
	ATR15    float64               `json:"atr15"`
	Swings   []StructuralSwingView `json:"swings"`
}

func prominenceToEntry(bars []CanonicalClosedBar, t, k, entry int, long bool, atr, entryPx float64) StructuralSwingView {
	sw := swingFromIndex(bars, t, k, entry, long)
	v := StructuralSwingView{
		K: k, AnchorAt: sw.AnchorAt, ConfirmedAt: sw.ConfirmedAt, Wick: sw.Wick,
		AgeBars: entry - t, InvalidGeo: sw.InvalidGeo,
	}
	if atr > 0 && isFinite(atr) {
		v.DistATR = math.Abs(entryPx-sw.Wick) / atr
	}
	lo, hi := t+k, entry
	ext := sw.Wick
	if long {
		for i := lo; i <= hi; i++ {
			if bars[i].High > ext {
				ext = bars[i].High
			}
		}
		v.FavorExtreme = ext
		if atr > 0 && isFinite(atr) {
			v.ProminenceATR = (ext - sw.Wick) / atr
		}
	} else {
		for i := lo; i <= hi; i++ {
			if bars[i].Low < ext {
				ext = bars[i].Low
			}
		}
		v.FavorExtreme = ext
		if atr > 0 && isFinite(atr) {
			v.ProminenceATR = (sw.Wick - ext) / atr
		}
	}
	return v
}

// LastCausalK2WithProminence returns the latest n k=2 swings at entry, S0 = newest.
func LastCausalK2WithProminence(bars []CanonicalClosedBar, entryAt int64, long bool, n int) ([]StructuralSwingView, float64, error) {
	if n <= 0 {
		n = StructuralSignificanceLastN
	}
	entry, err := entryIndex(bars, entryAt)
	if err != nil {
		return nil, 0, err
	}
	h, l, c := atrOHLC(bars)
	atr, err := indicators.ATRSeries(indicators.CanonicalATRSpec(), h, l, c)
	if err != nil {
		return nil, 0, err
	}
	atr15 := atr[entry]
	ts := causalSwingTimes(bars, entry, 2, long)
	if len(ts) > n {
		ts = ts[len(ts)-n:]
	}
	entryPx := bars[entry].Close
	out := make([]StructuralSwingView, 0, len(ts))
	for i := len(ts) - 1; i >= 0; i-- {
		v := prominenceToEntry(bars, ts[i], 2, entry, long, atr15, entryPx)
		v.ID = fmt.Sprintf("S%d", len(out))
		out = append(out, v)
	}
	return out, atr15, nil
}

// FormatStructuralSignificanceAudit is elicitation text. No τ, no PnL.
func FormatStructuralSignificanceAudit(rows []StructuralSignificanceRow) string {
	var b strings.Builder
	b.WriteString("STRUCTURAL-SIGNIFICANCE-AUDIT (k=2 P2 atom; last 4; NO τ; NO STOP freeze)\n")
	b.WriteString("Prominence = (max High from ConfirmedAt→ENTRY − Low) / ATR15(ENTRY). Causal.\n")
	b.WriteString("Say THIS ONE on each chart. A=latest  B=previous  C=prominence gap  D=impulse origin\n\n")
	for _, r := range rows {
		tag := "AGREE-control"
		if r.Disputed {
			tag = "DISPUTED"
		}
		b.WriteString(fmt.Sprintf("LONG %d/30  %s  entry=%.2f  ATR15=%.2f\n", r.Ordinal, tag, r.Entry, r.ATR15))
		for _, s := range r.Swings {
			b.WriteString(fmt.Sprintf("  %s  wick=%.2f  age=%d  R/ATR=%.2f  prominence=%.2f ATR  geo_ok=%v\n",
				s.ID, s.Wick, s.AgeBars, s.DistATR, s.ProminenceATR, !s.InvalidGeo))
		}
		b.WriteByte('\n')
	}
	b.WriteString("NO τ SELECTED. Finger: which Si is the invalidation wick?\n")
	return b.String()
}
