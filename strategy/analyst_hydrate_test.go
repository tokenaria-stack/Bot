package strategy

import (
	"testing"
	"time"

	"trading_bot/exchange"
)

func TestMarker_LoadHistoricalKlines_ConcurrentUpdateNoDeadlock(t *testing.T) {
	m := NewMarker(nil, nil, "1m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	base := int64(1_700_000_000_000)

	updatesDone := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			m.UpdateKlineTick(exchange.Kline{
				OpenTime: base + int64(i)*60_000,
				Close:    100 + float64(i),
				High:     101 + float64(i),
				Low:      99 + float64(i),
				Volume:   1,
			}, i%10 == 0)
		}
		close(updatesDone)
	}()

	hydrateDone := make(chan struct{})
	go func() {
		history := make([]exchange.Kline, 400)
		for i := range history {
			history[i] = exchange.Kline{
				OpenTime: base + int64(i)*60_000,
				Close:    50 + float64(i%20),
				High:     52 + float64(i%20),
				Low:      48 + float64(i%20),
				Volume:   1,
			}
		}
		m.LoadHistoricalKlines(history)
		close(hydrateDone)
	}()

	timeout := time.After(5 * time.Second)
	for _, ch := range []chan struct{}{updatesDone, hydrateDone} {
		select {
		case <-ch:
		case <-timeout:
			t.Fatal("concurrent hydrate/update timed out — possible deadlock")
		}
	}
}

func TestRSXMarkerState_SnapshotIgnoresGlobalMutation(t *testing.T) {
	ResetRSXSettings()
	t.Cleanup(ResetRSXSettings)

	ApplyRSXSettings(RSXSettings{DivLookback: 30, Source: "close", DivMethod: "tv"})
	state := newRSXMarkerStateFromSettings(GetRSXSettings())
	for i := 0; i < 40; i++ {
		state.appendBar(120, 100, 118, 50+float64(i%5))
	}
	baseline := len(state.tvCloses)

	ApplyRSXSettings(RSXSettings{DivLookback: 30, Source: "hlc3", DivMethod: "fractal", PivotRadius: 3})
	state.appendBar(120, 100, 118, 55)
	if len(state.tvCloses) != baseline+1 {
		t.Fatalf("fractal flip should not reset TV state: tv len %d want %d", len(state.tvCloses), baseline+1)
	}
	if state.cfg.useFractal {
		t.Fatal("cfg.useFractal should remain false after global flip")
	}
}
