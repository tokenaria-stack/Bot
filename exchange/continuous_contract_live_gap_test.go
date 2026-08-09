package exchange

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"trading_bot/data"
)

// Against the real archive: BeforeEnd at the former Aug1–Aug7 hole boundary
// must return ~N actual older rows (count-based). After archive repair the
// window is continuous — time-span lookback also fills, so we no longer assert
// span collapse here (that proof lives in TestLoadContinuousContractBeforeEnd_GapYieldsNewBars).
func TestLiveArchive_1mFormerGap_BeforeEndProgress(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dbPath := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "history.db"))
	if _, err := os.Stat(dbPath); err != nil {
		t.Skip("history.db not present")
	}

	data.ResetDBForTest(dbPath)
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}

	const (
		boundaryMs = int64(1_786_077_480_000) // former gap-end candle
		limit      = 3000
		warmup     = 300
	)
	wantBars := limit + warmup

	got, err := LoadContinuousContractBeforeEnd("BTCUSDT", "1m", boundaryMs, wantBars)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < limit {
		t.Fatalf("before-end len=%d want >= %d", len(got), limit)
	}
	newBars := 0
	for _, c := range got {
		if c.OpenTime < boundaryMs {
			newBars++
		}
	}
	if newBars < limit-1 {
		t.Fatalf("newBars=%d — zero-progress boundary payload still present", newBars)
	}

	// Continuity through the repaired interior (Aug1 18:34 → Aug7 04:37).
	rows, err := data.LoadKlines("BTCUSDT", "1m", 1_785_609_240_000, 1_786_077_420_000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 7800 {
		t.Fatalf("repaired interior rows=%d want >= 7800", len(rows))
	}
	step, err := data.IntervalDurationMs("1m")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].OpenTime-rows[i-1].OpenTime != step {
			t.Fatalf("discontinuity at i=%d %d → %d", i, rows[i-1].OpenTime, rows[i].OpenTime)
		}
	}
}
