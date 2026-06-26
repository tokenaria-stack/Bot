package strategy_test

import (
	"encoding/json"
	"testing"

	"trading_bot/strategy"
)

func TestBacktestRunSettings_UnmarshalFrontendPayload(t *testing.T) {
	body := `{
		"symbol": "BTCUSDT",
		"interval": "15m",
		"startDate": "2025-01-01",
		"endDate": "2025-06-01",
		"settings": {
			"matrix": {
				"useRSX": true,
				"useWozduhCross": true,
				"useRedCross": true,
				"useGeometry": true,
				"useGeometryBounce": true,
				"useGeometryTriangle": true,
				"useTrendlines": true,
				"useDivergence": true,
				"useFib": true,
				"useExpRegime": true,
				"useJurikTrend": true,
				"useWozduhSpike": true,
				"useAD": true,
				"useAOCross": true
			},
			"navigators": {
				"price": {
					"enabled": true,
					"source": "Price",
					"trendType": "Wicks",
					"term": "Long",
					"useLong": true,
					"longLen": 60,
					"useMedium": true,
					"mediumLen": 30,
					"useShort": true,
					"shortLen": 10,
					"momentumEnabled": false,
					"momentumBars": 14,
					"momentumPercent": 100,
					"timeHoldEnabled": false,
					"timeHoldBars": 2,
					"barColor": false,
					"backgroundColor": false
				},
				"rsx": {
					"enabled": false,
					"source": "RSX",
					"useLong": true,
					"longLen": 60
				},
				"wozduh": {
					"enabled": false,
					"source": "Wozduh",
					"useLong": true,
					"longLen": 60
				}
			},
			"risk": {
				"risk_per_trade": 1,
				"max_drawdown": 5,
				"leverage": 10,
				"stop_loss_type": "fractal_atr",
				"atr_multiplier": 1.5
			}
		}
	}`

	var req struct {
		Settings *strategy.BacktestRunSettings `json:"settings"`
	}
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Settings == nil {
		t.Fatal("settings is nil")
	}
	if !req.Settings.Matrix.UseTrendlines || !req.Settings.Matrix.UseRSX {
		t.Fatalf("matrix not parsed: %+v", req.Settings.Matrix)
	}
	price, ok := req.Settings.Navigators["price"]
	if !ok || !price.Enabled || !price.UseLong || price.LongLen != 60 {
		t.Fatalf("price navigator not parsed: %+v ok=%v", price, ok)
	}
	if req.Settings.Risk == nil || req.Settings.Risk.Leverage != 10 {
		t.Fatalf("risk not parsed: %+v", req.Settings.Risk)
	}

	matrix := strategy.ResolveBacktestMatrix(req.Settings)
	if !matrix.UseTrendlines {
		t.Fatalf("resolved matrix missing trendlines: %+v", matrix)
	}
	navs := strategy.ResolveBacktestNavigators(req.Settings, nil, strategy.NavigatorUISettings{})
	if len(navs) != 3 {
		t.Fatalf("resolved navigators len = %d, want 3", len(navs))
	}
	if !navs["price"].Enabled || navs["price"].Source != "Price" {
		t.Fatalf("resolved price navigator: %+v", navs["price"])
	}
}

func TestResolveBacktestNavigators_ForcesSourceFromPaneKey(t *testing.T) {
	settings := &strategy.BacktestRunSettings{
		Navigators: map[string]strategy.NavigatorUISettings{
			"rsx": {
				Enabled: true,
				Source:  "Price",
				UseLong: true,
				LongLen: 30,
			},
		},
	}
	navs := strategy.ResolveBacktestNavigators(settings, nil, strategy.NavigatorUISettings{})
	rsx := navs["rsx"]
	if rsx.Source != "RSX" {
		t.Fatalf("expected RSX source, got %q", rsx.Source)
	}
}

func TestResolveBacktestMatrix_FallsBackWhenEmpty(t *testing.T) {
	strategy.ResetScoringMatrix()
	t.Cleanup(strategy.ResetScoringMatrix)

	strategy.SetScoringMatrix(strategy.ScoringMatrix{
		UseRSX:        true,
		UseTrendlines: true,
	})

	empty := &strategy.BacktestRunSettings{}
	m := strategy.ResolveBacktestMatrix(empty)
	if !m.UseRSX || !m.UseTrendlines {
		t.Fatalf("expected global matrix fallback, got %+v", m)
	}
}

func TestApplyMtfOptionsToNavigators(t *testing.T) {
	navs := map[string]strategy.NavigatorUISettings{
		"price": {
			Enabled: true,
			Periods: []string{"15m"},
		},
	}
	strategy.ApplyMtfOptionsToNavigators(navs, map[string]bool{"4h": true, "1d": true})
	periods := navs["price"].Periods
	if len(periods) != 3 {
		t.Fatalf("periods = %v, want 3 entries", periods)
	}
	seen := map[string]bool{}
	for _, p := range periods {
		seen[p] = true
	}
	if !seen["15m"] || !seen["4h"] || !seen["1d"] {
		t.Fatalf("missing expected periods: %v", periods)
	}

	strategy.ApplyMtfOptionsToNavigators(navs, map[string]bool{"4h": false})
	seen = map[string]bool{}
	for _, p := range navs["price"].Periods {
		seen[p] = true
	}
	if seen["4h"] {
		t.Fatalf("4h should be removed after disable: %v", navs["price"].Periods)
	}
}
