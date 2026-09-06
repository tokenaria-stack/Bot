package indicators_test

import (
	"testing"

	"trading_bot/indicators"
)

func tvForcedSeries() (closes, rsx []float64, opens []int64) {
	n := 40
	closes = make([]float64, n)
	rsx = make([]float64, n)
	opens = make([]int64, n)
	const step int64 = 60_000
	base := int64(1_700_000_000_000)
	for i := 0; i < n; i++ {
		opens[i] = base + int64(i)*step
		closes[i] = 100 + float64(i)
		if i <= 12 {
			rsx[i] = 40 + float64(i)
		} else {
			rsx[i] = 52 - float64(i-12)*1.5
		}
	}
	return closes, rsx, opens
}

func TestTVDivergenceFacts_MapHitsWithoutLegacyLabels(t *testing.T) {
	t.Parallel()
	closes, rsx, opens := tvForcedSeries()
	facts := indicators.TVDivergenceFacts(closes, rsx, opens, 90)
	if len(facts) == 0 {
		t.Fatal("expected at least one TV divergence fact")
	}
	sawBull, sawBear := false, false
	for _, ev := range facts {
		if ev.Source != indicators.FactSourceRSXTVDiv {
			t.Fatalf("source=%q", ev.Source)
		}
		if ev.Direction == indicators.FactDirBullish {
			sawBull = true
		} else if ev.Direction == indicators.FactDirBearish {
			sawBear = true
		} else {
			t.Fatalf("direction=%q", ev.Direction)
		}
		if ev.ConfirmedAt-ev.AnchorAt != 60_000 {
			t.Fatalf("want one 1m bar delay, got confirmed=%d anchor=%d", ev.ConfirmedAt, ev.AnchorAt)
		}
	}
	if !sawBear {
		t.Fatalf("expected bearish TV fact, n=%d", len(facts))
	}
	_ = sawBull
}

func TestTVDivergenceFacts_EventPurity(t *testing.T) {
	t.Parallel()
	closes, rsx, opens := tvForcedSeries()
	for _, ev := range indicators.TVDivergenceFacts(closes, rsx, opens, 90) {
		if ev.Source != indicators.FactSourceRSXTVDiv {
			t.Fatalf("source %q", ev.Source)
		}
		if ev.Direction != indicators.FactDirBullish && ev.Direction != indicators.FactDirBearish {
			t.Fatalf("direction %q", ev.Direction)
		}
		switch ev.Direction {
		case "L", "LL", "S", "SS", "BUY", "SELL", "LONG", "SHORT":
			t.Fatalf("legacy meaning leaked: %q", ev.Direction)
		}
		if ev.AnchorAt <= 0 || ev.ConfirmedAt <= 0 {
			t.Fatal("times must be closed-bar OpenTime ms")
		}
		if ev.ConfirmedAt <= ev.AnchorAt {
			t.Fatal("ConfirmedAt must be after AnchorAt")
		}
	}
}

func TestTVDivergenceFacts_XbarsChangesHits(t *testing.T) {
	t.Parallel()
	n := 100
	closes := make([]float64, n)
	rsx := make([]float64, n)
	opens := make([]int64, n)
	base := int64(1_700_000_000_000)
	for i := 0; i < n; i++ {
		opens[i] = base + int64(i)*60_000
		closes[i] = 100 + float64(i)
		if i == 8 {
			rsx[i] = 92
		} else if i <= 20 {
			rsx[i] = 40 + float64(i)*0.5
		} else {
			rsx[i] = 50 - float64(i-20)*0.3
		}
	}
	a := indicators.TVDivergenceFacts(closes, rsx, opens, 8)
	b := indicators.TVDivergenceFacts(closes, rsx, opens, 90)
	if len(a) == 0 && len(b) == 0 {
		t.Fatal("expected TV facts on rising-close / rolling-RSX series")
	}
	if len(a) == len(b) {
		identical := true
		for i := range a {
			if a[i] != b[i] {
				identical = false
				break
			}
		}
		if identical {
			t.Fatal("xbars 8 vs 90 must not emit the same fact sequence")
		}
	}
}

func TestTVDivergenceFacts_FlatHasNone(t *testing.T) {
	t.Parallel()
	n := 80
	closes := make([]float64, n)
	rsx := make([]float64, n)
	opens := make([]int64, n)
	for i := 0; i < n; i++ {
		closes[i] = 100
		rsx[i] = 50
		opens[i] = 1_700_000_000_000 + int64(i)*60_000
	}
	if got := indicators.TVDivergenceFacts(closes, rsx, opens, 90); len(got) != 0 {
		t.Fatalf("flat series facts=%+v", got)
	}
}

func TestTVDivergenceFacts_BullishInverse(t *testing.T) {
	t.Parallel()
	n := 40
	closes := make([]float64, n)
	rsx := make([]float64, n)
	opens := make([]int64, n)
	base := int64(1_700_000_000_000)
	for i := 0; i < n; i++ {
		opens[i] = base + int64(i)*60_000
		closes[i] = 200 - float64(i)
		if i <= 12 {
			rsx[i] = 70 - float64(i)
		} else {
			rsx[i] = 58 + float64(i-12)*1.5
		}
	}
	facts := indicators.TVDivergenceFacts(closes, rsx, opens, 90)
	sawBull := false
	for _, ev := range facts {
		if ev.Direction == indicators.FactDirBullish {
			sawBull = true
			if ev.ConfirmedAt-ev.AnchorAt != 60_000 {
				t.Fatalf("bull delay %d", ev.ConfirmedAt-ev.AnchorAt)
			}
		}
	}
	if !sawBull {
		t.Fatalf("expected bullish fact, n=%d", len(facts))
	}
}
