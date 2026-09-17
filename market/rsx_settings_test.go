package market

import (
	"path/filepath"
	"testing"
)

func TestApplyRSXSettings_Clamp(t *testing.T) {
	ResetRSXSettings()
	SetRSXSettingsPath(filepath.Join(t.TempDir(), "rsx.json"))
	t.Cleanup(func() {
		ResetRSXSettings()
		SetRSXSettingsPath("")
	})

	got := ApplyRSXSettings(RSXSettings{
		DivLookback:  500,
		SignalLength: 99,
		Length:       200,
	}).Settings
	if got.DivLookback != MaxRSXDivLookback {
		t.Fatalf("DivLookback = %d, want %d", got.DivLookback, MaxRSXDivLookback)
	}
	if got.SignalLength != MaxRSXSignalLength {
		t.Fatalf("SignalLength = %d, want %d", got.SignalLength, MaxRSXSignalLength)
	}
	if got.Length != MaxRSXLength {
		t.Fatalf("Length = %d, want %d", got.Length, MaxRSXLength)
	}

	got = ApplyRSXSettings(RSXSettings{
		Length:       7,
		DivLookback:  45,
		SignalLength: 12,
		Source:       "hlc3",
		PivotRadius:  3,
	}).Settings
	if got.Length != 7 || got.DivLookback != 45 || got.SignalLength != 12 {
		t.Fatalf("applied = %+v", got)
	}
	if got.Source != "hlc3" || got.PivotRadius != 3 {
		t.Fatalf("source/pivot = %+v", got)
	}
	cur := GetRSXSettings()
	if cur.Length != 7 || cur.DivLookback != 45 || cur.SignalLength != 12 {
		t.Fatalf("globals = %+v", cur)
	}
}
