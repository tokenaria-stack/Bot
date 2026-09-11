package market

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/forecast"
)

func spec2(t *testing.T) forecast.FeatureSpec2 {
	t.Helper()
	s, err := ResearchFeatureSpec2()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func synthBars(start int64, tf string, n int, seed float64) []exchange.Kline {
	out := make([]exchange.Kline, n)
	ot := start
	for i := 0; i < n; i++ {
		px := seed + float64(i)*0.35
		ct, _ := data.BarCloseTimeMs(ot, tf)
		out[i] = exchange.Kline{
			OpenTime: ot, CloseTime: ct,
			Open: px, High: px + 1.5, Low: px - 0.4, Close: px + 0.2, Volume: 10,
		}
		ot, _ = data.NextBarOpen(ot, tf)
	}
	return out
}

func alignedStart() int64 {
	return time.Date(2020, 1, 6, 0, 0, 0, 0, time.UTC).UnixMilli() // Monday 00:00
}

func TestDumpFeatureTape2_SyntheticDeterminismAndWidth(t *testing.T) {
	s := spec2(t)
	start := alignedStart()
	p := synthBars(start, "15m", 320, 10000)
	h1 := synthBars(start, "1h", 80, 10000)
	h4 := synthBars(start, "4h", 20, 10000)
	dir := t.TempDir()
	a := filepath.Join(dir, "a.featuretape2")
	b := filepath.Join(dir, "b.featuretape2")
	if err := DumpFeatureTape2(a, s, p, h1, h4); err != nil {
		t.Fatal(err)
	}
	if err := DumpFeatureTape2(b, s, p, h1, h4); err != nil {
		t.Fatal(err)
	}
	_, ra, fa, err := forecast.ReadTape2(a)
	if err != nil {
		t.Fatal(err)
	}
	_, rb, fb, err := forecast.ReadTape2(b)
	if err != nil {
		t.Fatal(err)
	}
	if fa.ContentDigest != fb.ContentDigest {
		t.Fatal("determinism")
	}
	if fa.RowCount != len(p) {
		t.Fatalf("row count %d want %d", fa.RowCount, len(p))
	}
	ready := 0
	for i, row := range ra {
		if row.At != p[i].OpenTime {
			t.Fatal("At lockstep")
		}
		if row.Ready {
			ready++
			if err := forecast.Vector2Finite(row.Values); err != nil {
				t.Fatal(err)
			}
			if rb[i].Values != row.Values {
				t.Fatal("Float64bits")
			}
			for j := range row.Values {
				if math.Float64bits(row.Values[j]) != math.Float64bits(rb[i].Values[j]) {
					t.Fatal("bits")
				}
			}
		} else if row.Values != (forecast.FeatureVector2{}) {
			t.Fatal("fake zeros")
		}
	}
	if ready == 0 || fa.ReadyCount != ready {
		t.Fatal("no ready rows")
	}
	t.Log("rows", fa.RowCount, "ready", fa.ReadyCount, "not_ready", fa.NotReadyCount, "digest", fa.ContentDigest)
}

func TestDumpFeatureTape2_EqualCloseTimeHTFVisible(t *testing.T) {
	s := spec2(t)
	start := alignedStart()
	p := synthBars(start, "15m", 320, 10000)
	h1 := synthBars(start, "1h", 80, 10000)
	h4 := synthBars(start, "4h", 20, 10000)
	rt, err := NewFeatureRuntime2(s)
	if err != nil {
		t.Fatal(err)
	}
	i1, i4 := 0, 0
	var sawEqual bool
	for _, bar := range p {
		pct, _ := data.BarCloseTimeMs(bar.OpenTime, "15m")
		for i4 < len(h4) {
			ct, _ := data.BarCloseTimeMs(h4[i4].OpenTime, "4h")
			if ct > pct {
				break
			}
			k := h4[i4]
			if err := rt.Update4h(k.OpenTime, k.High, k.Low, k.Close); err != nil {
				t.Fatal(err)
			}
			i4++
		}
		equal := false
		for i1 < len(h1) {
			ct, _ := data.BarCloseTimeMs(h1[i1].OpenTime, "1h")
			if ct > pct {
				break
			}
			if ct == pct {
				equal = true
			}
			k := h1[i1]
			if err := rt.Update1h(k.OpenTime, k.High, k.Low, k.Close); err != nil {
				t.Fatal(err)
			}
			i1++
		}
		if _, err := rt.Update15m(bar.OpenTime, bar.High, bar.Low, bar.Close); err != nil {
			t.Fatal(err)
		}
		if equal {
			sawEqual = true
			if rt.ctx1h.closeTime != pct {
				t.Fatalf("HTF at equal CloseTime not visible: htf=%d primary=%d", rt.ctx1h.closeTime, pct)
			}
		}
	}
	if !sawEqual {
		t.Fatal("no equal CloseTime pair")
	}
}

func TestDumpFeatureTape2_FormingHTFInvisibleAndHoldLast(t *testing.T) {
	s := spec2(t)
	start := alignedStart()
	p := synthBars(start, "15m", 320, 10000)
	h1 := synthBars(start, "1h", 80, 10000)
	h4 := synthBars(start, "4h", 20, 10000)
	rt, err := NewFeatureRuntime2(s)
	if err != nil {
		t.Fatal(err)
	}
	i1, i4 := 0, 0
	var last1h int64
	for _, bar := range p {
		pct, _ := data.BarCloseTimeMs(bar.OpenTime, "15m")
		for i4 < len(h4) {
			ct, _ := data.BarCloseTimeMs(h4[i4].OpenTime, "4h")
			if ct > pct {
				break
			}
			k := h4[i4]
			_ = rt.Update4h(k.OpenTime, k.High, k.Low, k.Close)
			i4++
		}
		for i1 < len(h1) {
			ct, _ := data.BarCloseTimeMs(h1[i1].OpenTime, "1h")
			if ct > pct {
				break
			}
			k := h1[i1]
			_ = rt.Update1h(k.OpenTime, k.High, k.Low, k.Close)
			last1h = k.OpenTime
			i1++
		}
		if _, err := rt.Update15m(bar.OpenTime, bar.High, bar.Low, bar.Close); err != nil {
			t.Fatal(err)
		}
		if i1 < len(h1) {
			nextCT, _ := data.BarCloseTimeMs(h1[i1].OpenTime, "1h")
			if nextCT > pct && rt.ctx1h.at != last1h && last1h != 0 {
				t.Fatal("forming/future 1h leaked")
			}
		}
	}
}

func TestDumpFeatureTape2_GapAborts(t *testing.T) {
	s := spec2(t)
	start := alignedStart()
	p := synthBars(start, "15m", 40, 100)
	p = append(p[:10], p[12:]...)
	h1 := synthBars(start, "1h", 20, 100)
	h4 := synthBars(start, "4h", 16, 100)
	if err := DumpFeatureTape2(filepath.Join(t.TempDir(), "g.featuretape2"), s, p, h1, h4); err == nil {
		t.Fatal("gap")
	}
}

func TestDumpFeatureTape2_FutureHTFDoesNotChangeDigest(t *testing.T) {
	s := spec2(t)
	start := alignedStart()
	p := synthBars(start, "15m", 320, 10000)
	h1 := synthBars(start, "1h", 80, 10000)
	h4 := synthBars(start, "4h", 20, 10000)
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	if err := DumpFeatureTape2(a, s, p, h1, h4); err != nil {
		t.Fatal(err)
	}
	_, _, fa, err := forecast.ReadTape2(a)
	if err != nil {
		t.Fatal(err)
	}
	extra1 := synthBars(h1[len(h1)-1].OpenTime, "1h", 5, 20000)
	h1b := append(append([]exchange.Kline{}, h1...), extra1[1:]...)
	b := filepath.Join(dir, "b")
	if err := DumpFeatureTape2(b, s, p, h1b, h4); err != nil {
		t.Fatal(err)
	}
	hdrA, _, _, _ := forecast.ReadTape2(a)
	hdrB, _, fb, _ := forecast.ReadTape2(b)
	if hdrA.HTF1hSource != hdrB.HTF1hSource || fa.ContentDigest != fb.ContentDigest {
		t.Fatal("future 1h changed consumed digest")
	}
}

func TestDumpFeatureTape2_MatchExisting(t *testing.T) {
	s := spec2(t)
	start := alignedStart()
	p := synthBars(start, "15m", 320, 10000)
	h1 := synthBars(start, "1h", 80, 10000)
	h4 := synthBars(start, "4h", 20, 10000)
	path := filepath.Join(t.TempDir(), "m.featuretape2")
	if err := DumpFeatureTape2(path, s, p, h1, h4); err != nil {
		t.Fatal(err)
	}
	if err := DumpFeatureTape2(path, s, p, h1, h4); err != nil {
		t.Fatal(err)
	}
}

func TestDumpFeatureTape2_RefusesWrongAnalysis(t *testing.T) {
	s := spec2(t)
	bad, err := forecast.ResolveAnalysisRecipe("x", forecast.AnalysisRecipeDraft{
		RSXLength: 14, RSXSignal: 9, RSXSource: "hlc3", DivLookback: 90, EnableTV: true,
	}, "analysis:v2")
	if err != nil {
		t.Fatal(err)
	}
	s.Analysis = bad
	start := alignedStart()
	if err := DumpFeatureTape2(filepath.Join(t.TempDir(), "x"), s,
		synthBars(start, "15m", 20, 1), synthBars(start, "1h", 16, 1), synthBars(start, "4h", 16, 1)); err == nil {
		t.Fatal("signal 9")
	}
}

func TestFeatureRuntime2_UsesHistoryDemandNotFeatureHistoryBars(t *testing.T) {
	s := spec2(t)
	if s.Plan.FeatureHistoryBars == 256 {
		t.Fatal("spec2 ages")
	}
	if !s.Demand.IIRFromSourceStart {
		t.Fatal("IIR")
	}
	rt, err := NewFeatureRuntime2(s)
	if err != nil {
		t.Fatal(err)
	}
	if rt.spec.Demand.Primary15mWindowBars != s.Demand.Primary15mWindowBars {
		t.Fatal("demand")
	}
}

func TestTape2Files_V1Firewall(t *testing.T) {
	files := []string{
		"feature_runtime2.go", "native_rsx_context.go", "feature_tape2_dump.go",
	}
	banned := []string{
		"FeatureEvaluator", "BindFeatureEvaluator", "ResearchRSXSettings",
		"FeatureHistoryBars", "validateTape1ASchema", "feature-tape-v1", "DumpFeatureTape(",
	}
	for _, name := range files {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		src := string(body)
		for _, b := range banned {
			if strings.Contains(src, b) {
				t.Fatalf("%s contains %s", name, b)
			}
		}
	}
}

func TestPatternAgeCache_EqualsOrdinal(t *testing.T) {
	f := timedFact{at: 100, ord: 3}
	p, a := f.presentAge(10, 36)
	if p != 1 || a != 7 {
		t.Fatal(p, a)
	}
	wantP, wantA := forecast.NativePresentAge(10, 3, 36)
	if p != wantP || a != wantA {
		t.Fatal("cache != canonical stepping")
	}
}

func TestDumpFeatureTape2_NoLocalHTFAggregation(t *testing.T) {
	body, err := os.ReadFile("feature_tape2_dump.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	if strings.Contains(src, "15m ×") || strings.Contains(src, "aggregate") {
		t.Fatal("aggregation")
	}
}
