package market

import (
	"testing"

	"trading_bot/exchange"
)

func TestHydrateDerivedFramesFromParent(t *testing.T) {
	parents := make([]exchange.Kline, 4)
	for i := 0; i < 4; i++ {
		ot := int64(i) * 60_000
		parents[i] = exchange.Kline{
			OpenTime: ot, CloseTime: ot + 59_999,
			Open: 1, High: 2, Low: 1, Close: 1, Volume: 1,
		}
	}
	frames := map[string]*Frame{
		"1m": NewFrame(parents, "1m", ChaosConfig{}),
	}
	HydrateDerivedFrames(frames, ChaosConfig{})
	child := frames["2m"]
	if child == nil {
		t.Fatal("missing 2m Frame")
	}
	got := child.GetKlines()
	if len(got) < 2 {
		t.Fatalf("hydrated 2m bars=%d want >=2", len(got))
	}
	if got[0].OpenTime != 0 {
		t.Fatalf("first open=%d", got[0].OpenTime)
	}
}
