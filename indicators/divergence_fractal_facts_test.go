package indicators_test

import (
	"strings"
	"testing"

	"trading_bot/indicators"
)

func fractalTestCfg() indicators.RSXScanConfig {
	return indicators.NormalizeRSXScanConfig(indicators.RSXScanConfig{
		Mode:        indicators.RSXScanFractal,
		Lookback:    90,
		PivotRadius: 2,
	})
}

func twoOscPeaks(price1, price2, osc1, osc2 float64, lows bool) (prices, rsx []float64, opens []int64) {
	n := 36
	prices = make([]float64, n)
	rsx = make([]float64, n)
	opens = make([]int64, n)
	base := int64(1_700_000_000_000)
	for i := 0; i < n; i++ {
		prices[i] = 100
		rsx[i] = 50
		opens[i] = base + int64(i)*60_000
	}
	i1, i2 := 8, 22
	shoulder := 45.0
	if lows {
		shoulder = 55
		rsx[i1] = osc1
		rsx[i2] = osc2
		for _, j := range []int{i1 - 2, i1 - 1, i1 + 1, i1 + 2, i2 - 2, i2 - 1, i2 + 1, i2 + 2} {
			rsx[j] = shoulder
		}
	} else {
		for _, j := range []int{i1 - 2, i1 - 1, i1 + 1, i1 + 2, i2 - 2, i2 - 1, i2 + 1, i2 + 2} {
			rsx[j] = shoulder
		}
		rsx[i1] = osc1
		rsx[i2] = osc2
	}
	prices[i1] = price1
	prices[i2] = price2
	return prices, rsx, opens
}

func findFractalDiv(t *testing.T, facts []indicators.IndicatorFactEvent, dir, pattern string) indicators.IndicatorFactEvent {
	t.Helper()
	var found []indicators.IndicatorFactEvent
	for _, ev := range facts {
		if ev.Source == indicators.FactSourceRSXFractalDiv && ev.Direction == dir && ev.Pattern == pattern {
			found = append(found, ev)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want 1 %s %s, got %d from %+v", dir, pattern, len(found), facts)
	}
	return found[0]
}

func assertNoLegacyVocab(t *testing.T, facts []indicators.IndicatorFactEvent) {
	t.Helper()
	legacy := []string{"L", "LL", "S", "SS", "P", "BUY", "SELL"}
	for _, ev := range facts {
		blob := strings.Join([]string{ev.Source, ev.Direction, ev.Pattern}, " ")
		for _, tok := range legacy {
			if ev.Direction == tok || ev.Pattern == tok || ev.Source == tok {
				t.Fatalf("legacy vocab in fact %+v", ev)
			}
			if blob == tok {
				t.Fatalf("legacy vocab %q in %+v", tok, ev)
			}
		}
	}
}

func TestFractalFacts_ClassMapping(t *testing.T) {
	t.Parallel()
	cfg := fractalTestCfg()
	type tc struct {
		name           string
		lows           bool
		p1, p2, o1, o2 float64
		dir, pattern   string
	}
	cases := []tc{
		{"bullish A", true, 90, 80, 20, 30, indicators.FactDirBullish, indicators.FactPatternClassA},
		{"bullish B", true, 90, 90, 20, 30, indicators.FactDirBullish, indicators.FactPatternClassB},
		{"bullish C", true, 90, 80, 20, 20, indicators.FactDirBullish, indicators.FactPatternClassC},
		{"bearish A", false, 100, 110, 80, 70, indicators.FactDirBearish, indicators.FactPatternClassA},
		{"bearish B", false, 100, 100, 80, 70, indicators.FactDirBearish, indicators.FactPatternClassB},
		{"bearish C", false, 100, 110, 80, 80, indicators.FactDirBearish, indicators.FactPatternClassC},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prices, rsx, opens := twoOscPeaks(c.p1, c.p2, c.o1, c.o2, c.lows)
			facts := indicators.FractalFacts(prices, rsx, opens, cfg)
			assertNoLegacyVocab(t, facts)
			ev := findFractalDiv(t, facts, c.dir, c.pattern)
			if ev.AnchorAt != opens[22] {
				t.Fatalf("AnchorAt=%d want %d", ev.AnchorAt, opens[22])
			}
			if ev.ConfirmedAt != opens[24] {
				t.Fatalf("ConfirmedAt=%d want confirm bar 24 %d", ev.ConfirmedAt, opens[24])
			}
			if ev.ConfirmedAt <= ev.AnchorAt {
				t.Fatal("lookahead: confirm must be after anchor")
			}
			if ev.AnchorValue != rsx[22] || ev.AnchorPrice != prices[22] {
				t.Fatalf("anchor sample %+v", ev)
			}
		})
	}
}

func TestFractalFacts_AAndCDoNotCollapse(t *testing.T) {
	t.Parallel()
	cfg := fractalTestCfg()
	aPrices, aRSX, aOpens := twoOscPeaks(100, 110, 80, 70, false)
	cPrices, cRSX, cOpens := twoOscPeaks(100, 110, 80, 80, false)
	aFacts := indicators.FractalFacts(aPrices, aRSX, aOpens, cfg)
	cFacts := indicators.FractalFacts(cPrices, cRSX, cOpens, cfg)
	aEv := findFractalDiv(t, aFacts, indicators.FactDirBearish, indicators.FactPatternClassA)
	cEv := findFractalDiv(t, cFacts, indicators.FactDirBearish, indicators.FactPatternClassC)
	if aEv.Pattern == cEv.Pattern {
		t.Fatal("Class A and Class C collapsed")
	}
}

func TestFractalFacts_NoDivergenceMacroPivot(t *testing.T) {
	t.Parallel()
	cfg := fractalTestCfg()
	prices, rsx, opens := twoOscPeaks(100, 110, 70, 85, false)
	facts := indicators.FractalFacts(prices, rsx, opens, cfg)
	assertNoLegacyVocab(t, facts)
	var divs, highs, lows int
	for _, ev := range facts {
		switch ev.Source {
		case indicators.FactSourceRSXFractalDiv:
			divs++
		case indicators.FactSourceRSXFractalPivot:
			if ev.Direction == indicators.FactDirPivotHigh {
				highs++
			}
			if ev.Direction == indicators.FactDirPivotLow {
				lows++
			}
			if ev.Pattern != "" {
				t.Fatalf("pivot pattern must be empty: %+v", ev)
			}
		}
	}
	if divs != 0 {
		t.Fatalf("expected no divergence, got %+v", facts)
	}
	if highs == 0 {
		t.Fatalf("expected pivot high, got %+v", facts)
	}
	_ = lows
}

func TestFractalFacts_PivotLow(t *testing.T) {
	t.Parallel()
	rsx := []float64{50, 48, 46, 42, 38, 36, 37, 30, 37, 39, 42, 46, 48, 50, 52}
	prices := make([]float64, len(rsx))
	opens := make([]int64, len(rsx))
	for i := range prices {
		prices[i] = 100 + rsx[i]
		opens[i] = 1_700_000_000_000 + int64(i)*60_000
	}
	facts := indicators.FractalFacts(prices, rsx, opens, fractalTestCfg())
	assertNoLegacyVocab(t, facts)
	found := false
	for _, ev := range facts {
		if ev.Source == indicators.FactSourceRSXFractalPivot && ev.Direction == indicators.FactDirPivotLow {
			found = true
			if ev.AnchorAt != opens[7] {
				t.Fatalf("low AnchorAt=%d want %d", ev.AnchorAt, opens[7])
			}
			if ev.ConfirmedAt != opens[7+indicators.DefaultRSXMacroPivotRadius] {
				t.Fatalf("low ConfirmedAt=%d", ev.ConfirmedAt)
			}
		}
	}
	if !found {
		t.Fatalf("expected fractal pivot low, got %+v", facts)
	}
}

func TestFractalFacts_EventOnceAndNoLookahead(t *testing.T) {
	t.Parallel()
	cfg := fractalTestCfg()
	prices, rsx, opens := twoOscPeaks(100, 110, 80, 70, false)
	confirm := 24
	if got := indicators.FractalFactsAt(prices, rsx, opens, cfg, confirm-1); len(got) != 0 {
		t.Fatalf("lookahead at %d: %+v", confirm-1, got)
	}
	first := indicators.FractalFactsAt(prices, rsx, opens, cfg, confirm)
	if len(first) != 1 || first[0].Pattern != indicators.FactPatternClassA {
		t.Fatalf("confirm bar facts %+v", first)
	}
	if got := indicators.FractalFactsAt(prices, rsx, opens, cfg, confirm+1); len(got) != 0 {
		t.Fatalf("re-emit on later bar: %+v", got)
	}
	batch := indicators.FractalFacts(prices, rsx, opens, cfg)
	var seq []indicators.IndicatorFactEvent
	for i := range prices {
		seq = append(seq, indicators.FractalFactsAt(prices, rsx, opens, cfg, i)...)
	}
	if len(batch) != len(seq) {
		t.Fatalf("batch %d seq %d", len(batch), len(seq))
	}
	for i := range batch {
		if batch[i] != seq[i] {
			t.Fatalf("i=%d batch=%+v seq=%+v", i, batch[i], seq[i])
		}
	}
}

func TestFractalDetectorParity_ScanHitsMatchFacts(t *testing.T) {
	t.Parallel()
	cfg := fractalTestCfg()
	prices, rsx, opens := twoOscPeaks(100, 110, 80, 70, false)
	bus := &stubDataBus{jurik: rsx, prices: prices, closes: prices}
	hits := indicators.ScanRSXMarkers(bus, cfg)
	facts := indicators.FractalFacts(prices, rsx, opens, cfg)
	if len(hits) != len(facts) {
		t.Fatalf("hits %d facts %d: hits=%+v facts=%+v", len(hits), len(facts), hits, facts)
	}
	for i, h := range hits {
		if h.Label != "" {
			t.Fatalf("hit label leak %q", h.Label)
		}
		ev := facts[i]
		if ev.AnchorAt != opens[h.PivotBar] || ev.ConfirmedAt != opens[h.DisplayBar] {
			t.Fatalf("timing hit=%+v fact=%+v", h, ev)
		}
	}
}
