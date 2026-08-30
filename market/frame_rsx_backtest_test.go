package market

import "testing"

func TestApplyBacktestRSXConfig_PinsSettingsAndReplays(t *testing.T) {
	t.Parallel()
	ResetRSXSettings()
	t.Cleanup(ResetRSXSettings)

	ApplyRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "close"})

	m := NewFrame(nil, "15m", ChaosConfig{})
	cfg := RSXSettings{
		Length:       21,
		SignalLength: 5,
		Source:       "hlc3",
		PivotRadius:  3,
		DivLookback:  60,
	}
	m.ApplyBacktestRSXConfig(cfg)

	m.mu.RLock()
	got := *m.rsxSettings
	m.mu.RUnlock()

	if got.Length != 21 || got.SignalLength != 5 || got.Source != "hlc3" {
		t.Fatalf("pinned settings mismatch: %+v", got)
	}
	if got.PivotRadius != 3 {
		t.Fatalf("pivot settings mismatch: %+v", got)
	}
}
