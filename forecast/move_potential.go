package forecast

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

var movePotentialCDFKnots = []float64{12, 24, 36, 48, 60, 72}

const (
	MoveBucketLevelFirst = "LEVEL_FIRST"
	MoveBucketStopFirst  = "STOP_FIRST"
	MoveBucketNeither    = "TIMEOUT_NEITHER"
	MoveBucketSkip       = "NOT_EVALUABLE"
)

// FormatMovePotential1 is descriptive R-space path potential for the frozen
// STRUCTURAL-STOP-2 law. It does not freeze a target or compute EV.
func FormatMovePotential1(r StructuralStopReport) string {
	var b strings.Builder
	b.WriteString("MOVE-POTENTIAL-1 (descriptive; NO TARGET SELECTED; not strategy EV)\n")
	b.WriteString("stop = S0 k=2 ± 0.15×Canonical ATR14 15m(entry); H=72; R=|entry-stop|\n")
	b.WriteString("MFE_before_stop uses EvaluateBarriers stop time + 1m until stop (stop bar High/Low not taken whole)\n\n")
	b.WriteString(movePotentialSide(r.Assignments, GeomSideLong))
	b.WriteString(movePotentialSide(r.Assignments, GeomSideShort))
	b.WriteString("NOTES\n")
	b.WriteString("  - H is not changed in this chapter\n")
	b.WriteString("  - time_to_MFE_before_stop informs the horizon; time_to_MFE_full_H is diagnostic\n")
	b.WriteString("  - per-level buckets are independent; no MetaLabel freeze\n")
	b.WriteString("  - incomplete H and 1m dual-hit Ambiguous are NOT_EVALUABLE, not timeout\n")
	b.WriteString("  - no fees, slippage, or expected profit\n")
	return b.String()
}

func movePotentialSide(rows []StructuralStopAssignment, side string) string {
	type lvl struct {
		name string
		hit  func(StructuralStopAssignment) bool
		tim  func(StructuralStopAssignment) float64
	}
	levels := []lvl{
		{"+2ATR", func(a StructuralStopAssignment) bool { return a.Hit2ATRBeforeStop }, func(a StructuralStopAssignment) float64 { return a.TimeTo2ATR }},
		{"+1R", func(a StructuralStopAssignment) bool { return a.Hit1RBeforeStop }, func(a StructuralStopAssignment) float64 { return a.TimeTo1R }},
		{"+2R", func(a StructuralStopAssignment) bool { return a.Hit2RBeforeStop }, func(a StructuralStopAssignment) float64 { return a.TimeTo2R }},
		{"+3R", func(a StructuralStopAssignment) bool { return a.Hit3RBeforeStop }, func(a StructuralStopAssignment) float64 { return a.TimeTo3R }},
	}
	var n1, n2, n3 int
	var mfeB, mfeF, tMB, tMF []float64
	var first, stopB, neither, skip [4]int
	var times [4][]float64
	var valid, usable int
	for _, a := range rows {
		if a.Side != side {
			continue
		}
		if a.Status != StructuralStopStatusValid {
			continue
		}
		valid++
		eval := a.IncompleteH || a.PathAmbiguous
		if !eval {
			usable++
			if a.Hit1RBeforeStop {
				n1++
			}
			if a.Hit2RBeforeStop {
				n2++
			}
			if a.Hit3RBeforeStop {
				n3++
			}
			if isFinite(a.MFEBeforeStopOverR) {
				mfeB = append(mfeB, a.MFEBeforeStopOverR)
			}
			if isFinite(a.MFEFullHOverR) {
				mfeF = append(mfeF, a.MFEFullHOverR)
			}
			if isFinite(a.TimeToMFEBefore) {
				tMB = append(tMB, a.TimeToMFEBefore)
			}
			if isFinite(a.TimeToMFEFullH) {
				tMF = append(tMF, a.TimeToMFEFullH)
			}
		}
		for i, lv := range levels {
			if eval {
				skip[i]++
				continue
			}
			if lv.hit(a) {
				first[i]++
				if isFinite(lv.tim(a)) && lv.tim(a) > 0 {
					times[i] = append(times[i], lv.tim(a))
				}
				continue
			}
			if a.StopHit {
				stopB[i]++
				continue
			}
			neither[i]++
		}
	}
	_ = skip
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s VALID_STRUCTURE=%d usable=%d NOT_EVALUABLE=%d\n", side, valid, usable, valid-usable))
	for i, lv := range levels {
		den := first[i] + stopB[i] + neither[i]
		p50, p75, p90, _ := summarizeTimes3(times[i])
		b.WriteString(fmt.Sprintf("  %s  %s=%.3f  %s=%.3f  %s=%.3f  n=%d  time p50/p75/p90=%.1f/%.1f/%.1f\n",
			lv.name,
			MoveBucketLevelFirst, cellRate(first[i], den),
			MoveBucketStopFirst, cellRate(stopB[i], den),
			MoveBucketNeither, cellRate(neither[i], den),
			den, p50, p75, p90))
	}
	b.WriteString(fmt.Sprintf("  P(2R|1R)=%.3f  P(3R|2R)=%.3f\n", cellRate(n2, n1), cellRate(n3, n2)))
	_, b50, b75, b90, _ := summarizeFinite4(mfeB)
	_, f50, f75, f90, _ := summarizeFinite4(mfeF)
	b.WriteString(fmt.Sprintf("  MFE_before/R p50/p75/p90=%.3f/%.3f/%.3f\n", b50, b75, b90))
	b.WriteString(fmt.Sprintf("  MFE_fullH/R  p50/p75/p90=%.3f/%.3f/%.3f\n", f50, f75, f90))
	mb50, mb75, mb90, _ := summarizeTimes3(tMB)
	mf50, mf75, mf90, _ := summarizeTimes3(tMF)
	b.WriteString(fmt.Sprintf("  time_to_MFE_before p50/p75/p90=%.1f/%.1f/%.1f\n", mb50, mb75, mb90))
	b.WriteString(fmt.Sprintf("  time_to_MFE_fullH  p50/p75/p90=%.1f/%.1f/%.1f\n", mf50, mf75, mf90))
	b.WriteString("  MFE_before CDF bars")
	for _, k := range movePotentialCDFKnots {
		b.WriteString(fmt.Sprintf("  %.0f=%.3f", k, cdfAtMost(tMB, k)))
	}
	b.WriteString("\n  MFE_fullH  CDF bars")
	for _, k := range movePotentialCDFKnots {
		b.WriteString(fmt.Sprintf("  %.0f=%.3f", k, cdfAtMost(tMF, k)))
	}
	b.WriteString("\n\n")
	return b.String()
}

func summarizeFinite4(xs []float64) (p25, p50, p75, p90 float64, n int) {
	var v []float64
	for _, x := range xs {
		if isFinite(x) {
			v = append(v, x)
		}
	}
	if len(v) == 0 {
		return math.NaN(), math.NaN(), math.NaN(), math.NaN(), 0
	}
	sort.Float64s(v)
	return qtile(v, 0.25), qtile(v, 0.50), qtile(v, 0.75), qtile(v, 0.90), len(v)
}

func summarizeTimes3(xs []float64) (p50, p75, p90 float64, n int) {
	var v []float64
	for _, x := range xs {
		if isFinite(x) {
			v = append(v, x)
		}
	}
	if len(v) == 0 {
		return math.NaN(), math.NaN(), math.NaN(), 0
	}
	sort.Float64s(v)
	return qtile(v, 0.50), qtile(v, 0.75), qtile(v, 0.90), len(v)
}

func cdfAtMost(xs []float64, knot float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	n := 0
	for _, x := range xs {
		if isFinite(x) && x <= knot {
			n++
		}
	}
	return cellRate(n, len(xs))
}
