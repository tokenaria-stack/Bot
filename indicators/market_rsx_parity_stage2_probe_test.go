// MARKET-RSX-PARITY-1 Stage 2 — disposable probe. Do not commit as a lab.
// Reference replica is literal Everget Pine (docs/PINE_INDICATOR_SOURCES.md).
// It must not call JurikRSX, scanRSXTVHits, or rolling-extrema helpers.
package indicators

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"
)

const (
	probeLength = 14
	probeXBars  = 90
	probeCSV    = "/tmp/btcusdt_15m_prefix.csv"
)

type probeBar struct {
	openTime int64
	o, h, l, c float64
}

type probeDivEvent struct {
	confirmI int
	dir      string // "bear" | "bull"
}

func TestMarketRSXParityStage2Probe(t *testing.T) {
	bars := loadProbeCSV(t, probeCSV)
	n := len(bars)
	if n < probeXBars+16 {
		t.Fatalf("prefix too short: %d", n)
	}

	hlc3 := make([]float64, n)
	closes := make([]float64, n)
	opens := make([]int64, n)
	for i, b := range bars {
		hlc3[i] = (b.h + b.l + b.c) / 3.0
		closes[i] = b.c
		opens[i] = b.openTime
	}

	refRSX := literalPineJurikRSX(hlc3, probeLength)
	prod := NewJurikRSX(probeLength)
	prodRSX := make([]float64, n)
	for i := range hlc3 {
		prodRSX[i] = prod.Update(hlc3[i])
	}

	fmt.Printf("MARKET-RSX-PARITY-1 STAGE 2\n")
	fmt.Printf("MarketKey: BINANCE BTCUSDT FUTURES_PERP 15m\n")
	fmt.Printf("firstOpenTime=%d lastOpenTime=%d bars=%d length=%d xbars=%d src=hlc3\n",
		opens[0], opens[n-1], n, probeLength, probeXBars)

	firstI, maxAbs := -1, 0.0
	obMismatch, osMismatch := 0, 0
	for i := 0; i < n; i++ {
		d := math.Abs(refRSX[i] - prodRSX[i])
		if d > maxAbs {
			maxAbs = d
		}
		if d != 0 && firstI < 0 {
			firstI = i
		}
		if (refRSX[i] > 70) != (prodRSX[i] > 70) {
			obMismatch++
		}
		if (refRSX[i] < 30) != (prodRSX[i] < 30) {
			osMismatch++
		}
	}

	if firstI >= 0 {
		fmt.Printf("2A first non-bit-equal i=%d openTime=%d hlc3=%.12g ref=%.12g prod=%.12g absDelta=%.12g\n",
			firstI, opens[firstI], hlc3[firstI], refRSX[firstI], prodRSX[firstI], math.Abs(refRSX[firstI]-prodRSX[firstI]))
	}
	fmt.Printf("2A maxAbsDelta=%.12g obZoneMismatch=%d osZoneMismatch=%d\n", maxAbs, obMismatch, osMismatch)
	const jurikUlpOK = 1e-9
	if maxAbs > jurikUlpOK || obMismatch > 0 || osMismatch > 0 {
		fmt.Printf("CLASSIFICATION: JURIK PARITY DEFECT\n")
		return
	}
	if maxAbs == 0 {
		fmt.Printf("2A Jurik: BIT-EQUAL on all %d bars\n", n)
	} else {
		fmt.Printf("2A Jurik: GREEN (ulp only, no 70/30 zone flip)\n")
	}

	eqMax, eqMin, n100, n0 := extremaNotes(refRSX, probeXBars)
	fmt.Printf("edge: equalMaxWindows=%d equalMinWindows=%d exact100=%d exact0=%d\n", eqMax, eqMin, n100, n0)

	refTrace := literalEvergetDiv(closes, refRSX, probeXBars)
	fmt.Printf("tie_rule_replica: newest-wins (strict > / <, seed current bar). NOT certified Pine builtin.\n")
	fmt.Printf("2B replica both-on-same-confirm=%d  bear=%d bull=%d\n",
		refTrace.bothSame, len(refTrace.bears), len(refTrace.bulls))

	prodHits := scanRSXTVHits(closes, prodRSX, probeXBars)
	prodBears, prodBulls := splitHits(prodHits)
	prodBoth := 0
	bearAt := map[int]bool{}
	for _, b := range prodBears {
		bearAt[b.confirmI] = true
	}
	for _, b := range prodBulls {
		if bearAt[b.confirmI] {
			prodBoth++
		}
	}
	fmt.Printf("2B production bear=%d bull=%d both-on-same-confirm=%d (scanRSXTVHits)\n",
		len(prodBears), len(prodBulls), prodBoth)

	layer, di, detail := firstDivBreak(refTrace, prodRSX, prodBears, prodBulls, opens, probeXBars)
	if layer == "" {
		fmt.Printf("2B divergence events MATCH (confirm OpenTime + direction + AnchorAt=confirm-1)\n")
		fmt.Printf("CLASSIFICATION: GO MATCHES LITERAL PINE REPLICA\n")
		fmt.Printf("rsx_value: certified vs literal Pine replica (not vs TV runtime)\n")
		fmt.Printf("rsx_tv_div: certified vs replica only; Pine highestbars builtin uncertified\n")
		alt := literalEvergetDivOldestTie(closes, refRSX, probeXBars)
		fmt.Printf("sensitivity oldest-wins replica: bear=%d bull=%d (vs newest-wins %d/%d)\n",
			len(alt.bears), len(alt.bulls), len(refTrace.bears), len(refTrace.bulls))
		if sameDivEvents(refTrace, alt) {
			fmt.Printf("sensitivity: oldest-wins EVENTS identical to newest-wins on this prefix (ties present but did not move divs)\n")
		} else {
			fmt.Printf("sensitivity: oldest-wins EVENTS DIFFER from newest-wins — ties can move labels on this prefix\n")
		}
		fmt.Printf("tv_bull_*: remain quarantined\n")
		fmt.Printf("TradingView export: YES — leftover is Pine builtin and/or TV history prefix\n")
		return
	}
	fmt.Printf("2B FIRST BREAK layer=%s i=%d openTime=%d %s\n", layer, di, opens[di], detail)
	fmt.Printf("CLASSIFICATION: %s\n", layer)
}

type evergetTrace struct {
	hb, lb                         []int
	maxC, minC, maxR, minR         []float64
	divBear, divBull               []bool
	bears, bulls                   []probeDivEvent
	bothSame                       int
}

func literalPineJurikRSX(src []float64, length int) []float64 {
	out := make([]float64, len(src))
	f18 := 3.0 / (float64(length) + 2.0)
	f20 := 1.0 - f18
	var prevF8, f28, f30, f38, f40, f48, f50, f58, f60, f68, f70, f78, f80 float64
	var prevF88, prevF90 float64
	for i, s := range src {
		f8 := 100.0 * s
		f10 := prevF8 // nz(f8[1]) → 0 on bar 0
		v8 := f8 - f10

		f28 = f20*f28 + f18*v8
		f30 = f18*f28 + f20*f30
		vC := f28*1.5 - f30*0.5
		f38 = f20*f38 + f18*vC
		f40 = f18*f38 + f20*f40
		v10 := f38*1.5 - f40*0.5
		f48 = f20*f48 + f18*v10
		f50 = f18*f48 + f20*f50
		v14 := f48*1.5 - f50*0.5
		f58 = f20*f58 + f18*math.Abs(v8)
		f60 = f18*f58 + f20*f60
		v18 := f58*1.5 - f60*0.5
		f68 = f20*f68 + f18*v18
		f70 = f18*f68 + f20*f70
		v1C := f68*1.5 - f70*0.5
		f78 = f20*f78 + f18*v1C
		f80 = f18*f78 + f20*f80
		v20 := f78*1.5 - f80*0.5

		var f90U float64
		if prevF90 == 0 {
			f90U = 1
		} else if prevF88 <= prevF90 {
			f90U = prevF88 + 1
		} else {
			f90U = prevF90 + 1
		}
		f88 := 5.0
		if prevF90 == 0 && float64(length)-1 >= 5 {
			f88 = float64(length) - 1
		}
		f0 := 0.0
		if f88 >= f90U && f8 != f10 {
			f0 = 1
		}
		f90 := f90U
		if f88 == f90U && f0 == 0 {
			f90 = 0
		}
		v4 := 50.0
		if f88 < f90 && v20 > 0 {
			v4 = (v14/v20 + 1) * 50
		}
		rsx := v4
		if rsx > 100 {
			rsx = 100
		}
		if rsx < 0 {
			rsx = 0
		}
		out[i] = rsx
		prevF8, prevF88, prevF90 = f8, f88, f90
	}
	return out
}

func replicaHighestBarsAgo(rsx []float64, i, xbars int) int {
	start := i - xbars + 1
	if start < 0 {
		start = 0
	}
	bestIdx, best := i, rsx[i]
	for j := start; j <= i; j++ {
		if rsx[j] > best {
			best, bestIdx = rsx[j], j
		}
	}
	return i - bestIdx
}

func replicaLowestBarsAgo(rsx []float64, i, xbars int) int {
	start := i - xbars + 1
	if start < 0 {
		start = 0
	}
	bestIdx, best := i, rsx[i]
	for j := start; j <= i; j++ {
		if rsx[j] < best {
			best, bestIdx = rsx[j], j
		}
	}
	return i - bestIdx
}

func literalEvergetDiv(closes, rsx []float64, xbars int) evergetTrace {
	n := len(rsx)
	tr := evergetTrace{
		hb: make([]int, n), lb: make([]int, n),
		maxC: make([]float64, n), minC: make([]float64, n),
		maxR: make([]float64, n), minR: make([]float64, n),
		divBear: make([]bool, n), divBull: make([]bool, n),
	}
	var maxC, maxR, minC, minR float64
	hasMax, hasMin := false, false
	for i := 0; i < n; i++ {
		hb := replicaHighestBarsAgo(rsx, i, xbars)
		lb := replicaLowestBarsAgo(rsx, i, xbars)
		tr.hb[i], tr.lb[i] = hb, lb

		if hb == 0 || !hasMax {
			maxC, maxR, hasMax = closes[i], rsx[i], true
		}
		if lb == 0 || !hasMin {
			minC, minR, hasMin = closes[i], rsx[i], true
		}
		if closes[i] > maxC {
			maxC = closes[i]
		}
		if rsx[i] > maxR {
			maxR = rsx[i]
		}
		if closes[i] < minC {
			minC = closes[i]
		}
		if rsx[i] < minR {
			minR = rsx[i]
		}
		tr.maxC[i], tr.maxR[i], tr.minC[i], tr.minR[i] = maxC, maxR, minC, minR

		if i >= 2 {
			if tr.maxC[i-1] > tr.maxC[i-2] && rsx[i-1] < maxR && rsx[i] <= rsx[i-1] {
				tr.divBear[i] = true
				tr.bears = append(tr.bears, probeDivEvent{confirmI: i, dir: "bear"})
			}
			if tr.minC[i-1] < tr.minC[i-2] && rsx[i-1] > minR && rsx[i] >= rsx[i-1] {
				tr.divBull[i] = true
				tr.bulls = append(tr.bulls, probeDivEvent{confirmI: i, dir: "bull"})
			}
			if tr.divBear[i] && tr.divBull[i] {
				tr.bothSame++
			}
		}
	}
	return tr
}

func replicaHighestBarsAgoOldest(rsx []float64, i, xbars int) int {
	start := i - xbars + 1
	if start < 0 {
		start = 0
	}
	bestIdx, best := start, rsx[start]
	for j := start + 1; j <= i; j++ {
		if rsx[j] > best {
			best, bestIdx = rsx[j], j
		}
	}
	return i - bestIdx
}

func replicaLowestBarsAgoOldest(rsx []float64, i, xbars int) int {
	start := i - xbars + 1
	if start < 0 {
		start = 0
	}
	bestIdx, best := start, rsx[start]
	for j := start + 1; j <= i; j++ {
		if rsx[j] < best {
			best, bestIdx = rsx[j], j
		}
	}
	return i - bestIdx
}

func literalEvergetDivOldestTie(closes, rsx []float64, xbars int) evergetTrace {
	n := len(rsx)
	tr := evergetTrace{}
	maxHist := make([]float64, n)
	minHist := make([]float64, n)
	var maxC, maxR, minC, minR float64
	hasMax, hasMin := false, false
	for i := 0; i < n; i++ {
		hb := replicaHighestBarsAgoOldest(rsx, i, xbars)
		lb := replicaLowestBarsAgoOldest(rsx, i, xbars)
		if hb == 0 || !hasMax {
			maxC, maxR, hasMax = closes[i], rsx[i], true
		}
		if lb == 0 || !hasMin {
			minC, minR, hasMin = closes[i], rsx[i], true
		}
		if closes[i] > maxC {
			maxC = closes[i]
		}
		if rsx[i] > maxR {
			maxR = rsx[i]
		}
		if closes[i] < minC {
			minC = closes[i]
		}
		if rsx[i] < minR {
			minR = rsx[i]
		}
		maxHist[i], minHist[i] = maxC, minC
		if i >= 2 {
			if maxHist[i-1] > maxHist[i-2] && rsx[i-1] < maxR && rsx[i] <= rsx[i-1] {
				tr.bears = append(tr.bears, probeDivEvent{confirmI: i, dir: "bear"})
			}
			if minHist[i-1] < minHist[i-2] && rsx[i-1] > minR && rsx[i] >= rsx[i-1] {
				tr.bulls = append(tr.bulls, probeDivEvent{confirmI: i, dir: "bull"})
			}
		}
	}
	return tr
}

func splitHits(hits []RSXMarkerHit) (bears, bulls []probeDivEvent) {
	for _, h := range hits {
		ev := probeDivEvent{confirmI: h.DisplayBar, dir: "bear"}
		if h.Label == "L" {
			ev.dir = "bull"
			bulls = append(bulls, ev)
		} else {
			bears = append(bears, ev)
		}
	}
	return
}

func sameDivEvents(a, b evergetTrace) bool {
	if len(a.bears) != len(b.bears) || len(a.bulls) != len(b.bulls) {
		return false
	}
	for i := range a.bears {
		if a.bears[i] != b.bears[i] {
			return false
		}
	}
	for i := range a.bulls {
		if a.bulls[i] != b.bulls[i] {
			return false
		}
	}
	return true
}

func firstDivBreak(ref evergetTrace, prodRSX []float64, prodBears, prodBulls []probeDivEvent, opens []int64, xbars int) (layer string, i int, detail string) {
	refSeq := append(append([]probeDivEvent{}, ref.bears...), ref.bulls...)
	prodSeq := append(append([]probeDivEvent{}, prodBears...), prodBulls...)
	sortDiv := func(s []probeDivEvent) {
		for a := 0; a < len(s); a++ {
			for b := a + 1; b < len(s); b++ {
				if s[b].confirmI < s[a].confirmI || (s[b].confirmI == s[a].confirmI && s[b].dir < s[a].dir) {
					s[a], s[b] = s[b], s[a]
				}
			}
		}
	}
	sortDiv(refSeq)
	sortDiv(prodSeq)
	lim := len(refSeq)
	if len(prodSeq) < lim {
		lim = len(prodSeq)
	}
	mismatch := -1
	var want, got probeDivEvent
	for k := 0; k < lim; k++ {
		if refSeq[k].confirmI != prodSeq[k].confirmI || refSeq[k].dir != prodSeq[k].dir {
			mismatch = k
			want, got = refSeq[k], prodSeq[k]
			break
		}
	}
	if mismatch < 0 && len(refSeq) == len(prodSeq) {
		return "", 0, ""
	}
	if mismatch < 0 {
		if len(refSeq) > len(prodSeq) {
			want = refSeq[len(prodSeq)]
			i = want.confirmI
		} else {
			got = prodSeq[len(refSeq)]
			i = got.confirmI
			want.dir = "(none)"
		}
	} else {
		i = want.confirmI
		if got.confirmI < i {
			i = got.confirmI
		}
	}
	prodHB := highestBarsAgo(prodRSX, i, xbars)
	prodLB := lowestBarsAgo(prodRSX, i, xbars)
	if prodHB != ref.hb[i] || prodLB != ref.lb[i] {
		detail = fmt.Sprintf("ref hb/lb=%d/%d prod hb/lb=%d/%d refEvent=%s@%d prodEvent=%s@%d",
			ref.hb[i], ref.lb[i], prodHB, prodLB, want.dir, want.confirmI, got.dir, got.confirmI)
		return "ROLLING EXTREMA PARITY DEFECT", i, detail
	}
	detail = fmt.Sprintf("hb/lb match (%d/%d) refEvent=%s@%d prodEvent=%s@%d max=%.8g max_rsi=%.8g min=%.8g min_rsi=%.8g",
		ref.hb[i], ref.lb[i], want.dir, want.confirmI, got.dir, got.confirmI, ref.maxC[i], ref.maxR[i], ref.minC[i], ref.minR[i])
	return "DIVERGENCE CONDITION DEFECT", i, detail
}

func extremaNotes(rsx []float64, xbars int) (eqMax, eqMin, n100, n0 int) {
	for _, v := range rsx {
		if v == 100 {
			n100++
		}
		if v == 0 {
			n0++
		}
	}
	for i := range rsx {
		start := i - xbars + 1
		if start < 0 {
			start = 0
		}
		mx, mn := rsx[start], rsx[start]
		cMax, cMin := 0, 0
		for j := start; j <= i; j++ {
			if rsx[j] > mx {
				mx = rsx[j]
			}
			if rsx[j] < mn {
				mn = rsx[j]
			}
		}
		for j := start; j <= i; j++ {
			if rsx[j] == mx {
				cMax++
			}
			if rsx[j] == mn {
				cMin++
			}
		}
		if cMax > 1 {
			eqMax++
		}
		if cMin > 1 {
			eqMin++
		}
	}
	return
}

func loadProbeCSV(t *testing.T, path string) []probeBar {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open prefix csv: %v", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv: %v", err)
	}
	if len(rows) < 2 {
		t.Fatal("empty csv")
	}
	out := make([]probeBar, 0, len(rows)-1)
	for i, row := range rows[1:] {
		if len(row) < 5 {
			t.Fatalf("row %d", i)
		}
		ot, _ := strconv.ParseInt(row[0], 10, 64)
		o, _ := strconv.ParseFloat(row[1], 64)
		h, _ := strconv.ParseFloat(row[2], 64)
		l, _ := strconv.ParseFloat(row[3], 64)
		c, _ := strconv.ParseFloat(row[4], 64)
		out = append(out, probeBar{openTime: ot, o: o, h: h, l: l, c: c})
	}
	return out
}
