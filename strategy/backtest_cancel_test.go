package strategy

import (
	"context"
	"testing"
	"time"

	"trading_bot/exchange"
)

func TestBacktestEngine_Run_CancelledReturnsPartial(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	engine := NewBacktestEngine(BacktestConfig{
		Interval: "1m",
		FeeRate:  DefaultScalpFeeRate,
	})

	candles := make([]exchange.Candle, 200)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range candles {
		ms := base.Add(time.Duration(i) * time.Minute).UnixMilli()
		price := 100.0 + float64(i)*0.01
		candles[i] = exchange.Candle{
			OpenTime: ms,
			Open:     price,
			High:     price + 0.5,
			Low:      price - 0.5,
			Close:    price,
			Volume:   1,
		}
	}

	cancel()

	run, err := engine.Run(ctx, candles)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if run == nil {
		t.Fatal("nil result")
	}
	if !run.Cancelled {
		t.Fatal("expected cancelled=true")
	}
}
