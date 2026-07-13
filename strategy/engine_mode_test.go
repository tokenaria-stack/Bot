package strategy

import "testing"

func TestNormalizeEngineMode(t *testing.T) {
	t.Parallel()
	cases := map[string]EngineMode{
		"":           EngineModeChartOnly,
		"ChartOnly":  EngineModeChartOnly,
		"chart_only": EngineModeChartOnly,
		"live":       EngineModeLive,
		"LIVE":       EngineModeLive,
		"garbage":    EngineModeChartOnly,
	}
	for in, want := range cases {
		if got := NormalizeEngineMode(in); got != want {
			t.Fatalf("NormalizeEngineMode(%q)=%q want %q", in, got, want)
		}
	}
}

func TestEngineAllowsStrategies_Gate(t *testing.T) {
	prev := GetEngineMode()
	t.Cleanup(func() { SetEngineMode(prev) })

	SetEngineMode(EngineModeChartOnly)
	if EngineAllowsStrategies() {
		t.Fatal("ChartOnly must gate strategies")
	}
	SetEngineMode(EngineModeLive)
	if !EngineAllowsStrategies() {
		t.Fatal("Live must allow strategies")
	}
}
