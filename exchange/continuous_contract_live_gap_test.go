package exchange

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"trading_bot/data"
)

// Cheap proof against the real archive (if present): the known 1m gap near
// open_time 1786077480000 must yield N bars via BeforeEnd, not a sole boundary.
func TestLiveArchive_1mGap_BeforeEndProgress(t *testing.T) {
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
		boundaryMs = int64(1_786_077_480_000) // store-left boundary from live 1m probe
		limit      = 3000
		warmup     = 300 // market.IndicatorWarmupBars — avoid import cycle in tests
	)
	wantBars := limit + warmup
	stepMs, err := data.IntervalDurationMs("1m")
	if err != nil {
		t.Fatal(err)
	}

	spanStart := boundaryMs - stepMs*int64(wantBars)
	span, err := LoadContinuousContractFromDB("BTCUSDT", "1m", spanStart, boundaryMs, wantBars)
	if err != nil {
		t.Fatal(err)
	}
	if len(span) > 5 {
		t.Fatalf("expected time-span collapse near gap, got %d bars", len(span))
	}

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
}
