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
