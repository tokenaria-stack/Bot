package market

import (
	"testing"

	"trading_bot/forecast"
	"trading_bot/indicators"
)

func TestTVIgnitionUsesConfirmedAtNotAnchorAt(t *testing.T) {
	t.Parallel()
	c, err := newNativeRSXContext("15m", forecast.Spec2RSXSource, forecast.Spec2RSXLength, forecast.Spec2RSXSignal,
		forecast.Spec2TVLookback, forecast.Spec2TVPivotAgePrimary, forecast.Spec2SimpleCrossAgePrim, true)
	if err != nil {
		t.Fatal(err)
	}
	base := int64(1_609_459_200_000)
	step := int64(15 * 60 * 1000)
	var opens []int64
	var closes, rsx []float64
	sawBull := false
	for i := 0; i < 500; i++ {
		at := base + int64(i)*step
		px := 100.0 + float64(i%17)*0.4 - float64(i%11)*0.3
		high, low, close := px+0.2, px-0.2, px
		if err := c.update(at, at+step, high, low, close); err != nil {
			t.Fatal(err)
		}
		opens = append(opens, at)
		closes = append(closes, close)
		rsx = append(rsx, c.rsx)
		if c.tvBullNow {
			sawBull = true
			if c.tvBull.at != at {
				t.Fatalf("tv bull timedFact.at=%d want ConfirmedAt=bar open %d", c.tvBull.at, at)
			}
			p, age := c.tvBull.presentAge(c.ord, c.tvMax)
			if p != 1 || age != 0 {
				t.Fatalf("ignition bar present/age=%g/%g want 1/0", p, age)
			}
			facts := indicators.ReplayRSTVFacts(opens, closes, rsx, forecast.Spec2TVLookback)
			found := false
			for _, ev := range facts {
				if ev.Direction == indicators.FactDirBullish && ev.ConfirmedAt == at {
					found = true
					if ev.Source != indicators.FactSourceRSXTVDiv {
						t.Fatalf("source %q", ev.Source)
					}
					if ev.AnchorAt == ev.ConfirmedAt {
						t.Fatal("AnchorAt must not be used as event time")
					}
					if ev.AnchorAt >= ev.ConfirmedAt {
						t.Fatalf("AnchorAt=%d ConfirmedAt=%d", ev.AnchorAt, ev.ConfirmedAt)
					}
				}
			}
			if !found {
				t.Fatal("age==0 TV bull At does not equal canonical ConfirmedAt")
			}
		}
		if c.tvBearNow {
			if c.tvBear.at != at {
				t.Fatalf("tv bear timedFact.at=%d want %d", c.tvBear.at, at)
			}
		}
		if c.tvBull.at != 0 && !c.tvBullNow {
			p, age := c.tvBull.presentAge(c.ord, c.tvMax)
			if p == 1 && age == 0 {
				t.Fatal("remembered TV must not look like age==0 ignition")
			}
		}
	}
	if !sawBull {
		t.Log("no TV bull on this synthetic path; ConfirmedAt identity is still asserted whenever tvBullNow fires")
	}
}
