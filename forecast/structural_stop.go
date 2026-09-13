package forecast

import (
	"fmt"
	"math"
	"sort"

	"trading_bot/data"
	"trading_bot/indicators"
)

const (
	StructuralStopHorizon       = 72
	StructuralStopStatusValid   = "VALID_STRUCTURE"
	StructuralStopStatusNone    = "NO_STRUCTURE"
	StructuralStopStatusInvalid = "INVALID_GEOMETRY"
	StopOwnerRSXFractal         = "rsx_fractal"
	StopOwnerPriceK2            = "price_k2"
)

// StructuralStopInput is archive truth. Fractal facts must already be the
// frozen research series (analysis:v2 Jurik + default fractal radius), not live JSON.
type StructuralStopInput struct {
	PrimaryTF   string
	FinerTF     string
	HTF1hTF     string
	FinerMarket MarketKey
	Primary     []CanonicalClosedBar
	Finer       []CanonicalClosedBar
	HTF1h       []CanonicalClosedBar
	Tape        []FeatureRow2
	Fractals    []indicators.IndicatorFactEvent
	// StopOwner is "" / "rsx_fractal" (STOP-1) or "price_k2" (S0 wick freeze).
	StopOwner string
}

// StructuralStopAssignment is one 50-cross projection. Overlay must paint this
// object; it must not recompute RSX/fractals/stops.
type StructuralStopAssignment struct {
	Status             string  `json:"status"`
	Side               string  `json:"side"`
	At                 int64   `json:"at"`
	Entry              float64 `json:"entry"`
	PivotAnchorAt      int64   `json:"pivot_anchor_at,omitempty"`
	PivotConfirmedAt   int64   `json:"pivot_confirmed_at,omitempty"`
	AnchorAgeBars      int     `json:"anchor_age_bars,omitempty"`
	ConfirmAgeBars     int     `json:"confirm_age_bars,omitempty"`
	Stop               float64 `json:"stop,omitempty"`
	R                  float64 `json:"r,omitempty"`
	RPct               float64 `json:"r_pct,omitempty"`
	ROverATR15         float64 `json:"r_over_atr15,omitempty"`
	ROverATR1h         float64 `json:"r_over_atr1h,omitempty"`
	Plus1R             float64 `json:"plus_1r,omitempty"`
	Plus2R             float64 `json:"plus_2r,omitempty"`
	Plus3R             float64 `json:"plus_3r,omitempty"`
	Plus2ATR           float64 `json:"plus_2atr15,omitempty"`
	TwoATROverR        float64 `json:"two_atr_over_r,omitempty"`
	HEndAt             int64   `json:"h_end_at,omitempty"`
	StopHit            bool    `json:"stop_hit,omitempty"`
	Hit1RBeforeStop    bool    `json:"hit_1r_before_stop,omitempty"`
	Hit2RBeforeStop    bool    `json:"hit_2r_before_stop,omitempty"`
	Hit3RBeforeStop    bool    `json:"hit_3r_before_stop,omitempty"`
	Hit2ATRBeforeStop  bool    `json:"hit_2atr_before_stop,omitempty"`
	TimeToStop         float64 `json:"time_to_stop,omitempty"`
	TimeTo1R           float64 `json:"time_to_1r,omitempty"`
	TimeTo2R           float64 `json:"time_to_2r,omitempty"`
	TimeTo3R           float64 `json:"time_to_3r,omitempty"`
	TimeTo2ATR         float64 `json:"time_to_2atr,omitempty"`
	MFEBeforeStopOverR float64 `json:"mfe_before_stop_over_r,omitempty"`
	MFEFullHOverR      float64 `json:"mfe_full_h_over_r,omitempty"`
	MAEBeforeStopOverR float64 `json:"mae_before_stop_over_r,omitempty"`
	MAEFullHOverR      float64 `json:"mae_full_h_over_r,omitempty"`
	MFEBeforePrice     float64 `json:"mfe_before_price,omitempty"`
	MFEFullPrice       float64 `json:"mfe_full_price,omitempty"`
	MFEBeforeAt        int64   `json:"mfe_before_at,omitempty"`
	MFEFullAt          int64   `json:"mfe_full_at,omitempty"`
	StopHitAt          int64   `json:"stop_hit_at,omitempty"`
	IncompleteH        bool    `json:"incomplete_h,omitempty"`
}

// StructuralStopReport is descriptive. No winner / no target freeze.
type StructuralStopReport struct {
	Assignments []StructuralStopAssignment
	LongSample  []int
	Notes       []string
	Text        string
	WickOwner   string
}

func collect50Cross(tape []FeatureRow2) []geomEvent {
	var out []geomEvent
	for _, row := range tape {
		if ignitionAge0(row, FeatureRSX50CrossUpPresent, FeatureRSX50CrossUpAge) {
			out = append(out, geomEvent{At: row.At, Fam: GeomFamily50Cross, Side: GeomSideLong})
		}
		if ignitionAge0(row, FeatureRSX50CrossDownPresent, FeatureRSX50CrossDownAge) {
			out = append(out, geomEvent{At: row.At, Fam: GeomFamily50Cross, Side: GeomSideShort})
		}
	}
	return out
}

func latestConfirmedFractal(facts []indicators.IndicatorFactEvent, candidateAt int64, wantDir string) (indicators.IndicatorFactEvent, bool) {
	var best indicators.IndicatorFactEvent
	ok := false
	for _, ev := range facts {
		if ev.Source != indicators.FactSourceRSXFractalPivot {
			continue
		}
		if ev.Direction != wantDir {
			continue
		}
		if ev.ConfirmedAt <= 0 || ev.AnchorAt <= 0 {
			continue
		}
		if ev.ConfirmedAt > candidateAt {
			continue
		}
		if !ok || ev.ConfirmedAt > best.ConfirmedAt || (ev.ConfirmedAt == best.ConfirmedAt && ev.AnchorAt > best.AnchorAt) {
			best, ok = ev, true
		}
	}
	return best, ok
}

func barAge(fromAt, toAt int64, tf string) (int, error) {
	if fromAt > toAt {
		return -1, nil
	}
	if fromAt == toAt {
		return 0, nil
	}
	n := 0
	x := fromAt
	for x < toAt {
		nx, err := data.NextBarOpen(x, tf)
		if err != nil {
			return 0, err
		}
		if nx <= x {
			return 0, fmt.Errorf("forecast: NextBarOpen stalled")
		}
		x = nx
		n++
		if n > 2_000_000 {
			return n, nil
		}
	}
	return n, nil
}

func hEndAt(bars []CanonicalClosedBar, t, h int, tf string) int64 {
	if t < 0 || t >= len(bars) {
		return 0
	}
	at := bars[t].OpenTime
	for i := 0; i < h; i++ {
		nx, err := data.NextBarOpen(at, tf)
		if err != nil {
			return 0
		}
		at = nx
	}
	return at
}

func excursionUntil(bars []CanonicalClosedBar, t int, tf string, entry float64, long bool, stopAt int64, h int, favorable bool) (ext float64, px float64, at int64, complete bool, err error) {
	if t < 0 || t >= len(bars) {
		return 0, 0, 0, false, nil
	}
	prev := bars[t].OpenTime
	px = entry
	at = bars[t].OpenTime
	for n := 0; n < h; n++ {
		expected, e := data.NextBarOpen(prev, tf)
		if e != nil {
			return 0, 0, 0, false, e
		}
		k := t + 1 + n
		if k >= len(bars) {
			return ext, px, at, false, nil
		}
		b := bars[k]
		if b.OpenTime != expected {
			return ext, px, at, false, nil
		}
		var candPx, cand float64
		switch {
		case favorable && long:
			candPx, cand = b.High, b.High-entry
		case favorable && !long:
			candPx, cand = b.Low, entry-b.Low
		case !favorable && long:
			candPx, cand = b.Low, entry-b.Low
		default:
			candPx, cand = b.High, b.High-entry
		}
		if cand > ext {
			ext = cand
			px = candPx
			at = b.OpenTime
		}
		prev = b.OpenTime
		if stopAt > 0 && b.OpenTime == stopAt {
			return ext, px, at, true, nil
		}
	}
	return ext, px, at, true, nil
}

func evalFavorableVsStop(bars []CanonicalClosedBar, t int, tf string, h int, finer *FinerBarrierResolver, stop, target float64, long bool) (favFirst, stopFirst bool, favTime, stopTime float64, hitAt int64, err error) {
	favTime, stopTime = math.NaN(), math.NaN()
	var upper, lower float64
	if long {
		upper, lower = target, stop
	} else {
		upper, lower = stop, target
	}
	row, err := EvaluateBarriers(bars, t, upper, lower, h, tf, finer)
	if err != nil {
		return false, false, favTime, stopTime, 0, err
	}
	barsTo := func(at int64) float64 {
		if at <= 0 {
			return math.NaN()
		}
		n, e := barAge(bars[t].OpenTime, at, tf)
		if e != nil || n < 0 {
			return math.NaN()
		}
		return float64(n)
	}
	switch row.Outcome {
	case OutcomeUpFirst:
		if long {
			return true, false, barsTo(row.HitAt), math.NaN(), row.HitAt, nil
		}
		return false, true, math.NaN(), barsTo(row.HitAt), row.HitAt, nil
	case OutcomeDownFirst:
		if long {
			return false, true, math.NaN(), barsTo(row.HitAt), row.HitAt, nil
		}
		return true, false, barsTo(row.HitAt), math.NaN(), row.HitAt, nil
	default:
		return false, false, math.NaN(), math.NaN(), 0, nil
	}
}

func evenIndices(n, want int) []int {
	if n <= 0 || want <= 0 {
		return nil
	}
	if n <= want {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	out := make([]int, want)
	for i := 0; i < want; i++ {
		out[i] = i * n / want
	}
	return out
}

func summarizeFinite(xs []float64) (p25, p50, p75 float64, n int) {
	var v []float64
	for _, x := range xs {
		if isFinite(x) {
			v = append(v, x)
		}
	}
	sort.Float64s(v)
	return qtile(v, 0.25), qtile(v, 0.50), qtile(v, 0.75), len(v)
}

// RunStructuralStop1 assigns fractal-pivot stops to 50-cross ignitions and
// measures path potential. It does not freeze a TargetSpec.
func RunStructuralStop1(in StructuralStopInput) (StructuralStopReport, error) {
	var z StructuralStopReport
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
	events := collect50Cross(in.Tape)
	ats := make([]int64, len(in.Tape))
	for i, r := range in.Tape {
		ats[i] = r.At
	}
	tapeIdx, err := joinCandidatesToPrimary(in.Primary, ats)
	if err != nil {
		return z, err
	}
	atToIdx := map[int64]int{}
	for i, at := range ats {
		atToIdx[at] = tapeIdx[i]
	}
	openIdx := map[int64]int{}
	for i, b := range in.Primary {
		openIdx[b.OpenTime] = i
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
			atr1hAt[i], ok1hAt[i] = v, true
		}
	}
	finer := NewFinerBarrierResolver(in.FinerMarket, in.PrimaryTF, in.Finer)
	for _, e := range events {
		idx, ok := atToIdx[e.At]
		if !ok {
			return z, fmt.Errorf("forecast: 50-cross At %d missing from primary", e.At)
		}
		a := StructuralStopAssignment{Side: e.Side, At: e.At, Entry: in.Primary[idx].Close}
		a.HEndAt = hEndAt(in.Primary, idx, StructuralStopHorizon, in.PrimaryTF)
		long := e.Side == GeomSideLong
		owner := in.StopOwner
		if owner == "" {
			owner = StopOwnerRSXFractal
		}
		var stop float64
		var pivAnchor, pivConf int64
		var found bool
		if owner == StopOwnerPriceK2 {
			sw, err := LatestCausalPriceSwing(in.Primary, e.At, 2, long)
			if err != nil {
				return z, err
			}
			found = sw.Found
			pivAnchor, pivConf, stop = sw.AnchorAt, sw.ConfirmedAt, sw.Wick
			if found && sw.InvalidGeo {
				a.PivotAnchorAt, a.PivotConfirmedAt, a.Stop = pivAnchor, pivConf, stop
				aa, err := barAge(pivAnchor, e.At, in.PrimaryTF)
				if err != nil {
					return z, err
				}
				ca, err := barAge(pivConf, e.At, in.PrimaryTF)
				if err != nil {
					return z, err
				}
				a.AnchorAgeBars, a.ConfirmAgeBars = aa, ca
				a.Status = StructuralStopStatusInvalid
				z.Assignments = append(z.Assignments, a)
				continue
			}
		} else {
			want := indicators.FactDirPivotLow
			if !long {
				want = indicators.FactDirPivotHigh
			}
			piv, ok := latestConfirmedFractal(in.Fractals, e.At, want)
			found = ok
			if found {
				pivAnchor, pivConf = piv.AnchorAt, piv.ConfirmedAt
				pb, ok := openIdx[piv.AnchorAt]
				if !ok {
					a.Status = StructuralStopStatusNone
					a.PivotAnchorAt, a.PivotConfirmedAt = piv.AnchorAt, piv.ConfirmedAt
					z.Assignments = append(z.Assignments, a)
					continue
				}
				stop = in.Primary[pb].Low
				if !long {
					stop = in.Primary[pb].High
				}
			}
		}
		if !found {
			a.Status = StructuralStopStatusNone
			z.Assignments = append(z.Assignments, a)
			continue
		}
		a.PivotAnchorAt = pivAnchor
		a.PivotConfirmedAt = pivConf
		a.Stop = stop
		aa, err := barAge(pivAnchor, e.At, in.PrimaryTF)
		if err != nil {
			return z, err
		}
		ca, err := barAge(pivConf, e.At, in.PrimaryTF)
		if err != nil {
			return z, err
		}
		a.AnchorAgeBars, a.ConfirmAgeBars = aa, ca
		if !isFinite(stop) || (long && stop >= a.Entry) || (!long && stop <= a.Entry) {
			a.Status = StructuralStopStatusInvalid
			z.Assignments = append(z.Assignments, a)
			continue
		}
		r := math.Abs(a.Entry - stop)
		if !(r > 0) {
			a.Status = StructuralStopStatusInvalid
			z.Assignments = append(z.Assignments, a)
			continue
		}
		a.Status = StructuralStopStatusValid
		a.R = r
		if a.Entry != 0 {
			a.RPct = 100 * r / math.Abs(a.Entry)
		}
		v15 := atr15[idx]
		if isFinite(v15) && v15 > 0 {
			a.ROverATR15 = r / v15
			if long {
				a.Plus2ATR = a.Entry + 2*v15
			} else {
				a.Plus2ATR = a.Entry - 2*v15
			}
			a.TwoATROverR = (2 * v15) / r
		}
		if ok1hAt[idx] {
			a.ROverATR1h = r / atr1hAt[idx]
		}
		if long {
			a.Plus1R, a.Plus2R, a.Plus3R = a.Entry+r, a.Entry+2*r, a.Entry+3*r
		} else {
			a.Plus1R, a.Plus2R, a.Plus3R = a.Entry-r, a.Entry-2*r, a.Entry-3*r
		}
		type lvl struct {
			px   float64
			setH func(bool)
			setT func(float64)
		}
		levels := []lvl{
			{a.Plus1R, func(v bool) { a.Hit1RBeforeStop = v }, func(v float64) { a.TimeTo1R = v }},
			{a.Plus2R, func(v bool) { a.Hit2RBeforeStop = v }, func(v float64) { a.TimeTo2R = v }},
			{a.Plus3R, func(v bool) { a.Hit3RBeforeStop = v }, func(v float64) { a.TimeTo3R = v }},
		}
		if isFinite(a.Plus2ATR) {
			levels = append(levels, lvl{a.Plus2ATR, func(v bool) { a.Hit2ATRBeforeStop = v }, func(v float64) { a.TimeTo2ATR = v }})
		}
		a.TimeToStop, a.TimeTo1R, a.TimeTo2R, a.TimeTo3R, a.TimeTo2ATR = 0, 0, 0, 0, 0
		for _, lv := range levels {
			if !isFinite(lv.px) {
				continue
			}
			fav, st, ft, stt, hitAt, err := evalFavorableVsStop(in.Primary, idx, in.PrimaryTF, StructuralStopHorizon, finer, stop, lv.px, long)
			if err != nil {
				return z, err
			}
			lv.setH(fav)
			if isFinite(ft) {
				lv.setT(ft)
			}
			if st {
				a.StopHit = true
				if a.TimeToStop == 0 && isFinite(stt) {
					a.TimeToStop = stt
					a.StopHitAt = hitAt
				}
			}
		}
		stopCut := int64(0)
		if a.StopHit {
			stopCut = a.StopHitAt
		}
		mfeB, pxB, atB, okB, err := excursionUntil(in.Primary, idx, in.PrimaryTF, a.Entry, long, stopCut, StructuralStopHorizon, true)
		if err != nil {
			return z, err
		}
		mfeF, pxF, atF, okF, err := excursionUntil(in.Primary, idx, in.PrimaryTF, a.Entry, long, 0, StructuralStopHorizon, true)
		if err != nil {
			return z, err
		}
		maeB, _, _, okMAE_B, err := excursionUntil(in.Primary, idx, in.PrimaryTF, a.Entry, long, stopCut, StructuralStopHorizon, false)
		if err != nil {
			return z, err
		}
		maeF, _, _, okMAE_F, err := excursionUntil(in.Primary, idx, in.PrimaryTF, a.Entry, long, 0, StructuralStopHorizon, false)
		if err != nil {
			return z, err
		}
		a.IncompleteH = !okB || !okF || !okMAE_B || !okMAE_F
		if okF || mfeF > 0 {
			a.MFEFullHOverR = mfeF / r
			a.MFEFullPrice, a.MFEFullAt = pxF, atF
		}
		if okB || mfeB > 0 {
			a.MFEBeforeStopOverR = mfeB / r
			a.MFEBeforePrice, a.MFEBeforeAt = pxB, atB
		}
		if okMAE_B || maeB > 0 {
			a.MAEBeforeStopOverR = maeB / r
		}
		if okMAE_F || maeF > 0 {
			a.MAEFullHOverR = maeF / r
		}
		z.Assignments = append(z.Assignments, a)
	}
	var longIdx []int
	for i, a := range z.Assignments {
		if a.Side == GeomSideLong {
			longIdx = append(longIdx, i)
		}
	}
	z.LongSample = evenIndices(len(longIdx), 30)
	for i, li := range z.LongSample {
		z.LongSample[i] = longIdx[li]
	}
	z.WickOwner = in.StopOwner
	if z.WickOwner == "" {
		z.WickOwner = StopOwnerRSXFractal
	}
	z.Notes = []string{
		"NO TARGET SELECTED",
		"stop is AnchorAt bar Low/High, not fact.AnchorPrice (hlc3)",
		"fractal facts must be research Jurik + default radius; not live rsx_settings.json",
		"+2 ATR15 is a ruler, not a frozen target",
		"NO_STRUCTURE / INVALID_GEOMETRY have no ATR fallback",
	}
	if z.WickOwner == StopOwnerPriceK2 {
		z.Notes = []string{
			"NO TARGET SELECTED",
			"wick owner = latest causal price k=2 (S0); RSX is timing only",
			"no 0.25 ATR buffer yet; S1 prominence walk deferred",
			"NO_STRUCTURE / INVALID_GEOMETRY have no ATR fallback",
		}
	}
	z.Text = FormatStructuralStop1(z)
	return z, nil
}

func sideStats(rows []StructuralStopAssignment, side string) string {
	var all, valid, none, inv int
	var rATR, r1h, rPct, twoOver, mfeB, mfeF, maeB, maeF, agesA, agesC []float64
	var tStop, t1, t2, t3, tA []float64
	var stopN, n1, n2, n3, nA, usable int
	for _, a := range rows {
		if a.Side != side {
			continue
		}
		all++
		switch a.Status {
		case StructuralStopStatusValid:
			valid++
			if a.IncompleteH {
				break
			}
			usable++
			if isFinite(a.ROverATR15) {
				rATR = append(rATR, a.ROverATR15)
			}
			if isFinite(a.ROverATR1h) {
				r1h = append(r1h, a.ROverATR1h)
			}
			if isFinite(a.RPct) {
				rPct = append(rPct, a.RPct)
			}
			if isFinite(a.TwoATROverR) {
				twoOver = append(twoOver, a.TwoATROverR)
			}
			if isFinite(a.MFEBeforeStopOverR) {
				mfeB = append(mfeB, a.MFEBeforeStopOverR)
			}
			if isFinite(a.MFEFullHOverR) {
				mfeF = append(mfeF, a.MFEFullHOverR)
			}
			if isFinite(a.MAEBeforeStopOverR) {
				maeB = append(maeB, a.MAEBeforeStopOverR)
			}
			if isFinite(a.MAEFullHOverR) {
				maeF = append(maeF, a.MAEFullHOverR)
			}
			agesA = append(agesA, float64(a.AnchorAgeBars))
			agesC = append(agesC, float64(a.ConfirmAgeBars))
			if a.StopHit {
				stopN++
				if a.TimeToStop > 0 {
					tStop = append(tStop, a.TimeToStop)
				}
			}
			if a.Hit1RBeforeStop {
				n1++
				if a.TimeTo1R > 0 {
					t1 = append(t1, a.TimeTo1R)
				}
			}
			if a.Hit2RBeforeStop {
				n2++
				if a.TimeTo2R > 0 {
					t2 = append(t2, a.TimeTo2R)
				}
			}
			if a.Hit3RBeforeStop {
				n3++
				if a.TimeTo3R > 0 {
					t3 = append(t3, a.TimeTo3R)
				}
			}
			if a.Hit2ATRBeforeStop {
				nA++
				if a.TimeTo2ATR > 0 {
					tA = append(tA, a.TimeTo2ATR)
				}
			}
		case StructuralStopStatusNone:
			none++
		case StructuralStopStatusInvalid:
			inv++
		}
	}
	p := func(n int) float64 { return cellRate(n, usable) }
	a25, a50, a75, _ := summarizeFinite(agesA)
	c25, c50, c75, _ := summarizeFinite(agesC)
	r25, r50, r75, _ := summarizeFinite(rATR)
	h25, h50, h75, _ := summarizeFinite(r1h)
	p25, p50, p75, _ := summarizeFinite(rPct)
	t25, t50, t75, _ := summarizeFinite(twoOver)
	b25, b50, b75, _ := summarizeFinite(mfeB)
	f25, f50, f75, _ := summarizeFinite(mfeF)
	mb25, mb50, mb75, _ := summarizeFinite(maeB)
	mf25, mf50, mf75, _ := summarizeFinite(maeF)
	s25, s50, s75, _ := summarizeFinite(tStop)
	u25, u50, u75, _ := summarizeFinite(t1)
	v25, v50, v75, _ := summarizeFinite(t2)
	w25, w50, w75, _ := summarizeFinite(t3)
	x25, x50, x75, _ := summarizeFinite(tA)
	return fmt.Sprintf("%s events=%d VALID=%d NO_STRUCTURE=%d INVALID=%d usableH=%d\n  anchor_age p25/50/75=%.1f/%.1f/%.1f confirm_age p25/50/75=%.1f/%.1f/%.1f\n  R/price%% p25/50/75=%.3f/%.3f/%.3f\n  R/ATR15 p25/50/75=%.3f/%.3f/%.3f R/ATR1h p25/50/75=%.3f/%.3f/%.3f\n  2ATR/R p25/50/75=%.3f/%.3f/%.3f\n  stopHit=%.3f +1R=%.3f +2R=%.3f +3R=%.3f +2ATR=%.3f\n  time-to-stop p25/50/75=%.1f/%.1f/%.1f 1R=%.1f/%.1f/%.1f 2R=%.1f/%.1f/%.1f 3R=%.1f/%.1f/%.1f 2ATR=%.1f/%.1f/%.1f\n  MFE_before/R p25/50/75=%.3f/%.3f/%.3f MFE_fullH/R p25/50/75=%.3f/%.3f/%.3f\n  MAE_before/R p25/50/75=%.3f/%.3f/%.3f MAE_fullH/R p25/50/75=%.3f/%.3f/%.3f\n",
		side, all, valid, none, inv, usable,
		a25, a50, a75, c25, c50, c75,
		p25, p50, p75,
		r25, r50, r75, h25, h50, h75,
		t25, t50, t75,
		p(stopN), p(n1), p(n2), p(n3), p(nA),
		s25, s50, s75, u25, u50, u75, v25, v50, v75, w25, w50, w75, x25, x50, x75,
		b25, b50, b75, f25, f50, f75,
		mb25, mb50, mb75, mf25, mf50, mf75)
}

// FormatStructuralStop1 is the chapter report. No ranking.
func FormatStructuralStop1(r StructuralStopReport) string {
	s := "STRUCTURAL-STOP-1 (descriptive; NO TARGET SELECTED)\n\n"
	if r.WickOwner == StopOwnerPriceK2 {
		s = "STRUCTURAL-STOP-2 S0 price k=2 (descriptive; NO TARGET; NO 0.25 ATR yet)\n\n"
	}
	s += sideStats(r.Assignments, GeomSideLong)
	s += sideStats(r.Assignments, GeomSideShort)
	s += fmt.Sprintf("LONG visual sample n=%d (evenly spaced, not by outcome)\n", len(r.LongSample))
	s += "\nNOTES\n"
	for _, n := range r.Notes {
		s += "  - " + n + "\n"
	}
	return s
}
