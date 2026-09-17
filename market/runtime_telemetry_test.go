package market

import (
	"testing"

	"trading_bot/exchange"
)

// Shot 9B remnant: SeedClosedBarTelemetry must not mutate Frame regime.
func TestSeedClosedBarTelemetryDoesNotMutateRegime(t *testing.T) {
	cfg := ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34}
	marker := NewFrame(nil, "1m", cfg)
	master := NewRuntime(
		map[string]*Frame{"1m": marker},
		nil, nil,
		true, true, "BTCUSDT", "1m",
	)

	k := exchange.Kline{OpenTime: 1_700_000_000_000, CloseTime: 1_700_000_060_000, High: 101, Low: 99, Close: 100, Volume: 10}
	marker.UpdateKlineTick(k, true)
	before := marker.ClosedVolatilityRegime()
	master.SeedClosedBarTelemetry()
	if marker.ClosedVolatilityRegime() != before {
		t.Fatal("SeedClosedBarTelemetry must not mutate marker regime")
	}

	k2 := exchange.Kline{OpenTime: 1_700_000_060_000, CloseTime: 1_700_000_120_000, High: 105, Low: 95, Close: 102, Volume: 50}
	marker.UpdateKlineTick(k2, false)
	if marker.ClosedVolatilityRegime() != before {
		t.Fatal("SeedClosedBarTelemetry must remain read-only on intra-bar ticks")
	}
}
