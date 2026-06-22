package server

import (
	"encoding/json"
	"testing"

	"trading_bot/strategy"
)

func TestBacktestRequest_UnmarshalSettingsBlock(t *testing.T) {
	body := []byte(`{
		"symbol": "BTCUSDT",
		"interval": "15m",
		"startDate": "2025-01-01",
		"endDate": "2025-06-01",
		"settings": {
			"matrix": {
				"useRSX": true,
				"useTrendlines": true,
				"useWozduhCross": true
			},
			"navigators": {
				"price": {
					"enabled": true,
					"source": "Price",
					"useLong": true,
					"longLen": 60,
					"useMedium": true,
					"mediumLen": 30,
					"useShort": true,
					"shortLen": 10
				}
			}
		}
	}`)

	var req BacktestRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Settings == nil {
		t.Fatal("settings is nil")
	}
	if !req.Settings.Matrix.UseRSX || !req.Settings.Matrix.UseTrendlines {
		t.Fatalf("matrix not decoded: %+v", req.Settings.Matrix)
	}
	price := req.Settings.Navigators["price"]
	if !price.Enabled || price.LongLen != 60 {
		t.Fatalf("price navigator not decoded: %+v", price)
	}

	matrix := strategy.ResolveBacktestMatrix(req.Settings)
	if !strategy.ScoringMatrixEntrySourcesEnabledFor(matrix) {
		t.Fatalf("entry sources disabled after resolve: %+v", matrix)
	}
	navs := strategy.ResolveBacktestNavigators(req.Settings, req.Navigators, req.Navigator)
	if len(navs) != 1 || !navs["price"].Enabled {
		t.Fatalf("navigators after resolve: %+v", navs)
	}
}
