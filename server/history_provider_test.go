package server

import (
	"testing"

	"trading_bot/exchange"
)

func TestFilterKlinesUntilOpenMs(t *testing.T) {
	t.Parallel()
	step := int64(60_000)
	base := int64(1_700_000_000_000)
	in := []exchange.Kline{
		{OpenTime: base, Close: 1},
		{OpenTime: base + step, Close: 2},
		{OpenTime: base + 2*step, Close: 3},
		{OpenTime: base + 3*step, Close: 4},
	}
	end := base + 2*step
	got := filterKlinesUntilOpenMs(in, end)
	if len(got) != 3 {
		t.Fatalf("len = %d want 3", len(got))
	}
	if got[2].OpenTime != end {
		t.Fatalf("last open = %d want %d", got[2].OpenTime, end)
	}
}

func TestMergeHistoryWindow_RAMFillsSQLiteGap(t *testing.T) {
	t.Parallel()
	step := int64(60_000)
	base := int64(1_700_000_000_000)
	// SQLite tip stops early (stale archive).
	db := []exchange.Kline{
		{OpenTime: base, Open: 1, High: 1, Low: 1, Close: 1, Volume: 10},
		{OpenTime: base + step, Open: 2, High: 2, Low: 2, Close: 2, Volume: 10},
	}
	// RAM continues through the gap + live tip.
	ram := []exchange.Kline{
		{OpenTime: base + step, Open: 2, High: 2.5, Low: 2, Close: 2.2, Volume: 20}, // overlay wins
		{OpenTime: base + 2*step, Open: 3, High: 3, Low: 3, Close: 3, Volume: 30},
		{OpenTime: base + 3*step, Open: 4, High: 4, Low: 4, Close: 4, Volume: 40},
	}
	end := base + 3*step
	merged := mergeKlinesByOpenTime(db, filterKlinesUntilOpenMs(ram, end))
	if len(merged) != 4 {
		t.Fatalf("merged len = %d want 4 (no sync gap)", len(merged))
	}
	for i := 1; i < len(merged); i++ {
		if merged[i].OpenTime-merged[i-1].OpenTime != step {
			t.Fatalf("gap at %d: %d → %d", i, merged[i-1].OpenTime, merged[i].OpenTime)
		}
	}
	// Overlay OHLC from RAM on duplicate open.
	if merged[1].Volume != 20 || merged[1].Close != 2.2 {
		t.Fatalf("overlay not applied: %+v", merged[1])
	}
}

func TestFilterKlinesUntilOpenMs_DeepHistoryExcludesLiveTip(t *testing.T) {
	t.Parallel()
	step := int64(60_000)
	base := int64(1_700_000_000_000)
	ram := []exchange.Kline{
		{OpenTime: base + 100*step, Close: 100},
		{OpenTime: base + 101*step, Close: 101},
	}
	deepEnd := base + 10*step
	got := filterKlinesUntilOpenMs(ram, deepEnd)
	if len(got) != 0 {
		t.Fatalf("deep-history filter must drop live tip, got %d bars", len(got))
	}
}
