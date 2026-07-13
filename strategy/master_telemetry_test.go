package strategy

import (
	"testing"

	"trading_bot/exchange"
)

// Shot 9B: ScoreMatrix/Falcon closed-bar telemetry is frozen (refreshClosedBarTelemetry no-op).
func TestScoreDecisionForTelemetryFrozen_Shot9B(t *testing.T) {
	cfg := ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34}
	marker := NewMarker(nil, nil, "1m", "", cfg)
	master := NewMasterGeneral(
		map[string]*Marker{"1m": marker},
		NewAnalyst(true),
		nil, nil, nil,
		true, true, "BTCUSDT", "1m",
	)

	k := exchange.Kline{OpenTime: 1_700_000_000_000, CloseTime: 1_700_000_060_000, High: 101, Low: 99, Close: 100, Volume: 10}
	marker.UpdateKlineTick(k, true)
	before := marker.ClosedVolatilityRegime()
	master.SeedClosedBarTelemetry()

	decision := master.ScoreDecisionForTelemetry(marker)
	if decision.FinalAction != "" || decision.RawAction != "" || len(decision.Factors) != 0 {
		t.Fatalf("Shot 9B: expected empty frozen telemetry, got %+v", decision)
	}
	if marker.ClosedVolatilityRegime() != before {
		t.Fatal("SeedClosedBarTelemetry must not mutate marker when scoring is frozen")
	}

	k2 := exchange.Kline{OpenTime: 1_700_000_060_000, CloseTime: 1_700_000_120_000, High: 105, Low: 95, Close: 102, Volume: 50}
	marker.UpdateKlineTick(k2, false)
	_ = master.ScoreDecisionForTelemetry(marker)
	if marker.ClosedVolatilityRegime() != before {
		t.Fatal("ScoreDecisionForTelemetry must remain read-only on intra-bar ticks")
	}
}
