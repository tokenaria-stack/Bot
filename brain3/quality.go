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
	"time"

	"trading_bot/ml"
)

const (
	QualityCurveFormat        = "setup-quality-curve-1"
	QualityScoreLaw           = "softmax_p_tp_first"
	QualityTieLaw             = "score_desc_at_asc"
	QualityRankingLaw         = "fold_local_percentile"
	QualityVerdictBroad       = "BROAD_STABLE_RANKING"
	QualityVerdictRegime      = "REGIME_SENSITIVE_RANKING"
	QualityVerdictTail        = "TAIL_ONLY_RANKING"
	QualityVerdictFlat        = "FLAT_RANKING"
	QualityVerdictTimeoutFilt = "TIMEOUT_FILTER_RANKING"
)

var QualityCoverages = []float64{1.0, 0.75, 0.50, 0.30, 0.20, 0.10, 0.05}

type ClassCensus struct {
	N, TP, Stop, Timeout int
}

func (c ClassCensus) pct(n int) float64 {
	if c.N == 0 {
		return 0
	}
	return 100 * float64(n) / float64(c.N)
}

func (c ClassCensus) TPPct() float64      { return c.pct(c.TP) }
func (c ClassCensus) StopPct() float64    { return c.pct(c.Stop) }
func (c ClassCensus) TimeoutPct() float64 { return c.pct(c.Timeout) }

func (c ClassCensus) ResolvedTP() (float64, bool) {
	d := c.TP + c.Stop
	if d == 0 {
		return 0, false
	}
	return float64(c.TP) / float64(d), true
}

func countY(ys []int) ClassCensus {
	var c ClassCensus
	c.N = len(ys)
	for _, y := range ys {
		switch y {
		case 0:
			c.TP++
		case 1:
			c.Stop++
		case 2:
			c.Timeout++
		}
	}
	return c
}

type SliceStats struct {
	Coverage                                float64
	N, TP, Stop, Timeout                    int
	TPPct, StopPct, TOPct                   float64
	TPLiftPP, TPRelLift, StopChgPP, TOChgPP float64
	ResolvedTP                              float64
	ResolvedOK                              bool
}

func statsVs(c, base ClassCensus, coverage float64) SliceStats {
	s := SliceStats{
		Coverage: coverage, N: c.N, TP: c.TP, Stop: c.Stop, Timeout: c.Timeout,
		TPPct: c.TPPct(), StopPct: c.StopPct(), TOPct: c.TimeoutPct(),
	}
	s.TPLiftPP = s.TPPct - base.TPPct()
	if base.TPPct() > 0 {
		s.TPRelLift = s.TPLiftPP / base.TPPct()
	}
	s.StopChgPP = s.StopPct - base.StopPct()
	s.TOChgPP = s.TOPct - base.TimeoutPct()
	if r, ok := c.ResolvedTP(); ok {
		s.ResolvedTP, s.ResolvedOK = r, true
	}
	return s
}

func SelectedN(coverage float64, n int) int {
	if n <= 0 {
		return 0
	}
	if coverage >= 1 {
		return n
	}
	k := int(math.Ceil(coverage * float64(n)))
	if k < 1 {
		return 1
	}
	if k > n {
		return n
	}
	return k
}

func ScorePTP(z [3]float64) float64 {
	return ml.Softmax3(z)[0]
}

func rankFold(rows []OOFRow) []OOFRow {
	out := append([]OOFRow(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool {
		si, sj := ScorePTP(out[i].Logit), ScorePTP(out[j].Logit)
		if si != sj {
			return si > sj
		}
		return out[i].At < out[j].At
	})
	return out
}

func takeTop(ranked []OOFRow, n int) []OOFRow {
	if n > len(ranked) {
		n = len(ranked)
	}
	return ranked[:n]
}

func ysOf(rows []OOFRow) []int {
	ys := make([]int, len(rows))
	for i, r := range rows {
		ys[i] = r.Y
	}
	return ys
}

type FoldCurve struct {
	Fold     int
	Year     int
	N        int
	Baseline ClassCensus
	Slices   []SliceStats
}

type Stability struct {
	Coverage    float64
	TPBetterN   int
	StopBetterN int
}

type QualityResult struct {
	SourceDigest   string
	ResultDigest   string
	HoldoutStartAt int64
	MaxAt          int64
	Rows           int
	Folds          []FoldCurve
	Pooled         []SliceStats
	PooledBase     ClassCensus
	Product        []Stability
	Tail           []Stability
	Verdict        string
	VerdictWhy     string
	Text           string
}

func coverageLabel(c float64) string {
	if c >= 1 {
		return "100%"
	}
	return fmt.Sprintf("%g%%", c*100)
}

func yearOf(at int64) int {
	return time.UnixMilli(at).UTC().Year()
}

func curveForFold(fold int, rows []OOFRow) FoldCurve {
	ranked := rankFold(rows)
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

func pooledAt(folds [][]OOFRow, coverage float64) ([]OOFRow, error) {
	var sel []OOFRow
	seen := map[int64]struct{}{}
	for _, rows := range folds {
		ranked := rankFold(rows)
		top := takeTop(ranked, SelectedN(coverage, len(rows)))
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

func stabilityAt(folds []FoldCurve, coverage float64) Stability {
	var s Stability
	s.Coverage = coverage
	for _, f := range folds {
		var sl SliceStats
		for _, x := range f.Slices {
			if x.Coverage == coverage {
				sl = x
				break
			}
		}
		if sl.TPPct > f.Baseline.TPPct() {
			s.TPBetterN++
		}
		if sl.StopPct < f.Baseline.StopPct() {
			s.StopBetterN++
		}
	}
	return s
}

func timeoutBetterN(folds []FoldCurve, coverage float64) int {
	n := 0
	for _, f := range folds {
		for _, sl := range f.Slices {
			if sl.Coverage == coverage && sl.TOChgPP < 0 {
				n++
				break
			}
		}
	}
	return n
}

// Verdict is descriptive, not a strategy gate.
// TIMEOUT_FILTER_RANKING is a correction: GPT's A–D buckets assumed STOP would fall if TP rose.
func describeVerdict(folds []FoldCurve, product, tail []Stability) (string, string) {
	tp50, st50 := product[0].TPBetterN, product[0].StopBetterN
	tp30, st30 := product[1].TPBetterN, product[1].StopBetterN
	tp20, st20 := product[2].TPBetterN, product[2].StopBetterN
	tp10, tp05 := tail[0].TPBetterN, tail[1].TPBetterN
	to50, to30 := timeoutBetterN(folds, 0.50), timeoutBetterN(folds, 0.30)

	f2025 := folds[len(folds)-1]
	rev2025 := 0
	for _, c := range []float64{0.50, 0.30, 0.20} {
		for _, sl := range f2025.Slices {
			if sl.Coverage == c && sl.TPPct < f2025.Baseline.TPPct() && sl.StopPct > f2025.Baseline.StopPct() {
				rev2025++
			}
		}
	}
	broad := tp50 >= 3 && tp30 >= 3 && tp20 >= 3 && st50 >= 3 && st30 >= 3 && st20 >= 3
	productWeak := tp50 <= 1 && tp30 <= 1 && tp20 <= 1
	tailStrong := tp10 >= 3 || tp05 >= 3
	timeoutFilter := tp50 >= 3 && tp30 >= 3 && tp20 >= 3 && st50 <= 1 && st30 <= 1 && to50 >= 3 && to30 >= 3

	switch {
	case productWeak && tailStrong:
		return QualityVerdictTail, "product region (50/30/20) is weak; 10/5% looks better (sparse)."
	case timeoutFilter:
		return QualityVerdictTimeoutFilt, "TP% rises in the product region in all/most folds, but STOP% does not fall. TIMEOUT is stripped from the selected set, so both TP and STOP shares rise. P(TP_FIRST) ranks resolution vs H=72 expiry more than TP vs STOP."
	case rev2025 >= 2 && (tp50 >= 3 || tp30 >= 3):
		return QualityVerdictRegime, "2025 reverses TP and STOP in the product region while earlier years improve."
	case broad && rev2025 == 0:
		return QualityVerdictBroad, "TP up and STOP down in >=3/4 folds at 50/30/20%; 2025 is not reversed."
	case (tp50 >= 3 || tp30 >= 3 || tp20 >= 3) && (tp50 < 4 || tp30 < 4 || tp20 < 4 || rev2025 > 0):
		return QualityVerdictRegime, "ranking helps some years in the product region but is not uniform across 2022–2025."
	default:
		return QualityVerdictFlat, "P(TP_FIRST) does not organize TP/STOP in a broad, consistent way."
	}
}

func RunQualityCurve(oof OOFFile) (QualityResult, error) {
	var z QualityResult
	if err := oof.validate(Metalabel1OOFDigest); err != nil {
		return z, err
	}
	by := foldRows(oof.Rows)
	if len(by) != 4 {
		return z, fmt.Errorf("brain3: need 4 folds")
	}
	z.SourceDigest = oof.Content
	z.HoldoutStartAt = oof.HoldoutStartAt
	z.MaxAt = oof.MaxAt
	z.Rows = len(oof.Rows)
	for i, rows := range by {
		z.Folds = append(z.Folds, curveForFold(i, rows))
	}
	baseSel, err := pooledAt(by, 1.0)
	if err != nil {
		return z, err
	}
	z.PooledBase = countY(ysOf(baseSel))
	for _, c := range QualityCoverages {
		sel, err := pooledAt(by, c)
		if err != nil {
			return z, err
		}
		z.Pooled = append(z.Pooled, statsVs(countY(ysOf(sel)), z.PooledBase, c))
	}
	for _, c := range []float64{0.50, 0.30, 0.20} {
		z.Product = append(z.Product, stabilityAt(z.Folds, c))
	}
	for _, c := range []float64{0.10, 0.05} {
		z.Tail = append(z.Tail, stabilityAt(z.Folds, c))
	}
	z.Verdict, z.VerdictWhy = describeVerdict(z.Folds, z.Product, z.Tail)
	z.ResultDigest = hashQuality(z)
	z.Text = FormatQualityReport(z, oof)
	return z, nil
}

func hashQuality(z QualityResult) string {
	h := sha256.New()
	meta, _ := json.Marshal(struct {
		Fmt, Score, Tie, Rank, Source string
		Coverages                     []float64
	}{QualityCurveFormat, QualityScoreLaw, QualityTieLaw, QualityRankingLaw, z.SourceDigest, QualityCoverages})
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
	for _, f := range z.Folds {
		writeI(f.Fold)
		writeI(f.N)
		for _, s := range f.Slices {
			writeF(s.Coverage)
			writeI(s.N)
			writeI(s.TP)
			writeI(s.Stop)
			writeI(s.Timeout)
		}
	}
	for _, s := range z.Pooled {
		writeF(s.Coverage)
		writeI(s.N)
		writeI(s.TP)
		writeI(s.Stop)
		writeI(s.Timeout)
	}
	h.Write([]byte(z.Verdict))
	return hex.EncodeToString(h.Sum(nil))
}

func fmtSlice(s SliceStats) string {
	res := "n/a"
	if s.ResolvedOK {
		res = fmt.Sprintf("%.4f", s.ResolvedTP)
	}
	return fmt.Sprintf("%6s  n=%4d  TP=%4d (%5.1f%% %+5.1fpp rel%+.2f)  STOP=%4d (%5.1f%% %+5.1fpp)  TO=%4d (%5.1f%% %+5.1fpp)  resolved_tp=%s",
		coverageLabel(s.Coverage), s.N, s.TP, s.TPPct, s.TPLiftPP, s.TPRelLift, s.Stop, s.StopPct, s.StopChgPP, s.Timeout, s.TOPct, s.TOChgPP, res)
}

func FormatQualityReport(z QualityResult, oof OOFFile) string {
	var b strings.Builder
	b.WriteString("SETUP-QUALITY-CURVE-1 (read-only; no strategy)\n")
	b.WriteString(fmt.Sprintf("source_oof=%s\nresult=%s\n", z.SourceDigest, z.ResultDigest))
	b.WriteString(fmt.Sprintf("score=%s  tie=%s  rank=%s\n", QualityScoreLaw, QualityTieLaw, QualityRankingLaw))
	b.WriteString(fmt.Sprintf("rows=%d folds=%d schema=%v max_at=%d holdout=%d\n",
		z.Rows, len(z.Folds), oof.ClassSchema, z.MaxAt, z.HoldoutStartAt))
	b.WriteString("A. SOURCE: frozen BRAIN3-METALABEL-1 brain3-oof-logits-v1\n")
	b.WriteString("B. PER-FOLD (headline = TP% / STOP% / TIMEOUT%; resolved_tp is secondary)\n")
	for _, f := range z.Folds {
		b.WriteString(fmt.Sprintf("fold %d year=%d n=%d baseline TP/STOP/TO=%d/%d/%d (%.1f/%.1f/%.1f%%)\n",
			f.Fold, f.Year, f.N, f.Baseline.TP, f.Baseline.Stop, f.Baseline.Timeout,
			f.Baseline.TPPct(), f.Baseline.StopPct(), f.Baseline.TimeoutPct()))
		for _, s := range f.Slices {
			b.WriteString("  " + fmtSlice(s) + "\n")
		}
	}
	b.WriteString("C. POOLED (fold-local top-X% concatenated; row-weighted)\n")
	b.WriteString(fmt.Sprintf("pooled 100%% n=%d TP/STOP/TO=%d/%d/%d (%.1f/%.1f/%.1f%%)\n",
		z.PooledBase.N, z.PooledBase.TP, z.PooledBase.Stop, z.PooledBase.Timeout,
		z.PooledBase.TPPct(), z.PooledBase.StopPct(), z.PooledBase.TimeoutPct()))
	for _, s := range z.Pooled {
		b.WriteString("  " + fmtSlice(s) + "\n")
	}
	b.WriteString("D. PRODUCT-REGION STABILITY (50/30/20)\n")
	for _, s := range z.Product {
		b.WriteString(fmt.Sprintf("  %s  TP better than fold 100%% in %d/4  STOP lower in %d/4  TIMEOUT lower in %d/4\n",
			coverageLabel(s.Coverage), s.TPBetterN, s.StopBetterN, timeoutBetterN(z.Folds, s.Coverage)))
	}
	b.WriteString("E. TAIL DIAGNOSTIC (10/5; sparse)\n")
	for _, s := range z.Tail {
		b.WriteString(fmt.Sprintf("  %s  TP better in %d/4  STOP lower in %d/4\n",
			coverageLabel(s.Coverage), s.TPBetterN, s.StopBetterN))
	}
	b.WriteString("F. VERDICT (descriptive; not a strategy approval)\n")
	b.WriteString(fmt.Sprintf("%s\n%s\n", z.Verdict, z.VerdictWhy))
	b.WriteString("GPT A–D taxonomy assumed STOP would fall if TP rose; TIMEOUT_FILTER_RANKING is the missing bucket.\n")
	b.WriteString("No calibration / no 2026 / no PnL / no CatBoost retune.\n")
	b.WriteString("CORRECTIONS: resolved_tp is secondary; TIMEOUT stays in the census;\n")
	b.WriteString("100% selected_n = N (ceil unused); fold year is UTC year of first At in that fold;\n")
	b.WriteString("global raw-P sort omitted (second ranking law, not free of complexity).\n")
	return b.String()
}
