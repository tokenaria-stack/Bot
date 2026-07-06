package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestScoreDecisionForTelemetryReadOnly(t *testing.T) {
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
	master.SeedClosedBarTelemetry()

	before := marker.ClosedVolatilityRegime()
	decision := master.ScoreDecisionForTelemetry(marker)
	if decision.FinalAction == "" && decision.RawAction == "" {
		t.Fatal("expected seeded closed-bar decision")
	}
	if marker.ClosedVolatilityRegime() != before {
		t.Fatal("ScoreDecisionForTelemetry must not mutate marker layer2 snapshot")
	}

	// Intra-bar tick must not refresh telemetry cache via ScoreDecisionForTelemetry.
	k2 := exchange.Kline{OpenTime: 1_700_000_060_000, CloseTime: 1_700_000_120_000, High: 105, Low: 95, Close: 102, Volume: 50}
	marker.UpdateKlineTick(k2, false)
	_ = master.ScoreDecisionForTelemetry(marker)
	if marker.ClosedVolatilityRegime() != before {
		t.Fatal("ScoreDecisionForTelemetry must remain read-only on intra-bar ticks")
	}
}
