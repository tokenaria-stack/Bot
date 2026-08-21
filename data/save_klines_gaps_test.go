package data

import (
	"path/filepath"
	"testing"
)

const gapTestStep int64 = 60_000

func gapTestBar(ot int64) Candle {
	return Candle{
		OpenTime: ot, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1,
		CloseTime: ot + gapTestStep - 1,
	}
}

func gapTestBars(startOT int64, n int) []Candle {
	out := make([]Candle, n)
	for i := 0; i < n; i++ {
		out[i] = gapTestBar(startOT + int64(i)*gapTestStep)
	}
	return out
}

func gapTestOpen(t *testing.T, symbol, interval string) []ArchiveGap {
	t.Helper()
	gaps, err := ListArchiveGaps(symbol, interval, 64)
	if err != nil {
		t.Fatal(err)
	}
	return gaps
}

func gapTestStatus(t *testing.T, after, before int64) (status, reason string, ok bool) {
	t.Helper()
	err := db.QueryRow(`
SELECT status, reason FROM archive_gaps
WHERE symbol = ? AND interval = ? AND after_open_ms = ? AND before_open_ms = ?`,
		"BTCUSDT", "1m", after, before).Scan(&status, &reason)
	if err != nil {
		return "", "", false
	}
	return status, reason, true
}

func TestSaveKlines_ContiguousBatchNoOpenGap(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "gaps_contiguous.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := (int64(1_700_000_000_000) / gapTestStep) * gapTestStep
	if err := SaveKlines("BTCUSDT", "1m", gapTestBars(a, 50)); err != nil {
		t.Fatal(err)
	}
	if gaps := gapTestOpen(t, "BTCUSDT", "1m"); len(gaps) != 0 {
		t.Fatalf("contiguous batch must not create OPEN gaps, got %#v", gaps)
	}
}

func TestSaveKlines_IslandSplitsLargeHole(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "gaps_island.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := (int64(1_700_000_000_000) / gapTestStep) * gapTestStep
	z := a + 100*gapTestStep
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(a)}); err != nil {
		t.Fatal(err)
	}
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(z)}); err != nil {
		t.Fatal(err)
	}
	gaps := gapTestOpen(t, "BTCUSDT", "1m")
	if len(gaps) != 1 || gaps[0].AfterOpenMs != a || gaps[0].BeforeOpenMs != z {
		t.Fatalf("want one hole A→Z, got %#v", gaps)
	}

	b := a + 40*gapTestStep
	c := b + 9*gapTestStep // 10-bar island
	if err := SaveKlines("BTCUSDT", "1m", gapTestBars(b, 10)); err != nil {
		t.Fatal(err)
	}
	gaps = gapTestOpen(t, "BTCUSDT", "1m")
	if len(gaps) != 2 {
		t.Fatalf("island should yield two discontinuities, got %#v", gaps)
	}
	seen := map[gapPair]bool{}
	for _, g := range gaps {
		seen[gapPair{g.AfterOpenMs, g.BeforeOpenMs}] = true
	}
	if !seen[gapPair{a, b}] || !seen[gapPair{c, z}] {
		t.Fatalf("want A→B and C→Z, got %#v", gaps)
	}
}

func TestSaveKlines_LaterFillRemovesGaps(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "gaps_fill.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := (int64(1_700_000_000_000) / gapTestStep) * gapTestStep
	z := a + 30*gapTestStep
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(a), gapTestBar(z)}); err != nil {
		t.Fatal(err)
	}
	if err := SaveKlines("BTCUSDT", "1m", gapTestBars(a+gapTestStep, 29)); err != nil {
		t.Fatal(err)
	}
	if gaps := gapTestOpen(t, "BTCUSDT", "1m"); len(gaps) != 0 {
		t.Fatalf("full fill must clear OPEN gaps, got %#v", gaps)
	}
}

func TestSaveKlines_BatchSizeDoesNotChangeFinalGaps(t *testing.T) {
	a := (int64(1_700_000_000_000) / gapTestStep) * gapTestStep
	z := a + 80*gapTestStep
	fill := func(t *testing.T, chunk int) {
		t.Helper()
		resetDBConnection(filepath.Join(t.TempDir(), "gaps_chunk.db"))
		if err := InitDB(); err != nil {
			t.Fatal(err)
		}
		if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(a), gapTestBar(z)}); err != nil {
			t.Fatal(err)
		}
		for ot := a + gapTestStep; ot < z; ot += int64(chunk) * gapTestStep {
			n := chunk
			if ot+int64(n)*gapTestStep > z {
				n = int((z - ot) / gapTestStep)
			}
			if n <= 0 {
				break
			}
			if err := SaveKlines("BTCUSDT", "1m", gapTestBars(ot, n)); err != nil {
				t.Fatal(err)
			}
		}
		if gaps := gapTestOpen(t, "BTCUSDT", "1m"); len(gaps) != 0 {
			t.Fatalf("chunk=%d leftover gaps %#v", chunk, gaps)
		}
	}
	fill(t, 3)
	fill(t, 17)
	fill(t, 80)
}

func TestSaveKlines_LargeVisionStyleBatchSameInvariant(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "gaps_vision.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := (int64(1_700_000_000_000) / gapTestStep) * gapTestStep
	if err := SaveKlines("BTCUSDT", "1m", gapTestBars(a, 2000)); err != nil {
		t.Fatal(err)
	}
	if gaps := gapTestOpen(t, "BTCUSDT", "1m"); len(gaps) != 0 {
		t.Fatalf("large contiguous SaveKlines must not create gaps, got %#v", gaps)
	}
	z := a + 2500*gapTestStep
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(z)}); err != nil {
		t.Fatal(err)
	}
	gaps := gapTestOpen(t, "BTCUSDT", "1m")
	if len(gaps) != 1 || gaps[0].AfterOpenMs != a+1999*gapTestStep || gaps[0].BeforeOpenMs != z {
		t.Fatalf("large batch then far bar: %#v", gaps)
	}
}

func TestSaveKlines_ExhaustedIdenticalPKNotReopened(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "gaps_exhausted.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := (int64(1_700_000_000_000) / gapTestStep) * gapTestStep
	z := a + 20*gapTestStep
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(a), gapTestBar(z)}); err != nil {
		t.Fatal(err)
	}
	if err := MarkArchiveGapExhausted("BTCUSDT", "1m", a, z, "empty_rest"); err != nil {
		t.Fatal(err)
	}
	// Re-persist the same endpoints (idempotent UPSERT). Must not reopen tombstone.
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(a), gapTestBar(z)}); err != nil {
		t.Fatal(err)
	}
	if gaps := gapTestOpen(t, "BTCUSDT", "1m"); len(gaps) != 0 {
		t.Fatalf("exhausted PK reopened: %#v", gaps)
	}
	status, reason, ok := gapTestStatus(t, a, z)
	if !ok || status != ArchiveGapStatusExhausted || reason != "empty_rest" {
		t.Fatalf("tombstone lost: ok=%v status=%q reason=%q", ok, status, reason)
	}

	// Filling the hole removes the stale exhausted pair.
	if err := SaveKlines("BTCUSDT", "1m", gapTestBars(a+gapTestStep, 19)); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := gapTestStatus(t, a, z); ok {
		t.Fatal("exhausted pair should be deleted once it is no longer a physical neighbor")
	}
	if gaps := gapTestOpen(t, "BTCUSDT", "1m"); len(gaps) != 0 {
		t.Fatalf("fill left OPEN gaps %#v", gaps)
	}
}

func TestSaveKlines_KlinesAndGapsRollbackTogether(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "gaps_rollback.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := (int64(1_700_000_000_000) / gapTestStep) * gapTestStep
	z := a + 10*gapTestStep
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(a)}); err != nil {
		t.Fatal(err)
	}
	before, err := LoadKlines("BTCUSDT", "1m", a, z, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
CREATE TRIGGER test_gap_rollback
BEFORE INSERT ON archive_gaps
BEGIN
  SELECT RAISE(ROLLBACK, 'test_rollback');
END`); err != nil {
		t.Fatal(err)
	}
	err = SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(z)})
	if err == nil {
		t.Fatal("expected rollback from gap INSERT trigger")
	}
	after, err := LoadKlines("BTCUSDT", "1m", a, z, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("klines must roll back with gaps: before=%d after=%d err=%v", len(before), len(after), err)
	}
	if gaps := gapTestOpen(t, "BTCUSDT", "1m"); len(gaps) != 0 {
		t.Fatalf("gap INSERT rolled back, got %#v", gaps)
	}
}
