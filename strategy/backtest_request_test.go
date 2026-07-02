package strategy

import (
	"encoding/json"
	"testing"
)

func TestRSXSettings_UnmarshalJSON_PivotRadiusAliases(t *testing.T) {
	t.Parallel()

	var fromSnake RSXSettings
	if err := json.Unmarshal([]byte(`{"pivot_radius":4,"div_method":"fractal"}`), &fromSnake); err != nil {
		t.Fatalf("snake: %v", err)
	}
	if fromSnake.PivotRadius != 4 {
		t.Fatalf("pivot_radius = %d, want 4", fromSnake.PivotRadius)
	}

	var fromCamel RSXSettings
	if err := json.Unmarshal([]byte(`{"pivotRadius":3,"div_method":"fractal"}`), &fromCamel); err != nil {
		t.Fatalf("camel: %v", err)
	}
	if fromCamel.PivotRadius != 3 {
		t.Fatalf("pivotRadius = %d, want 3", fromCamel.PivotRadius)
	}
}

func TestResolveBacktestThresholds(t *testing.T) {
	t.Parallel()

	long, short := ResolveBacktestThresholds(nil)
	if long != DefaultScoreThreshold || short != DefaultScoreThreshold {
		t.Fatalf("nil settings = %d/%d, want default %d", long, short, DefaultScoreThreshold)
	}

	long, short = ResolveBacktestThresholds(&BacktestRunSettings{
		LongThreshold:  35,
		ShortThreshold: 40,
	})
	if long != 35 || short != 40 {
		t.Fatalf("explicit = %d/%d, want 35/40", long, short)
	}

	long, short = ResolveBacktestThresholds(&BacktestRunSettings{
		LongThreshold:  5,
		ShortThreshold: 250,
	})
	if long != DefaultScoreThreshold || short != DefaultScoreThreshold {
		t.Fatalf("out of range = %d/%d, want default", long, short)
	}
}

func TestResolveBacktestRSXSettings_PivotRadiusDefault(t *testing.T) {
	t.Parallel()

	got, ok := ResolveBacktestRSXSettings(&BacktestRunSettings{
		RSXSettings: &RSXSettings{
			Length:      14,
			DivLookback: 90,
			DivMethod:   "fractal",
			Source:      "close",
		},
	})
	if !ok {
		t.Fatal("expected settings")
	}
	if got.PivotRadius != DefaultRSXPivotRadius {
		t.Fatalf("PivotRadius = %d, want default %d", got.PivotRadius, DefaultRSXPivotRadius)
	}
}
