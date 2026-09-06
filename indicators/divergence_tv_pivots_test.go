package indicators_test

import (
	"testing"

	"trading_bot/indicators"
)

func tvPivotPeakSeries() (closes, rsx []float64, opens []int64) {
	n := 24
	closes = make([]float64, n)
	rsx = make([]float64, n)
	opens = make([]int64, n)
	base := int64(1_700_000_000_000)
	for i := 0; i < n; i++ {
		opens[i] = base + int64(i)*60_000
		closes[i] = 100
		rsx[i] = 50
	}
	rsx[10] = 80
	return closes, rsx, opens
}

func TestTVPivotFacts_HighTwoBarDelay(t *testing.T) {
	t.Parallel()
	closes, rsx, opens := tvPivotPeakSeries()
	facts := indicators.TVPivotFacts(closes, rsx, opens, 90)
	if len(facts) == 0 {
		t.Fatal("expected at least one TV pivot")
	}
	sawHigh := false
	for _, ev := range facts {
		if ev.Source != indicators.FactSourceRSXTVPivot {
			t.Fatalf("source %q", ev.Source)
		}
		if ev.ConfirmedAt-ev.AnchorAt != 120_000 {
			t.Fatalf("want 2-bar delay, got %d", ev.ConfirmedAt-ev.AnchorAt)
		}
		if ev.Direction == indicators.FactDirPivotHigh {
			sawHigh = true
			if ev.AnchorAt != opens[10] {
				t.Fatalf("high pivot should sit on peak bar, anchor=%d want %d", ev.AnchorAt, opens[10])
			}
			if ev.ConfirmedAt != opens[12] {
				t.Fatalf("confirm at +2 bars, got %d want %d", ev.ConfirmedAt, opens[12])
			}
		}
	}
	if !sawHigh {
		t.Fatalf("expected pivot high, facts=%+v", facts)
	}
}

func TestTVPivotFacts_NoLookahead(t *testing.T) {
	t.Parallel()
	closes, rsx, opens := tvPivotPeakSeries()
	tooEarly := indicators.TVPivotFacts(closes[:12], rsx[:12], opens[:12], 90)
	for _, ev := range tooEarly {
		if ev.AnchorAt == opens[10] && ev.Direction == indicators.FactDirPivotHigh {
			t.Fatal("must not confirm pivot high before two right bars")
		}
	}
	atConfirm := indicators.TVPivotFacts(closes[:13], rsx[:12+1], opens[:13], 90)
	found := false
	for _, ev := range atConfirm {
		if ev.Direction == indicators.FactDirPivotHigh && ev.AnchorAt == opens[10] && ev.ConfirmedAt == opens[12] {
			found = true
		}
	}
	if !found {
		t.Fatalf("pivot must be knowable on confirm bar, facts=%+v", atConfirm)
	}
}
