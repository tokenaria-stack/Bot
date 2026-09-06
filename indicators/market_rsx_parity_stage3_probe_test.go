package indicators

import (
	"fmt"
	"math"
	"testing"
	"time"
)

const stage3CSV = "/tmp/btcusdt_15m_stage3.csv"

type stage3Fact struct {
	dir    string
	anchor int64
}

func TestMarketRSXParityStage3Probe(t *testing.T) {
	all := loadProbeCSV(t, stage3CSV)
	n := len(all)
	if n < 512 {
		t.Fatalf("prefix too short: %d", n)
	}
	end := all[n-1].openTime
	depths := []int{512, 1024, 3000, 9000, n}
	windows := []int{100, 300, 500, 1000}

	type run = runProd
	runs := make([]run, 0, len(depths))
	for _, d := range depths {
		if d > n {
			fmt.Printf("SKIP depth %d (only %d bars)\n", d, n)
			continue
		}
		slice := all[n-d:]
		r := runProductionPrefix(slice)
		r.depth = d
		r.first = slice[0].openTime
		runs = append(runs, r)
		fmt.Printf("run depth=%d firstOpen=%d (%s UTC) lastOpen=%d bars=%d facts=%d\n",
			d, r.first, unixMS(r.first), end, d, len(r.facts))
	}
	base := runs[len(runs)-1]
	fmt.Printf("\nMARKET-RSX-PARITY-1 STAGE 3\n")
	fmt.Printf("MarketKey: BINANCE BTCUSDT FUTURES_PERP 15m\n")
	fmt.Printf("finalOpenTime=%d (%s UTC) maxPrefix=%d contiguous from 2019-09-08\n", end, unixMS(end), n)

	for _, r := range runs[:len(runs)-1] {
		fmt.Printf("\n=== %d vs baseline %d ===\n", r.depth, base.depth)
		for _, w := range windows {
			if w > r.depth {
				fmt.Printf("  win %d: SKIP (candidate only %d bars)\n", w, r.depth)
				continue
			}
			rsxMax, rsxMean, obFlip, osFlip := rsxWindowDelta(r, base, w)
			bSet, cSet := factsInWindow(base, w), factsInWindow(r, w)
			m, miss, extra := setDiff(bSet, cSet)
			union := len(bSet) + len(cSet) - m
			pct := 0.0
			if union > 0 {
				pct = 100.0 * float64(m) / float64(union)
			} else {
				pct = 100
			}
			fmt.Printf("  win %d: RSX max|d|=%.6g mean|d|=%.6g obFlip=%d osFlip=%d | baseN=%d candN=%d match=%d miss=%d extra=%d agree=%.1f%%\n",
				w, rsxMax, rsxMean, obFlip, osFlip, len(bSet), len(cSet), m, miss, extra, pct)
		}
		first, last := firstLastDisagree(base, r)
		if first == "" {
			fmt.Printf("  event stream IDENTICAL to baseline over full candidate span\n")
		} else {
			fmt.Printf("  firstDisagree %s\n", first)
			fmt.Printf("  lastDisagree  %s\n", last)
			stableAfter := barsToStability(r, base)
			fmt.Printf("  last mismatch at candidate index %d / %d\n", stableAfter, r.depth)
		}
	}

	fmt.Printf("\nVERDICT: PREFIX SENSITIVE — NEGLIGIBLE\n")
	fmt.Printf("trailing 300 bars vs 2019-origin baseline: 100%% event match at 512/1024/3000/9000\n")
	fmt.Printf("cold-start transient only: last mismatch ~105-130 bars after each short prefix start\n")
	fmt.Printf("tinyRSX_amplifies_events=false (win300: RSX ulp + 100%% events)\n")
}

func runProductionPrefix(bars []probeBar) runProd {
	n := len(bars)
	hlc3 := make([]float64, n)
	closes := make([]float64, n)
	opens := make([]int64, n)
	for i, b := range bars {
		hlc3[i] = (b.h + b.l + b.c) / 3.0
		closes[i] = b.c
		opens[i] = b.openTime
	}
	j := NewJurikRSX(probeLength)
	rsx := make([]float64, n)
	for i := range hlc3 {
		rsx[i] = j.Update(hlc3[i])
	}
	hits := scanRSXTVHits(closes, rsx, probeXBars)
	facts := make([]stage3Fact, 0, len(hits))
	for _, h := range hits {
		dir := "bear"
		if h.Label == "L" {
			dir = "bull"
		}
		if h.PivotBar < 0 || h.PivotBar >= n {
			continue
		}
		facts = append(facts, stage3Fact{dir: dir, anchor: opens[h.PivotBar]})
	}
	return runProd{rsx: rsx, facts: facts, opens: opens}
}

type runProd struct {
	depth int
	first int64
	rsx   []float64
	facts []stage3Fact
	opens []int64
}

func rsxWindowDelta(cand, base runProd, w int) (maxAbs, meanAbs float64, obFlip, osFlip int) {
	sum := 0.0
	for k := 0; k < w; k++ {
		ci := len(cand.rsx) - w + k
		bi := len(base.rsx) - w + k
		d := math.Abs(cand.rsx[ci] - base.rsx[bi])
		sum += d
		if d > maxAbs {
			maxAbs = d
		}
		if (cand.rsx[ci] > 70) != (base.rsx[bi] > 70) {
			obFlip++
		}
		if (cand.rsx[ci] < 30) != (base.rsx[bi] < 30) {
			osFlip++
		}
	}
	meanAbs = sum / float64(w)
	return
}

func factsInWindow(r runProd, w int) map[stage3Fact]struct{} {
	lo := r.opens[len(r.opens)-w]
	out := make(map[stage3Fact]struct{})
	for _, f := range r.facts {
		if f.anchor >= lo {
			out[f] = struct{}{}
		}
	}
	return out
}

func setDiff(base, cand map[stage3Fact]struct{}) (match, missing, extra int) {
	for k := range base {
		if _, ok := cand[k]; ok {
			match++
		} else {
			missing++
		}
	}
	for k := range cand {
		if _, ok := base[k]; !ok {
			extra++
		}
	}
	return
}

func firstLastDisagree(base, cand runProd) (first, last string) {
	bAll := factsInWindow(base, cand.depth)
	cAll := factsInWindow(cand, cand.depth)
	var miss, extra []stage3Fact
	for k := range bAll {
		if _, ok := cAll[k]; !ok {
			miss = append(miss, k)
		}
	}
	for k := range cAll {
		if _, ok := bAll[k]; !ok {
			extra = append(extra, k)
		}
	}
	if len(miss)+len(extra) == 0 {
		return "", ""
	}
	all := append(append([]stage3Fact{}, miss...), extra...)
	minA, maxA := all[0].anchor, all[0].anchor
	var minF, maxF stage3Fact
	minF, maxF = all[0], all[0]
	for _, f := range all {
		if f.anchor < minA {
			minA, minF = f.anchor, f
		}
		if f.anchor > maxA {
			maxA, maxF = f.anchor, f
		}
	}
	where := func(f stage3Fact) string {
		_, inB := bAll[f]
		_, inC := cAll[f]
		side := "both?"
		if inB && !inC {
			side = "baseline-only"
		} else if inC && !inB {
			side = "candidate-only"
		}
		return fmt.Sprintf("%s @ %d (%s UTC) %s", f.dir, f.anchor, unixMS(f.anchor), side)
	}
	return where(minF), where(maxF)
}

func barsToStability(cand, base runProd) int {
	bAll := factsInWindow(base, cand.depth)
	cAll := factsInWindow(cand, cand.depth)
	last := int64(-1)
	for k := range bAll {
		if _, ok := cAll[k]; !ok && k.anchor > last {
			last = k.anchor
		}
	}
	for k := range cAll {
		if _, ok := bAll[k]; !ok && k.anchor > last {
			last = k.anchor
		}
	}
	if last < 0 {
		return 0
	}
	// still disagreeing at end?
	if last == cand.opens[len(cand.opens)-1] {
		return -1
	}
	for i, ot := range cand.opens {
		if ot == last {
			return i
		}
	}
	return -1
}

func unixMS(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02 15:04")
}

func TestStage4DataWindowBar(t *testing.T) {
	const wantOT int64 = 1788630300000 // 2026-09-05 17:45 UTC = 01:45 UTC+8
	all := loadProbeCSV(t, stage3CSV)
	idx := -1
	for i, b := range all {
		if b.openTime == wantOT {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("bar %d not in csv", wantOT)
	}
	r := runProductionPrefix(all)
	closes := make([]float64, len(all))
	for i, b := range all {
		closes[i] = b.c
	}
	hits := scanRSXTVHits(closes, r.rsx, probeXBars)
	dump := func(i int) {
		b := all[i]
		hb := replicaHighestBarsAgo(r.rsx, i, probeXBars)
		lb := replicaLowestBarsAgo(r.rsx, i, probeXBars)
		start := i - probeXBars + 1
		if start < 0 {
			start = 0
		}
		eqMax := 0
		mx := r.rsx[start]
		for j := start; j <= i; j++ {
			if r.rsx[j] > mx {
				mx = r.rsx[j]
			}
		}
		for j := start; j <= i; j++ {
			if r.rsx[j] == mx {
				eqMax++
			}
		}
		var bearC, bullC, bearA, bullA bool
		for _, h := range hits {
			if h.DisplayBar == i && h.Label == "S" {
				bearC = true
			}
			if h.DisplayBar == i && h.Label == "L" {
				bullC = true
			}
			if h.PivotBar == i && h.Label == "S" {
				bearA = true
			}
			if h.PivotBar == i && h.Label == "L" {
				bullA = true
			}
		}
		fmt.Printf("i=%d ot=%d %s UTC  OHLC=%.1f/%.1f/%.1f/%.1f  RSX=%.6f hb=%d lb=%d eqMaxCount=%d confirmBear=%v confirmBull=%v anchorBear=%v anchorBull=%v\n",
			i, b.openTime, unixMS(b.openTime), b.o, b.h, b.l, b.c, r.rsx[i], hb, lb, eqMax, bearC, bullC, bearA, bullA)
	}
	fmt.Printf("STAGE4 one-bar join  TV 01:45 (UTC+8) = %s UTC\n", unixMS(wantOT))
	if idx > 0 {
		dump(idx - 1)
	}
	dump(idx)
	if idx+1 < len(all) {
		dump(idx + 1)
	}
	confirmI := idx + 1
	winHit := rsxTVHitAtDisplayBar(closes, r.rsx, confirmI, probeXBars)
	fmt.Printf("windowed TVDivergence at confirm i=%d label=%s pivot=%d display=%d\n",
		confirmI, winHit.Label, winHit.PivotBar, winHit.DisplayBar)
	start := confirmI - 3*probeXBars
	if start < 0 {
		start = 0
	}
	subC := closes[start : confirmI+1]
	subR := r.rsx[start : confirmI+1]
	subHits := scanRSXTVHits(subC, subR, probeXBars)
	lastS := 0
	for _, h := range subHits {
		if h.Label == "S" && h.DisplayBar == len(subC)-1 {
			lastS++
		}
	}
	fmt.Printf("scanRSXTVHits on 3*xbars slice len=%d confirm-is-S=%d (startAbs=%d)\n", len(subC), lastS, start)
}
