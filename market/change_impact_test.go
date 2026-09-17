package market

import (
	"math"
	"path/filepath"
	"testing"

	"trading_bot/exchange"
)

func TestRSXImpactOfChange_Classes(t *testing.T) {
	base := defaultRSXSettings()

	cases := []struct {
		name string
		next RSXSettings
		want ChangeImpact
	}{
		{"noop", base, ChangeImpactProjectionOnly},
		{"length", RSXSettings{Length: 21}, ChangeImpactIndicatorReplay},
		{"signal", RSXSettings{SignalLength: 14}, ChangeImpactIndicatorReplay},
		{"source", RSXSettings{Source: "close"}, ChangeImpactIndicatorReplay},
		{"pivot", RSXSettings{PivotRadius: 4}, ChangeImpactAnnotationOnly},
		{"lookback", RSXSettings{DivLookback: 120}, ChangeImpactAnnotationOnly},
		{"min_osc", RSXSettings{MinOscDelta: 1.5}, ChangeImpactAnnotationOnly},
		{"length_wins_over_pivot", RSXSettings{Length: 21, PivotRadius: 4}, ChangeImpactIndicatorReplay},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next := NormalizeRSXSettings(mergeRSXSettings(base, tc.next))
			got := RSXImpactOfChange(base, next)
			if got != tc.want {
				t.Fatalf("impact = %s, want %s", got, tc.want)
			}
			if (got == ChangeImpactIndicatorReplay) != RSXNeedsStreamingReplay(base, next) {
				t.Fatal("RSXNeedsStreamingReplay must match IndicatorReplay")
			}
		})
	}
}

func TestUpdateRSXScanConfig_PivotRadiusPreservesTip(t *testing.T) {
	ResetRSXSettings()
	SetRSXSettingsPath(filepath.Join(t.TempDir(), "rsx.json"))
	t.Cleanup(func() {
		ResetRSXSettings()
		SetRSXSettingsPath("")
	})

	ApplyRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", PivotRadius: 2})
	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	warmupMarkerBars(frame, 80, 1_700_000_000_000, 60_000)
	baseline := markerJurikRSX(frame)
	if baseline == 0 || math.IsNaN(baseline) {
		t.Fatalf("expected warm RSX tip, got %v", baseline)
	}

	prev := GetRSXSettings()
	nextPivot := NormalizeRSXSettings(mergeRSXSettings(prev, RSXSettings{PivotRadius: 4}))
	_ = ApplyRSXSettings(nextPivot)
	frame.UpdateRSXScanConfig(prev, nextPivot)
	afterPivot := markerJurikRSX(frame)
	if afterPivot != baseline {
		t.Fatalf("PivotRadius 2→4 mutated tip: before=%v after=%v", baseline, afterPivot)
	}

	prev2 := GetRSXSettings()
	nextLookback := NormalizeRSXSettings(mergeRSXSettings(prev2, RSXSettings{DivLookback: 120}))
	_ = ApplyRSXSettings(nextLookback)
	frame.UpdateRSXScanConfig(prev2, nextLookback)
	afterLookback := markerJurikRSX(frame)
	if afterLookback != baseline {
		t.Fatalf("DivLookback mutated tip: before=%v after=%v", baseline, afterLookback)
	}
}

func TestUpdateRSXScanConfig_LengthReplays(t *testing.T) {
	ResetRSXSettings()
	SetRSXSettingsPath(filepath.Join(t.TempDir(), "rsx.json"))
	t.Cleanup(func() {
		ResetRSXSettings()
		SetRSXSettingsPath("")
	})

	ApplyRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	klines := make([]exchange.Kline, 120)
	for i := range klines {
		p := 100 + math.Sin(float64(i)*0.35)*8
		klines[i] = exchange.Kline{
			OpenTime:  1_700_000_000_000 + int64(i)*60_000,
			CloseTime: 1_700_000_000_000 + int64(i+1)*60_000 - 1,
			Open:      p, High: p + 1.5, Low: p - 1.5, Close: p + 0.25, Volume: 10,
		}
	}
	frame.LoadHistoricalKlines(klines)
	before := markerJurikRSX(frame)

	prev := GetRSXSettings()
	next := NormalizeRSXSettings(mergeRSXSettings(prev, RSXSettings{Length: 7}))
	_ = ApplyRSXSettings(next)
	frame.UpdateRSXScanConfig(prev, next)
	after := markerJurikRSX(frame)
	if math.Abs(after-before) < 1e-6 {
		t.Fatalf("Length 14→7 should change tip RSX (before=%v after=%v)", before, after)
	}
}
