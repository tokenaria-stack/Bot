package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestMergeKlinesByOpenTime_OverlayWins(t *testing.T) {
	t.Parallel()

	db := []exchange.Kline{
		{OpenTime: 1_700_000_000_000, Close: 10, High: 11, Low: 9},
		{OpenTime: 1_700_000_060_000, Close: 20, High: 21, Low: 19},
	}
	live := []exchange.Kline{
		{OpenTime: 1_700_000_060_000, Close: 25, High: 26, Low: 24},
		{OpenTime: 1_700_000_120_000, Close: 30, High: 31, Low: 29},
	}
	got := mergeKlinesByOpenTime(db, live)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[1].Close != 25 {
		t.Fatalf("overlay bar close = %v, want 25", got[1].Close)
	}
	if got[2].OpenTime != 1_700_000_120_000 {
		t.Fatalf("tail open = %d, want %d", got[2].OpenTime, 1_700_000_120_000)
	}
}
