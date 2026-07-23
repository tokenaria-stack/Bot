package market

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRSXSettings_AutosaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rsx_settings.json")
	ResetRSXSettings()
	SetRSXSettingsPath(path)
	t.Cleanup(func() {
		ResetRSXSettings()
		SetRSXSettingsPath("")
	})

	res := ApplyRSXSettings(RSXSettings{
		Length:       21,
		SignalLength: 5,
		Source:       "close",
		DivLookback:  60,
	})
	if !res.Changed {
		t.Fatal("expected change")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk RSXSettings
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk.Length != 21 || onDisk.Source != "close" || onDisk.SignalLength != 5 {
		t.Fatalf("disk = %+v", onDisk)
	}

	ResetRSXSettings() // defaults hlc3
	if GetRSXSettings().Source != "hlc3" {
		t.Fatalf("reset source = %s", GetRSXSettings().Source)
	}
	if err := LoadRSXSettingsFromDisk(); err != nil {
		t.Fatal(err)
	}
	got := GetRSXSettings()
	if got.Length != 21 || got.Source != "close" || got.SignalLength != 5 {
		t.Fatalf("loaded = %+v", got)
	}
}

func TestRSXSettings_DefaultSourceHLC3(t *testing.T) {
	ResetRSXSettings()
	t.Cleanup(ResetRSXSettings)
	if got := GetRSXSettings().Source; got != "hlc3" {
		t.Fatalf("default source = %q, want hlc3", got)
	}
}

func TestApplyRSXSettings_NoopNoRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rsx.json")
	ResetRSXSettings()
	SetRSXSettingsPath(path)
	t.Cleanup(func() {
		ResetRSXSettings()
		SetRSXSettingsPath("")
	})
	first := ApplyRSXSettings(RSXSettings{Length: 21, Source: "close"})
	if !first.Changed {
		t.Fatal("first apply must change from defaults")
	}
	gen1 := RSXSettingsGeneration()
	res := ApplyRSXSettings(RSXSettings{Length: 21, Source: "close"})
	if res.Changed {
		t.Fatal("identical apply must not change")
	}
	if RSXSettingsGeneration() != gen1 {
		t.Fatal("generation must not bump on noop")
	}
}
