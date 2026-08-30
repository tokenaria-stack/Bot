package market

import (
	"encoding/json"
	"testing"
)

func TestRSXSettings_UnmarshalJSON_PivotRadiusAliases(t *testing.T) {
	t.Parallel()

	var fromSnake RSXSettings
	if err := json.Unmarshal([]byte(`{"pivot_radius":4}`), &fromSnake); err != nil {
		t.Fatalf("snake: %v", err)
	}
	if fromSnake.PivotRadius != 4 {
		t.Fatalf("pivot_radius = %d, want 4", fromSnake.PivotRadius)
	}

	var fromCamel RSXSettings
	if err := json.Unmarshal([]byte(`{"pivotRadius":3}`), &fromCamel); err != nil {
		t.Fatalf("camel: %v", err)
	}
	if fromCamel.PivotRadius != 3 {
		t.Fatalf("pivotRadius = %d, want 3", fromCamel.PivotRadius)
	}
}

func TestResolveBacktestRSXSettings_PivotRadiusDefault(t *testing.T) {
	t.Parallel()

	got, ok := ResolveBacktestRSXSettings(&BacktestRunSettings{
		RSXSettings: &RSXSettings{
			Length:      14,
			DivLookback: 90,
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

func TestResolveBacktestNavigators_FromSettings(t *testing.T) {
	t.Parallel()

	navs := ResolveBacktestNavigators(&BacktestRunSettings{
		Navigators: map[string]NavigatorUISettings{
			"price": {Enabled: true, UseLong: true, LongLen: 60},
		},
	}, nil, NavigatorUISettings{})
	if len(navs) != 1 || !navs["price"].Enabled || navs["price"].LongLen != 60 {
		t.Fatalf("navigators: %+v", navs)
	}
}
