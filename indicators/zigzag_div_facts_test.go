package indicators_test

import (
	"testing"

	"trading_bot/indicators"
)

func TestClassifyZigZagDivergence_FourGeometries(t *testing.T) {
	t.Parallel()
	prevLow := indicators.ZigZagSwingSample{IsHigh: false, Price: 100, RSX: 40}
	prevHigh := indicators.ZigZagSwingSample{IsHigh: true, Price: 100, RSX: 80}
	cases := []struct {
		name    string
		prev    indicators.ZigZagSwingSample
		latest  indicators.ZigZagSwingSample
		dir     string
		pattern string
		wantOK  bool
	}{
		{"regular bull", prevLow, indicators.ZigZagSwingSample{IsHigh: false, Price: 90, RSX: 50}, indicators.FactDirBullish, indicators.FactPatternRegular, true},
		{"hidden bull", prevLow, indicators.ZigZagSwingSample{IsHigh: false, Price: 110, RSX: 30}, indicators.FactDirBullish, indicators.FactPatternHidden, true},
		{"regular bear", prevHigh, indicators.ZigZagSwingSample{IsHigh: true, Price: 120, RSX: 60}, indicators.FactDirBearish, indicators.FactPatternRegular, true},
		{"hidden bear", prevHigh, indicators.ZigZagSwingSample{IsHigh: true, Price: 80, RSX: 90}, indicators.FactDirBearish, indicators.FactPatternHidden, true},
		{"equal price", prevLow, indicators.ZigZagSwingSample{IsHigh: false, Price: 100, RSX: 50}, "", "", false},
		{"equal rsx", prevLow, indicators.ZigZagSwingSample{IsHigh: false, Price: 90, RSX: 40}, "", "", false},
		{"mixed family", prevLow, indicators.ZigZagSwingSample{IsHigh: true, Price: 90, RSX: 50}, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, pattern, ok := indicators.ClassifyZigZagDivergence(tc.prev, tc.latest)
			if ok != tc.wantOK || dir != tc.dir || pattern != tc.pattern {
				t.Fatalf("got dir=%q pattern=%q ok=%v want %q %q %v", dir, pattern, ok, tc.dir, tc.pattern, tc.wantOK)
			}
		})
	}
}

func TestClassifyZigZagDivergence_UsesSwingRSXNotConfirmBar(t *testing.T) {
	t.Parallel()
	prev := indicators.ZigZagSwingSample{IsHigh: false, Price: 100, RSX: 50}
	atSwing := indicators.ZigZagSwingSample{IsHigh: false, Price: 90, RSX: 60}
	atConfirm := indicators.ZigZagSwingSample{IsHigh: false, Price: 90, RSX: 40}
	dir, pattern, ok := indicators.ClassifyZigZagDivergence(prev, atSwing)
	if !ok || dir != indicators.FactDirBullish || pattern != indicators.FactPatternRegular {
		t.Fatalf("swing-bar RSX should be regular bull, got %q %q %v", dir, pattern, ok)
	}
	if _, _, confirmOK := indicators.ClassifyZigZagDivergence(prev, atConfirm); confirmOK {
		t.Fatal("confirmation-bar RSX must not be used; it would invent a different geometry")
	}
}

func TestZigZagDivFact_RejectsLegacyAndEqualTimes(t *testing.T) {
	t.Parallel()
	if _, ok := indicators.ZigZagDivFact("L", indicators.FactPatternRegular, 2, 1, 1, 1); ok {
		t.Fatal("legacy direction")
	}
	if _, ok := indicators.ZigZagDivFact(indicators.FactDirBullish, "strong", 2, 1, 1, 1); ok {
		t.Fatal("legacy pattern")
	}
	if _, ok := indicators.ZigZagDivFact(indicators.FactDirBullish, indicators.FactPatternRegular, 1, 1, 1, 1); ok {
		t.Fatal("ConfirmedAt must be after AnchorAt")
	}
}
