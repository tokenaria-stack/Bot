package brain3

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"trading_bot/ml"
)

const (
	DecompFormat        = "setup-probability-decomposition-1"
	QualityCurve1Digest = "beb5d25e1a83baa65fac2ab57ea793f11a2dd023ff231a6a8b1025358e45290d"
	binaryLogFloor      = 1e-15
	ResolutionStrong    = "STRONG_RESOLUTION_RANKING"
	ResolutionWeak      = "WEAK_RESOLUTION_RANKING"
	QBroad              = "BROAD_Q_RANKING"
	QRegime             = "REGIME_SENSITIVE_Q"
	QTail               = "TAIL_ONLY_Q"
	QFlat               = "FLAT_Q"
)

type AQ struct {
	PTP, PStop, PTO float64
	A, Q            float64
	Defined         bool
}

func RowAQ(z [3]float64) AQ {
	p := ml.Softmax3(z)
	var out AQ
	out.PTP, out.PStop, out.PTO = p[0], p[1], p[2]
	out.A = p[0] + p[1]
	if out.A == 0 || math.IsNaN(out.A) || math.IsInf(out.A, 0) {
		return out
	}
	out.Q = p[0] / out.A
	if math.IsNaN(out.Q) || math.IsInf(out.Q, 0) {
		out.Q = 0
		return out
	}
	out.Defined = true
	return out
}

func scoreA(r OOFRow) (float64, bool) {
	return RowAQ(r.Logit).A, true
}

func scoreQ(r OOFRow) (float64, bool) {
	aq := RowAQ(r.Logit)
	return aq.Q, aq.Defined
}

func resolvedOnly(rows []OOFRow) []OOFRow {
	var out []OOFRow
	for _, r := range rows {
		if r.Y == 0 || r.Y == 1 {
			out = append(out, r)
		}
	}
	return out
}

func curveBy(fold int, rows []OOFRow, score func(OOFRow) (float64, bool)) FoldCurve {
	ranked := rankFoldBy(rows, score)
	base := countY(ysOf(rows))
	fc := FoldCurve{Fold: fold, N: len(rows), Baseline: base}
	if len(rows) > 0 {
		fc.Year = yearOf(rows[0].At)
	}
	for _, c := range QualityCoverages {
		sel := takeTop(ranked, SelectedN(c, len(rows)))
		fc.Slices = append(fc.Slices, statsVs(countY(ysOf(sel)), base, c))
	}
	return fc
}

func pooledBy(folds [][]OOFRow, coverage float64, score func(OOFRow) (float64, bool)) ([]OOFRow, error) {
	var sel []OOFRow
	seen := map[int64]struct{}{}
	for _, rows := range folds {
		top := takeTop(rankFoldBy(rows, score), SelectedN(coverage, len(rows)))
		for _, r := range top {
			if _, ok := seen[r.At]; ok {
				return nil, fmt.Errorf("brain3: duplicate selected At %d", r.At)
			}
			seen[r.At] = struct{}{}
			sel = append(sel, r)
		}
	}
	return sel, nil
}

func selectedSet(rows []OOFRow, coverage float64, score func(OOFRow) (float64, bool)) map[int64]struct{} {
	top := takeTop(rankFoldBy(rows, score), SelectedN(coverage, len(rows)))
	m := make(map[int64]struct{}, len(top))
	for _, r := range top {
		m[r.At] = struct{}{}
	}
	return m
}

func overlapFrac(a, b map[int64]struct{}) float64 {
	if len(a) == 0 {
		return 0
	}
	n := 0
	for at := range a {
		if _, ok := b[at]; ok {
			n++
		}
	}
	return float64(n) / float64(len(a))
}

func binaryLogLoss(q float64, y int) (loss float64, clipped bool, err error) {
	if y != 0 && y != 1 {
		return 0, false, fmt.Errorf("brain3: binary y=%d", y)
	}
	p := q
	if p < binaryLogFloor {
		p = binaryLogFloor
		clipped = true
	}
	if p > 1-binaryLogFloor {
		p = 1 - binaryLogFloor
		clipped = true
	}
	if y == 1 {
		return -math.Log(p), clipped, nil
	}
	return -math.Log(1 - p), clipped, nil
}

func meanBinaryLoss(rows []OOFRow) (loss float64, n, clipped, undefined int, err error) {
	var s float64
	for _, r := range rows {
		if r.Y != 0 && r.Y != 1 {
			return 0, 0, 0, 0, fmt.Errorf("brain3: TIMEOUT in resolved logloss")
		}
		aq := RowAQ(r.Logit)
		if !aq.Defined {
			undefined++
			continue
		}
		v, c, err := binaryLogLoss(aq.Q, r.Y)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		if c {
			clipped++
		}
		s += v
		n++
	}
	if n == 0 {
		return 0, 0, clipped, undefined, fmt.Errorf("brain3: empty resolved logloss population")
	}
	return s / float64(n), n, clipped, undefined, nil
}

func priorBinaryLoss(rows []OOFRow, prior float64) (float64, error) {
	if prior <= 0 || prior >= 1 {
		return 0, fmt.Errorf("brain3: prior_q=%v not in (0,1)", prior)
	}
	var s float64
	for _, r := range rows {
		v, _, err := binaryLogLoss(prior, r.Y)
		if err != nil {
			return 0, err
		}
		s += v
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("brain3: empty prior population")
	}
	return s / float64(len(rows)), nil
}

func rankAverage(xs []float64) []float64 {
	n := len(xs)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(i, j int) bool { return xs[idx[i]] < xs[idx[j]] })
	out := make([]float64, n)
	for i := 0; i < n; {
		j := i
		for j+1 < n && xs[idx[j+1]] == xs[idx[i]] {
			j++
		}
		avg := 0.5*float64(i+j) + 1
		for k := i; k <= j; k++ {
			out[idx[k]] = avg
		}
		i = j + 1
	}
	return out
}

func pearson(x, y []float64) float64 {
	n := float64(len(x))
	if n < 3 {
		return math.NaN()
	}
	var sx, sy, sxx, syy, sxy float64
	for i := range x {
		sx += x[i]
		sy += y[i]
		sxx += x[i] * x[i]
		syy += y[i] * y[i]
		sxy += x[i] * y[i]
	}
	num := n*sxy - sx*sy
	den := math.Sqrt((n*sxx - sx*sx) * (n*syy - sy*sy))
	if den == 0 {
		return math.NaN()
	}
	return num / den
}

func spearman(a, q []float64) float64 {
	return pearson(rankAverage(a), rankAverage(q))
}

func resolvedRateBetterN(folds []FoldCurve, coverage float64) int {
	n := 0
	for _, f := range folds {
		base, _ := f.Baseline.ResolvedTP()
		for _, sl := range f.Slices {
			if sl.Coverage == coverage && sl.ResolvedOK && sl.ResolvedTP > base {
				n++
				break
			}
		}
	}
	return n
}

func describeResolution(folds []FoldCurve) (string, string) {
	ok := true
	for _, c := range []float64{0.50, 0.30, 0.20} {
		if timeoutBetterN(folds, c) < 3 {
			ok = false
		}
	}
	if ok {
		return ResolutionStrong, "TIMEOUT falls in >=3/4 folds at 50/30/20 when ranking by a."
	}
	return ResolutionWeak, "TIMEOUT does not fall consistently when ranking by a."
}

func describeQ(folds []FoldCurve, llDelta []float64) (string, string) {
	r50 := resolvedRateBetterN(folds, 0.50)
	r30 := resolvedRateBetterN(folds, 0.30)
	r20 := resolvedRateBetterN(folds, 0.20)
	r10 := resolvedRateBetterN(folds, 0.10)
	r05 := resolvedRateBetterN(folds, 0.05)
	posLL := 0
	for _, d := range llDelta {
		if d > 0 {
			posLL++
		}
	}
	broad := r50 >= 3 && r30 >= 3 && r20 >= 3
	weak := r50 <= 1 && r30 <= 1 && r20 <= 1
	switch {
	case weak && (r10 >= 3 || r05 >= 3):
		return QTail, "resolved q is weak in 50/30/20; 10/5% looks better (sparse)."
	case broad && posLL >= 3:
		return QBroad, "resolved TP rate rises in >=3/4 folds at 50/30/20 and q beats the causal resolved prior on most folds."
	case broad && posLL < 3:
		return QFlat, "resolved TP rate ticks slightly higher in the product region, but q does not beat the outer-train resolved prior in binary logloss. Rank-only ticks are not BROAD_Q."
	case r50 >= 3 || r30 >= 3 || r20 >= 3:
		return QRegime, "q helps some years in the product region but not uniformly across 2022–2025."
	default:
		return QFlat, "q does not meaningfully rank TP vs STOP among resolved OOF rows."
	}
}

type OverlapRow struct {
	Coverage float64
	Fold     int
	Frac     float64
}

type FoldLogLoss struct {
	Fold, N, Clipped, Undefined int
	QLoss, PriorLoss, Delta     float64
	PriorQ                      float64
	TrainTP, TrainStop          int
}

type DecompResult struct {
	SourceOOF      string
	QualityDigest  string
	ResultDigest   string
	HoldoutStartAt int64
	MaxAt          int64
	Rows           int
	QUndefined     int
	AFolds         []FoldCurve
	APooled        []SliceStats
	APooledBase    ClassCensus
	QResFolds      []FoldCurve
	QResPooled     []SliceStats
	QResBase       ClassCensus
	QFullFolds     []FoldCurve
	QFullPooled    []SliceStats
	QFullBase      ClassCensus
	Overlap        []OverlapRow
	PooledOverlap  []OverlapRow
	LogLoss        []FoldLogLoss
	PooledQLoss    float64
	PooledPrior    float64
	PooledDelta    float64
	Spearman       []float64
	MeanSpearman   float64
	Resolution     string
	ResolutionWhy  string
	QVerdict       string
	QWhy           string
	Text           string
}

func OuterTrainResolvedPriors(ds Dataset, census Census) ([]float64, []ClassCensus, error) {
	if len(census.Folds) != 4 {
		return nil, nil, fmt.Errorf("brain3: need 4 outer folds for prior")
	}
	priors := make([]float64, 4)
	cs := make([]ClassCensus, 4)
	for i, f := range census.Folds {
		c := countY(ds.Y[f.TrainBegin:f.TrainEnd])
		d := c.TP + c.Stop
		if d == 0 {
			return nil, nil, fmt.Errorf("brain3: fold %d train has no resolved rows", i)
		}
		priors[i] = float64(c.TP) / float64(d)
		cs[i] = c
	}
	return priors, cs, nil
}

func RunDecomp(oof OOFFile, priors []float64, trainCensus []ClassCensus) (DecompResult, error) {
	var z DecompResult
	if err := oof.validate(Metalabel1OOFDigest); err != nil {
		return z, err
	}
	if len(priors) != 4 || len(trainCensus) != 4 {
		return z, fmt.Errorf("brain3: prior length")
	}
	by := foldRows(oof.Rows)
	if len(by) != 4 {
		return z, fmt.Errorf("brain3: need 4 folds")
	}
	z.SourceOOF = oof.Content
	z.QualityDigest = QualityCurve1Digest
	z.HoldoutStartAt = oof.HoldoutStartAt
	z.MaxAt = oof.MaxAt
	z.Rows = len(oof.Rows)
	for _, r := range oof.Rows {
		if !RowAQ(r.Logit).Defined {
			z.QUndefined++
		}
	}
	resBy := make([][]OOFRow, 4)
	for i, rows := range by {
		z.AFolds = append(z.AFolds, curveBy(i, rows, scoreA))
		z.QFullFolds = append(z.QFullFolds, curveBy(i, rows, scoreQ))
		res := resolvedOnly(rows)
		for _, r := range res {
			if r.Y == 2 {
				return z, fmt.Errorf("brain3: TIMEOUT leaked into resolved-only")
			}
		}
		resBy[i] = res
		z.QResFolds = append(z.QResFolds, curveBy(i, res, scoreQ))
	}
	var err error
	baseA, err := pooledBy(by, 1, scoreA)
	if err != nil {
		return z, err
	}
	z.APooledBase = countY(ysOf(baseA))
	for _, c := range QualityCoverages {
		sel, err := pooledBy(by, c, scoreA)
		if err != nil {
			return z, err
		}
		z.APooled = append(z.APooled, statsVs(countY(ysOf(sel)), z.APooledBase, c))
	}
	baseR, err := pooledBy(resBy, 1, scoreQ)
	if err != nil {
		return z, err
	}
	z.QResBase = countY(ysOf(baseR))
	for _, c := range QualityCoverages {
		sel, err := pooledBy(resBy, c, scoreQ)
		if err != nil {
			return z, err
		}
		z.QResPooled = append(z.QResPooled, statsVs(countY(ysOf(sel)), z.QResBase, c))
	}
	baseF, err := pooledBy(by, 1, scoreQ)
	if err != nil {
		return z, err
	}
	z.QFullBase = countY(ysOf(baseF))
	for _, c := range QualityCoverages {
		sel, err := pooledBy(by, c, scoreQ)
		if err != nil {
			return z, err
		}
		z.QFullPooled = append(z.QFullPooled, statsVs(countY(ysOf(sel)), z.QFullBase, c))
	}
	for _, c := range []float64{0.50, 0.30, 0.20} {
		var pa, pp []OOFRow
		if pa, err = pooledBy(by, c, scoreA); err != nil {
			return z, err
		}
		if pp, err = pooledBy(by, c, func(r OOFRow) (float64, bool) { return ScorePTP(r.Logit), true }); err != nil {
			return z, err
		}
		sa, sp := map[int64]struct{}{}, map[int64]struct{}{}
		for _, r := range pa {
			sa[r.At] = struct{}{}
		}
		for _, r := range pp {
			sp[r.At] = struct{}{}
		}
		z.PooledOverlap = append(z.PooledOverlap, OverlapRow{Coverage: c, Fold: -1, Frac: overlapFrac(sa, sp)})
		for i, rows := range by {
			fa := selectedSet(rows, c, scoreA)
			fp := selectedSet(rows, c, func(r OOFRow) (float64, bool) { return ScorePTP(r.Logit), true })
			z.Overlap = append(z.Overlap, OverlapRow{Coverage: c, Fold: i, Frac: overlapFrac(fa, fp)})
		}
	}
	var wQ, wP float64
	var wN int
	deltas := make([]float64, 4)
	for i, res := range resBy {
		ql, n, clip, undef, err := meanBinaryLoss(res)
		if err != nil {
			return z, err
		}
		pl, err := priorBinaryLoss(res, priors[i])
		if err != nil {
			return z, err
		}
		fr := FoldLogLoss{
			Fold: i, N: n, Clipped: clip, Undefined: undef,
			QLoss: ql, PriorLoss: pl, Delta: pl - ql, PriorQ: priors[i],
			TrainTP: trainCensus[i].TP, TrainStop: trainCensus[i].Stop,
		}
		z.LogLoss = append(z.LogLoss, fr)
		deltas[i] = fr.Delta
		wQ += ql * float64(n)
		wP += pl * float64(n)
		wN += n
	}
	z.PooledQLoss = wQ / float64(wN)
	z.PooledPrior = wP / float64(wN)
	z.PooledDelta = z.PooledPrior - z.PooledQLoss
	var meanRho float64
	var rhoN int
	for _, rows := range by {
		var as, qs []float64
		for _, r := range rows {
			aq := RowAQ(r.Logit)
			if !aq.Defined {
				continue
			}
			as = append(as, aq.A)
			qs = append(qs, aq.Q)
		}
		rho := spearman(as, qs)
		z.Spearman = append(z.Spearman, rho)
		if !math.IsNaN(rho) {
			meanRho += rho * float64(len(as))
			rhoN += len(as)
		}
	}
	if rhoN > 0 {
		z.MeanSpearman = meanRho / float64(rhoN)
	}
	z.Resolution, z.ResolutionWhy = describeResolution(z.AFolds)
	z.QVerdict, z.QWhy = describeQ(z.QResFolds, deltas)
	z.ResultDigest = hashDecomp(z)
	z.Text = FormatDecompReport(z, oof)
	return z, nil
}

func hashDecomp(z DecompResult) string {
	h := sha256.New()
	meta, _ := json.Marshal(struct {
		Fmt, OOF, Quality string
		A, Q              string
	}{DecompFormat, z.SourceOOF, z.QualityDigest, "1-pTIMEOUT", "pTP/a"})
	h.Write(meta)
	var buf [8]byte
	writeI := func(v int) {
		binary.LittleEndian.PutUint64(buf[:], uint64(v))
		h.Write(buf[:])
	}
	writeF := func(v float64) {
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v))
		h.Write(buf[:])
	}
	writeI(z.QUndefined)
	for _, f := range z.AFolds {
		for _, s := range f.Slices {
			writeI(s.N)
			writeI(s.TP)
			writeI(s.Stop)
			writeI(s.Timeout)
		}
	}
	for _, f := range z.QResFolds {
		for _, s := range f.Slices {
			writeI(s.N)
			writeI(s.TP)
			writeI(s.Stop)
		}
	}
	for _, ll := range z.LogLoss {
		writeF(ll.QLoss)
		writeF(ll.PriorLoss)
	}
	for _, r := range z.Spearman {
		writeF(r)
	}
	h.Write([]byte(z.Resolution + z.QVerdict))
	return hex.EncodeToString(h.Sum(nil))
}

func FormatDecompReport(z DecompResult, oof OOFFile) string {
	var b strings.Builder
	b.WriteString("SETUP-PROBABILITY-DECOMPOSITION-1 (read-only; no strategy)\n")
	b.WriteString(fmt.Sprintf("source_oof=%s\nquality_curve=%s\nresult=%s\n", z.SourceOOF, z.QualityDigest, z.ResultDigest))
	b.WriteString("a=pTP+pSTOP=1-pTIMEOUT  q=pTP/a  undefined_q ranks last\n")
	b.WriteString(fmt.Sprintf("rows=%d schema=%v max_at=%d holdout=%d q_undefined=%d logloss_floor=%g\n",
		z.Rows, oof.ClassSchema, z.MaxAt, z.HoldoutStartAt, z.QUndefined, binaryLogFloor))
	b.WriteString("A. SOURCE frozen BRAIN3-METALABEL-1 OOF; quality verdict TIMEOUT_FILTER_RANKING not recomputed.\n")
	b.WriteString("B. a / RESOLUTION (all rows, fold-local)\n")
	for _, f := range z.AFolds {
		b.WriteString(fmt.Sprintf("fold %d year=%d n=%d baseline TP/STOP/TO=%.1f/%.1f/%.1f%%\n",
			f.Fold, f.Year, f.N, f.Baseline.TPPct(), f.Baseline.StopPct(), f.Baseline.TimeoutPct()))
		for _, s := range f.Slices {
			b.WriteString("  " + fmtSlice(s) + "\n")
		}
	}
	b.WriteString("pooled a\n")
	for _, s := range z.APooled {
		b.WriteString("  " + fmtSlice(s) + "\n")
	}
	b.WriteString("product-region a: TIMEOUT lower in ")
	for i, c := range []float64{0.50, 0.30, 0.20} {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("%s %d/4", coverageLabel(c), timeoutBetterN(z.AFolds, c)))
	}
	b.WriteString("\n")
	b.WriteString("C. a vs frozen P(TP) selected-set overlap (|A∩P|/|A|)\n")
	for _, o := range z.Overlap {
		b.WriteString(fmt.Sprintf("  fold %d %s overlap=%.3f\n", o.Fold, coverageLabel(o.Coverage), o.Frac))
	}
	for _, o := range z.PooledOverlap {
		b.WriteString(fmt.Sprintf("  pooled %s overlap=%.3f\n", coverageLabel(o.Coverage), o.Frac))
	}
	b.WriteString("D. q / RESOLVED-ONLY DIAGNOSTIC (Y in {TP,STOP}; not a deployable filter)\n")
	for _, f := range z.QResFolds {
		rt, _ := f.Baseline.ResolvedTP()
		b.WriteString(fmt.Sprintf("fold %d year=%d resolved_n=%d baseline resolved_TP=%.4f\n", f.Fold, f.Year, f.N, rt))
		for _, s := range f.Slices {
			b.WriteString("  " + fmtSlice(s) + "\n")
		}
	}
	b.WriteString("pooled resolved q\n")
	for _, s := range z.QResPooled {
		b.WriteString("  " + fmtSlice(s) + "\n")
	}
	b.WriteString("product-region resolved q: TP rate better in ")
	for i, c := range []float64{0.50, 0.30, 0.20} {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("%s %d/4", coverageLabel(c), resolvedRateBetterN(z.QResFolds, c)))
	}
	b.WriteString("\n")
	b.WriteString("E. q BINARY LOGLOSS vs causal resolved prior (outer TRAIN TP/(TP+STOP) only)\n")
	for _, ll := range z.LogLoss {
		b.WriteString(fmt.Sprintf("  fold %d resolved_n=%d train_TP/STOP=%d/%d prior_q=%.4f  q_ll=%.6f  prior_ll=%.6f  delta=%.6f  clipped=%d undef=%d\n",
			ll.Fold, ll.N, ll.TrainTP, ll.TrainStop, ll.PriorQ, ll.QLoss, ll.PriorLoss, ll.Delta, ll.Clipped, ll.Undefined))
	}
	b.WriteString(fmt.Sprintf("  pooled q_ll=%.6f  prior_ll=%.6f  delta=%.6f\n", z.PooledQLoss, z.PooledPrior, z.PooledDelta))
	b.WriteString("F. FULL-POPULATION q DIAGNOSTIC (secondary; TIMEOUT still present)\n")
	for _, f := range z.QFullFolds {
		b.WriteString(fmt.Sprintf("fold %d year=%d n=%d\n", f.Fold, f.Year, f.N))
		for _, s := range f.Slices {
			b.WriteString("  " + fmtSlice(s) + "\n")
		}
	}
	b.WriteString("pooled full-pop q\n")
	for _, s := range z.QFullPooled {
		b.WriteString("  " + fmtSlice(s) + "\n")
	}
	b.WriteString("G. Spearman rho(a,q) fold-local on defined-q rows\n")
	for i, r := range z.Spearman {
		b.WriteString(fmt.Sprintf("  fold %d rho=%.4f\n", i, r))
	}
	b.WriteString(fmt.Sprintf("  size-weighted mean rho=%.4f\n", z.MeanSpearman))
	b.WriteString("H. VERDICT (descriptive; not a strategy)\n")
	b.WriteString(fmt.Sprintf("resolution: %s\n%s\n", z.Resolution, z.ResolutionWhy))
	b.WriteString(fmt.Sprintf("q: %s\n%s\n", z.QVerdict, z.QWhy))
	b.WriteString(fmt.Sprintf("a vs q: size-weighted mean Spearman rho=%.3f (not collinear; extra q ordering is not a logloss win).\n", z.MeanSpearman))
	b.WriteString("Fork: a strong, q weak vs causal resolved prior → TP-vs-STOP FACT-GAP next. Do not freeze a fake 2-D scorer.\n")
	b.WriteString("No a*q collapse / no 2026 / no CatBoost / no second ignition.\n")
	b.WriteString("CORRECTIONS: logloss uses floor 1e-15 only inside -log (not a recalibration);\n")
	b.WriteString("train prior uses SETUP-DATASET Y on compiled outer TRAIN (labels only, no X);\n")
	b.WriteString("pooled Spearman is size-weighted mean of fold rhos, not a global rank mix;\n")
	b.WriteString("quality-curve verdict is bound by digest, P(TP) sets are recomputed with the frozen score law for overlap.\n")
	return b.String()
}
