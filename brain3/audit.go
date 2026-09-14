package brain3

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func signedRankBiserial(tp, stop []float64) float64 {
	n1, n0 := len(tp), len(stop)
	if n1 == 0 || n0 == 0 {
		return 0
	}
	var gt, eq float64
	for _, a := range tp {
		for _, b := range stop {
			switch {
			case a > b:
				gt++
			case a == b:
				eq++
			}
		}
	}
	auc := (gt + 0.5*eq) / float64(n1*n0)
	return 2*auc - 1
}

func featureFamily(name string) string {
	switch {
	case name == "setup_risk_atr" || name == "s0_anchor_age_bars":
		return "setup"
	case strings.HasPrefix(name, "htf_1h_"):
		return "htf_1h"
	case strings.HasPrefix(name, "htf_4h_"):
		return "htf_4h"
	case strings.HasPrefix(name, "pattern_"):
		return "pattern"
	case strings.HasPrefix(name, "price_") || strings.HasPrefix(name, "atr_"):
		return "price_regime"
	case strings.HasPrefix(name, "tv_"):
		return "tv"
	case strings.HasPrefix(name, "rsx_"):
		return "rsx_core"
	default:
		return "other"
	}
}

type FeatureSep struct {
	Name     string
	Family   string
	FoldSep  []float64
	Median   float64
	SameSign int
}

type FamilySep struct {
	Name         string
	N            int
	Same4        int
	SameAtLeast3 int
	MedianAbs    float64
}

type AuditResult struct {
	Features []FeatureSep
	Families []FamilySep
	Text     string
}

func medianAbs(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return math.Abs(cp[mid])
	}
	return math.Abs(0.5 * (cp[mid-1] + cp[mid]))
}

func medianSigned(xs []float64) float64 {
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return 0.5 * (cp[mid-1] + cp[mid])
}

func sameSignCount(xs []float64) int {
	pos, neg := 0, 0
	for _, v := range xs {
		if v > 0 {
			pos++
		} else if v < 0 {
			neg++
		}
	}
	if pos >= neg {
		return pos
	}
	return neg
}

func RunInformationAudit(ds Dataset, geoms []BinaryFoldGeom) (AuditResult, error) {
	var z AuditResult
	w := ds.Width
	if w != 66 {
		return z, fmt.Errorf("brain3: audit width %d", w)
	}
	for fi, name := range ds.FeatureNames {
		seps := make([]float64, len(geoms))
		for k, g := range geoms {
			var tp, st []float64
			for _, r := range g.Val {
				if r.YBin == 1 {
					tp = append(tp, r.X[fi])
				} else {
					st = append(st, r.X[fi])
				}
			}
			seps[k] = signedRankBiserial(tp, st)
		}
		z.Features = append(z.Features, FeatureSep{
			Name: name, Family: featureFamily(name), FoldSep: seps,
			Median: medianSigned(seps), SameSign: sameSignCount(seps),
		})
	}
	famOrder := []string{"rsx_core", "tv", "pattern", "price_regime", "htf_1h", "htf_4h", "setup", "other"}
	for _, fam := range famOrder {
		var abs []float64
		n, s4, s3 := 0, 0, 0
		for _, f := range z.Features {
			if f.Family != fam {
				continue
			}
			n++
			abs = append(abs, math.Abs(f.Median))
			if f.SameSign == 4 {
				s4++
			}
			if f.SameSign >= 3 {
				s3++
			}
		}
		if n == 0 {
			continue
		}
		z.Families = append(z.Families, FamilySep{Name: fam, N: n, Same4: s4, SameAtLeast3: s3, MedianAbs: medianAbs(abs)})
	}
	z.Text = FormatAudit(z)
	return z, nil
}

func FormatAudit(z AuditResult) string {
	var b strings.Builder
	b.WriteString("TP-STOP-INFORMATION-AUDIT-1 (resolved OOF val only; no selection)\n")
	b.WriteString("metric = signed rank-biserial = 2*AUC(feature->TP)-1\n")
	for _, fam := range z.Families {
		b.WriteString(fmt.Sprintf("family %s  n=%d  same_sign_4/4=%d  same_sign_>=3/4=%d  median_|sep|=%.4f\n",
			fam.Name, fam.N, fam.Same4, fam.SameAtLeast3, fam.MedianAbs))
	}
	type hit struct {
		f FeatureSep
		a float64
	}
	var hits []hit
	for _, f := range z.Features {
		if f.SameSign >= 3 {
			hits = append(hits, hit{f, math.Abs(f.Median)})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].a > hits[j].a })
	if len(hits) > 12 {
		hits = hits[:12]
	}
	b.WriteString("strongest same-sign >=3/4 examples (not selected):\n")
	for _, h := range hits {
		b.WriteString(fmt.Sprintf("  %s fam=%s median=%.4f folds=", h.f.Name, h.f.Family, h.f.Median))
		for i, v := range h.f.FoldSep {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(fmt.Sprintf("%.3f", v))
		}
		b.WriteString(fmt.Sprintf(" same=%d/4\n", h.f.SameSign))
	}
	n4, n3 := 0, 0
	for _, f := range z.Features {
		if f.SameSign == 4 {
			n4++
		}
		if f.SameSign >= 3 {
			n3++
		}
	}
	b.WriteString(fmt.Sprintf("features with same-sign 4/4=%d  >=3/4=%d  of %d\n", n4, n3, len(z.Features)))
	b.WriteString("Audit cannot conclude facts work or fail. Binary probe owns that decision.\n")
	return b.String()
}
