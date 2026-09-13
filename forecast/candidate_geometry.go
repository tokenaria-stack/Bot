package forecast

import (
	"fmt"
	"math"
	"sort"

	"trading_bot/data"
	"trading_bot/indicators"
)

// CANDIDATE-GEOMETRY-1 local measurement matrix. Not a TargetSpec, not identity.
const (
	CandidateGeometryHorizon     = 72
	CandidateGeometryR           = 2.0
	candidateGeometryClusterBars = 4 // 4×15m = 1h; predeclared, not tuned
)

var candidateGeometryK = [...]float64{1, 2}

const (
	GeomFamily50Cross     = "rsx_50_cross"
	GeomFamilySignalCross = "rsx_signal_cross"
	GeomFamilyTV          = "tv_div"
	GeomFamilyZoneExit    = "rsx_zone_exit"
	GeomSideLong          = "LONG"
	GeomSideShort         = "SHORT"
	GeomScaleATR15        = "atr14_15m"
	GeomScaleATR1h        = "atr14_1h"
)

// CandidateGeometryInput is archive truth already owned elsewhere.
type CandidateGeometryInput struct {
	PrimaryTF   string
	FinerTF     string
	HTF1hTF     string
	FinerMarket MarketKey
	Primary     []CanonicalClosedBar
	Finer       []CanonicalClosedBar
	HTF1h       []CanonicalClosedBar
	Tape        []FeatureRow2
}

// GeomPopulation is one atom × side ignition set.
type GeomPopulation struct {
	Family, Side string
	N            int
	FirstAt      int64
	LastAt       int64
	MedianGap    float64
	P25Gap       float64
	P75Gap       float64
	AdjacentN    int
	ClusterN     int
	SameBarDup   int
}

// GeomCell is one descriptive ticket geometry. No ranking.
type GeomCell struct {
	Family, Side, Scale string
	K                   float64
	UsableN             int
	SkipNoVol           int
	SkipIncomplete      int
	TPFirst             int
	SLFirst             int
	Timeout             int
	Ambiguous           int
	Reasons             map[string]int
	MFE                 []float64
	MAE                 []float64
	TFav1               []float64
	TFav2               []float64
	TAdv1               []float64
	TAdv2               []float64
	HitFav1             int
	HitFav2             int
	HitAdv1             int
	HitAdv2             int
	ExcursionN          int
}

// CandidateGeometryReport is the chapter output. No winner field exists.
type CandidateGeometryReport struct {
	Populations []GeomPopulation
	Cells       []GeomCell
	ATR15Usable int
	ATR15Skip   int
	ATR1hUsable int
	ATR1hSkip   int
	Notes       []string
}

type geomEvent struct {
	At   int64
	Idx  int
	Side string
	Fam  string
}

func spec2Col(id FeatureID) int {
	ids := FeatureSpec2IDs()
	for i, x := range ids {
		if x == id {
			return i
		}
	}
	return -1
}

func ignitionAge0(row FeatureRow2, presentID, ageID FeatureID) bool {
	if row.Ready != IsReady {
		return false
	}
	ip, ia := spec2Col(presentID), spec2Col(ageID)
	if ip < 0 || ia < 0 {
		return false
	}
	return row.Values[ip] == 1 && row.Values[ia] == 0
}

func zoneExitLong(prev, cur FeatureRow2) bool {
	if prev.Ready != IsReady || cur.Ready != IsReady {
		return false
	}
	i := spec2Col(FeatureRSXValue)
	return prev.Values[i] <= 30 && cur.Values[i] > 30
}

func zoneExitShort(prev, cur FeatureRow2) bool {
	if prev.Ready != IsReady || cur.Ready != IsReady {
		return false
	}
	i := spec2Col(FeatureRSXValue)
	return prev.Values[i] >= 70 && cur.Values[i] < 70
}

func collectIgnitions(tape []FeatureRow2, primaryTF string) ([]geomEvent, error) {
	var out []geomEvent
	add := func(at int64, fam, side string) {
		out = append(out, geomEvent{At: at, Fam: fam, Side: side})
	}
	for _, row := range tape {
		if ignitionAge0(row, FeatureRSX50CrossUpPresent, FeatureRSX50CrossUpAge) {
			add(row.At, GeomFamily50Cross, GeomSideLong)
		}
		if ignitionAge0(row, FeatureRSX50CrossDownPresent, FeatureRSX50CrossDownAge) {
			add(row.At, GeomFamily50Cross, GeomSideShort)
		}
		if ignitionAge0(row, FeatureRSXSignalCrossUpPresent, FeatureRSXSignalCrossUpAge) {
			add(row.At, GeomFamilySignalCross, GeomSideLong)
		}
		if ignitionAge0(row, FeatureRSXSignalCrossDownPresent, FeatureRSXSignalCrossDownAge) {
			add(row.At, GeomFamilySignalCross, GeomSideShort)
		}
		if ignitionAge0(row, FeatureTVBullPresent, FeatureTVBullAge) {
			add(row.At, GeomFamilyTV, GeomSideLong)
		}
		if ignitionAge0(row, FeatureTVBearPresent, FeatureTVBearAge) {
			add(row.At, GeomFamilyTV, GeomSideShort)
		}
	}
	for i := 1; i < len(tape); i++ {
		prev, cur := tape[i-1], tape[i]
		next, err := data.NextBarOpen(prev.At, primaryTF)
		if err != nil {
			return nil, err
		}
		if cur.At != next {
			continue
		}
		if zoneExitLong(prev, cur) {
			add(cur.At, GeomFamilyZoneExit, GeomSideLong)
		}
		if zoneExitShort(prev, cur) {
			add(cur.At, GeomFamilyZoneExit, GeomSideShort)
		}
	}
	return out, nil
}

func atrOHLC(bars []CanonicalClosedBar) (high, low, close []float64) {
	high = make([]float64, len(bars))
	low = make([]float64, len(bars))
	close = make([]float64, len(bars))
	for i, b := range bars {
		high[i], low[i], close[i] = b.High, b.Low, b.Close
	}
	return high, low, close
}

func lastClosedHTFATR(h1 []CanonicalClosedBar, atr []float64, tf string, candCloseTime int64) (v float64, ok bool) {
	best := -1
	for i := range h1 {
		ct, err := data.BarCloseTimeMs(h1[i].OpenTime, tf)
		if err != nil {
			return 0, false
		}
		if ct > candCloseTime {
			break
		}
		best = i
	}
	if best < 0 {
		return 0, false
	}
	v = atr[best]
	if !isFinite(v) || v <= 0 {
		return 0, false
	}
	return v, true
}

func ticketBarriers(close, v, k float64, long bool) (upper, lower float64) {
	stop := k * v
	tp := CandidateGeometryR * stop
	if long {
		return close + tp, close - stop
	}
	return close + stop, close - tp
}

func classifyTicket(row LabelRow, long bool) (tp, sl, to, amb bool) {
	switch row.Outcome {
	case OutcomeTimeout:
		return false, false, true, false
	case OutcomeAmbiguous:
		return false, false, false, true
	case OutcomeUpFirst:
		if long {
			return true, false, false, false
		}
		return false, true, false, false
	case OutcomeDownFirst:
		if long {
			return false, true, false, false
		}
		return true, false, false, false
	default:
		return false, false, false, true
	}
}

func pathExcursion(bars []CanonicalClosedBar, t, h int, interval string, entry, v float64, long bool) (mfe, mae float64, tf1, tf2, ta1, ta2 float64, hitF1, hitF2, hitA1, hitA2 bool, complete bool, err error) {
	tf1, tf2, ta1, ta2 = math.NaN(), math.NaN(), math.NaN(), math.NaN()
	if v <= 0 || t < 0 || t >= len(bars) {
		return 0, 0, tf1, tf2, ta1, ta2, false, false, false, false, false, nil
	}
	prev := bars[t].OpenTime
	var maxFav, maxAdv float64
	for n := 0; n < h; n++ {
		expected, e := data.NextBarOpen(prev, interval)
		if e != nil {
			return 0, 0, tf1, tf2, ta1, ta2, false, false, false, false, false, e
		}
		k := t + 1 + n
		if k >= len(bars) {
			return 0, 0, tf1, tf2, ta1, ta2, false, false, false, false, false, nil
		}
		b := bars[k]
		if b.OpenTime != expected {
			return 0, 0, tf1, tf2, ta1, ta2, false, false, false, false, false, nil
		}
		var fav, adv float64
		if long {
			fav = (b.High - entry) / v
			adv = (entry - b.Low) / v
		} else {
			fav = (entry - b.Low) / v
			adv = (b.High - entry) / v
		}
		if fav > maxFav {
			maxFav = fav
		}
		if adv > maxAdv {
			maxAdv = adv
		}
		barN := float64(n + 1)
		if !hitF1 && fav >= 1 {
			hitF1, tf1 = true, barN
		}
		if !hitF2 && fav >= 2 {
			hitF2, tf2 = true, barN
		}
		if !hitA1 && adv >= 1 {
			hitA1, ta1 = true, barN
		}
		if !hitA2 && adv >= 2 {
			hitA2, ta2 = true, barN
		}
		prev = b.OpenTime
	}
	return maxFav, maxAdv, tf1, tf2, ta1, ta2, hitF1, hitF2, hitA1, hitA2, true, nil
}

func populateGaps(ats []int64, tf string) (median, p25, p75 float64, adjacent, cluster int) {
	if len(ats) < 2 {
		return math.NaN(), math.NaN(), math.NaN(), 0, 0
	}
	gaps := make([]float64, 0, len(ats)-1)
	for i := 1; i < len(ats); i++ {
		n := 0
		x := ats[i-1]
		for x < ats[i] {
			nx, err := data.NextBarOpen(x, tf)
			if err != nil || nx <= x {
				break
			}
			x = nx
			n++
			if n > 1_000_000 {
				break
			}
		}
		gaps = append(gaps, float64(n))
		if n == 1 {
			adjacent++
		}
		if n > 0 && n <= candidateGeometryClusterBars {
			cluster++
		}
	}
	sort.Float64s(gaps)
	return qtile(gaps, 0.50), qtile(gaps, 0.25), qtile(gaps, 0.75), adjacent, cluster
}

func qtile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return math.NaN()
	}
	if q <= 0 {
		return sorted[0]
	}
	if q >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := int(math.Ceil(q*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func summarizeTimes(xs []float64) (p50 float64, n int) {
	var v []float64
	for _, x := range xs {
		if !math.IsNaN(x) {
			v = append(v, x)
		}
	}
	sort.Float64s(v)
	return qtile(v, 0.5), len(v)
}

func summarizeExc(xs []float64) (p25, p50, p75, p90 float64) {
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	return qtile(cp, 0.25), qtile(cp, 0.50), qtile(cp, 0.75), qtile(cp, 0.90)
}

func families() []string {
	return []string{GeomFamily50Cross, GeomFamilySignalCross, GeomFamilyTV, GeomFamilyZoneExit}
}

func sides() []string { return []string{GeomSideLong, GeomSideShort} }

func scales() []string { return []string{GeomScaleATR15, GeomScaleATR1h} }

// RunCandidateGeometry1 is the descriptive study. It does not select a ticket.
func RunCandidateGeometry1(in CandidateGeometryInput) (CandidateGeometryReport, error) {
	var z CandidateGeometryReport
	if in.PrimaryTF == "" {
		in.PrimaryTF = Spec2PrimaryTF
	}
	if in.FinerTF == "" {
		in.FinerTF = "1m"
	}
	if in.HTF1hTF == "" {
		in.HTF1hTF = Spec2HTF1h
	}
	if err := validatePrimaryBars(in.Primary, in.PrimaryTF); err != nil {
		return z, err
	}
	if err := validateATRHistoryContinuity(in.Primary, in.PrimaryTF); err != nil {
		return z, err
	}
	if err := validatePrimaryBars(in.HTF1h, in.HTF1hTF); err != nil {
		return z, err
	}
	if err := validateATRHistoryContinuity(in.HTF1h, in.HTF1hTF); err != nil {
		return z, err
	}
	if in.FinerMarket.Timeframe == "" {
		in.FinerMarket.Timeframe = in.FinerTF
	}
	events, err := collectIgnitions(in.Tape, in.PrimaryTF)
	if err != nil {
		return z, err
	}
	ats := make([]int64, len(in.Tape))
	for i, r := range in.Tape {
		ats[i] = r.At
	}
	tapeIdx, err := joinCandidatesToPrimary(in.Primary, ats)
	if err != nil {
		return z, err
	}
	atToPrimary := map[int64]int{}
	for i, at := range ats {
		atToPrimary[at] = tapeIdx[i]
	}
	for i := range events {
		idx, ok := atToPrimary[events[i].At]
		if !ok {
			return z, fmt.Errorf("forecast: geometry event At %d missing from primary", events[i].At)
		}
		events[i].Idx = idx
	}

	spec := indicators.CanonicalATRSpec()
	h15, l15, c15 := atrOHLC(in.Primary)
	atr15, err := indicators.ATRSeries(spec, h15, l15, c15)
	if err != nil {
		return z, err
	}
	hh, ll, cc := atrOHLC(in.HTF1h)
	atr1h, err := indicators.ATRSeries(spec, hh, ll, cc)
	if err != nil {
		return z, err
	}

	finer := NewFinerBarrierResolver(in.FinerMarket, in.PrimaryTF, in.Finer)

	atr15At := atr15
	atr1hAt := make([]float64, len(in.Primary))
	ok1hAt := make([]bool, len(in.Primary))
	j := 0
	for i := range in.Primary {
		ct, err := data.BarCloseTimeMs(in.Primary[i].OpenTime, in.PrimaryTF)
		if err != nil {
			return z, err
		}
		for j+1 < len(in.HTF1h) {
			nct, err := data.BarCloseTimeMs(in.HTF1h[j+1].OpenTime, in.HTF1hTF)
			if err != nil {
				return z, err
			}
			if nct <= ct {
				j++
				continue
			}
			break
		}
		if j >= len(in.HTF1h) {
			continue
		}
		hct, err := data.BarCloseTimeMs(in.HTF1h[j].OpenTime, in.HTF1hTF)
		if err != nil {
			return z, err
		}
		if hct > ct {
			continue
		}
		v := atr1h[j]
		if isFinite(v) && v > 0 {
			atr1hAt[i] = v
			ok1hAt[i] = true
		}
	}

	byKey := map[string][]geomEvent{}
	for _, e := range events {
		k := e.Fam + "|" + e.Side
		byKey[k] = append(byKey[k], e)
	}
	for _, fam := range families() {
		for _, side := range sides() {
			evs := byKey[fam+"|"+side]
			p := GeomPopulation{Family: fam, Side: side, N: len(evs)}
			seen := map[int64]int{}
			pats := make([]int64, len(evs))
			for i, e := range evs {
				pats[i] = e.At
				seen[e.At]++
				if p.FirstAt == 0 || e.At < p.FirstAt {
					p.FirstAt = e.At
				}
				if e.At > p.LastAt {
					p.LastAt = e.At
				}
			}
			for _, c := range seen {
				if c > 1 {
					p.SameBarDup += c - 1
				}
			}
			p.MedianGap, p.P25Gap, p.P75Gap, p.AdjacentN, p.ClusterN = populateGaps(pats, in.PrimaryTF)
			z.Populations = append(z.Populations, p)
		}
	}
	for _, e := range events {
		v := atr15At[e.Idx]
		if isFinite(v) && v > 0 {
			z.ATR15Usable++
		} else {
			z.ATR15Skip++
		}
		if ok1hAt[e.Idx] {
			z.ATR1hUsable++
		} else {
			z.ATR1hSkip++
		}
	}

	for _, fam := range families() {
		for _, side := range sides() {
			evs := byKey[fam+"|"+side]
			long := side == GeomSideLong
			for _, scale := range scales() {
				for _, kMul := range candidateGeometryK {
					cell := GeomCell{
						Family: fam, Side: side, Scale: scale, K: kMul,
						Reasons: map[string]int{},
					}
					for _, e := range evs {
						b := in.Primary[e.Idx]
						var v float64
						ok := false
						switch scale {
						case GeomScaleATR15:
							v = atr15At[e.Idx]
							ok = isFinite(v) && v > 0
						case GeomScaleATR1h:
							v, ok = atr1hAt[e.Idx], ok1hAt[e.Idx]
						}
						if !ok {
							cell.SkipNoVol++
							continue
						}
						upper, lower := ticketBarriers(b.Close, v, kMul, long)
						row, err := EvaluateBarriers(in.Primary, e.Idx, upper, lower, CandidateGeometryHorizon, in.PrimaryTF, finer)
						if err != nil {
							return z, err
						}
						mfe, mae, tf1, tf2, ta1, ta2, hf1, hf2, ha1, ha2, complete, err := pathExcursion(
							in.Primary, e.Idx, CandidateGeometryHorizon, in.PrimaryTF, b.Close, v, long)
						if err != nil {
							return z, err
						}
						if !complete {
							cell.SkipIncomplete++
							cell.Reasons[string(row.Reason)]++
							if row.Outcome == OutcomeAmbiguous {
								cell.Ambiguous++
							}
							continue
						}
						cell.UsableN++
						tp, sl, to, amb := classifyTicket(row, long)
						switch {
						case tp:
							cell.TPFirst++
						case sl:
							cell.SLFirst++
						case to:
							cell.Timeout++
						default:
							cell.Ambiguous++
							_ = amb
						}
						cell.Reasons[string(row.Reason)]++
						cell.MFE = append(cell.MFE, mfe)
						cell.MAE = append(cell.MAE, mae)
						cell.ExcursionN++
						cell.TFav1 = append(cell.TFav1, tf1)
						cell.TFav2 = append(cell.TFav2, tf2)
						cell.TAdv1 = append(cell.TAdv1, ta1)
						cell.TAdv2 = append(cell.TAdv2, ta2)
						if hf1 {
							cell.HitFav1++
						}
						if hf2 {
							cell.HitFav2++
						}
						if ha1 {
							cell.HitAdv1++
						}
						if ha2 {
							cell.HitAdv2++
						}
					}
					z.Cells = append(z.Cells, cell)
				}
			}
		}
	}
	z.Notes = []string{
		"NO TARGET SELECTED",
		"q/D/CatBoost unused",
		"descriptive +2R/-1R/0 is not strategy EV",
		"1h ATR skip = no last-closed 1h or ATR<=0 at that bar; no stale fallback",
		"zone-exit requires adjacent Ready 15m bars (NextBarOpen); no hole interpolation",
		"MFE/MAE/time-to are in frozen-v units over full H; they do not depend on k",
	}
	return z, nil
}

func cellRate(n, d int) float64 {
	if d <= 0 {
		return math.NaN()
	}
	return float64(n) / float64(d)
}

// FormatCandidateGeometry1 is the chapter report. It never ranks cells.
func FormatCandidateGeometry1(r CandidateGeometryReport) string {
	var b []byte
	w := func(s string) { b = append(b, s...) }
	w("CANDIDATE-GEOMETRY-1 (descriptive; NO TARGET SELECTED)\n\n")
	w("POPULATIONS (ignition only: age==0 / zone-exit cross)\n")
	for _, p := range r.Populations {
		w(fmt.Sprintf("  %s %s N=%d first=%d last=%d median_gap_bars=%.1f p25=%.1f p75=%.1f adjacent=%d cluster<=%dbars=%d samebar_dup=%d\n",
			p.Family, p.Side, p.N, p.FirstAt, p.LastAt, p.MedianGap, p.P25Gap, p.P75Gap, p.AdjacentN, candidateGeometryClusterBars, p.ClusterN, p.SameBarDup))
	}
	w(fmt.Sprintf("\nVOL availability (once per ignition, not ×k):\n  ATR15 usable=%d skip=%d\n  ATR1h usable=%d skip=%d\n",
		r.ATR15Usable, r.ATR15Skip, r.ATR1hUsable, r.ATR1hSkip))
	w("\n32-CELL FIRST-PASSAGE (canonical EvaluateBarriers; 1m dual-hit)\n")
	w("family side scale k N_usable skip_vol skip_H TP SL TO AMB TP% SL% TO% AMB% descr_2R_units flags\n")
	for _, c := range r.Cells {
		n := c.UsableN
		tp, sl, to, amb := cellRate(c.TPFirst, n), cellRate(c.SLFirst, n), cellRate(c.Timeout, n), cellRate(c.Ambiguous, n)
		descr := 2*tp - 1*sl
		flag := degeneracyFlag(c)
		w(fmt.Sprintf("%s %s %s k=%.0f N=%d skipV=%d skipH=%d TP=%d SL=%d TO=%d AMB=%d TP%%=%.3f SL%%=%.3f TO%%=%.3f AMB%%=%.3f descr2R=%.3f %s\n",
			c.Family, c.Side, c.Scale, c.K, n, c.SkipNoVol, c.SkipIncomplete, c.TPFirst, c.SLFirst, c.Timeout, c.Ambiguous,
			tp, sl, to, amb, descr, flag))
	}
	w("\nMFE/MAE (full H window, v units; NOT ticket PnL)\n")
	for _, c := range r.Cells {
		p25, p50, p75, p90 := summarizeExc(c.MFE)
		a25, a50, a75, a90 := summarizeExc(c.MAE)
		w(fmt.Sprintf("%s %s %s k=%.0f n=%d MFE p25/50/75/90=%.3f/%.3f/%.3f/%.3f MAE p25/50/75/90=%.3f/%.3f/%.3f/%.3f\n",
			c.Family, c.Side, c.Scale, c.K, c.ExcursionN, p25, p50, p75, p90, a25, a50, a75, a90))
	}
	w("\nTIME-TO-EXCURSION (15m bars; independent first-touch)\n")
	for _, c := range r.Cells {
		f1, _ := summarizeTimes(c.TFav1)
		f2, _ := summarizeTimes(c.TFav2)
		a1, _ := summarizeTimes(c.TAdv1)
		a2, _ := summarizeTimes(c.TAdv2)
		den := c.UsableN
		w(fmt.Sprintf("%s %s %s k=%.0f +1v hit=%d (%.3f) med=%.1f +2v hit=%d (%.3f) med=%.1f -1v hit=%d (%.3f) med=%.1f -2v hit=%d (%.3f) med=%.1f\n",
			c.Family, c.Side, c.Scale, c.K,
			c.HitFav1, cellRate(c.HitFav1, den), f1,
			c.HitFav2, cellRate(c.HitFav2, den), f2,
			c.HitAdv1, cellRate(c.HitAdv1, den), a1,
			c.HitAdv2, cellRate(c.HitAdv2, den), a2))
	}
	w("\nNOTES\n")
	for _, n := range r.Notes {
		w("  - " + n + "\n")
	}
	return string(b)
}

func degeneracyFlag(c GeomCell) string {
	if c.UsableN == 0 {
		return "FLAG_EMPTY"
	}
	n := float64(c.UsableN)
	sl := float64(c.SLFirst) / n
	to := float64(c.Timeout) / n
	amb := float64(c.Ambiguous) / n
	var f string
	if c.UsableN < 30 {
		f += "FLAG_TINY_N "
	}
	if sl >= 0.90 {
		f += "FLAG_ALMOST_ALL_SL "
	}
	if to >= 0.90 {
		f += "FLAG_ALMOST_ALL_TIMEOUT "
	}
	if amb >= 0.20 {
		f += "FLAG_HIGH_AMBIG "
	}
	if f == "" {
		return "-"
	}
	return f
}
