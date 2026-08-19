package data

import (
	"path/filepath"
	"testing"
	"time"
)

// Debt #83 — timestamp contract tests (design phase).
// These encode the intended API: LoadKlines / LoadKlinesBeforeEnd take Unix ms
// and must not re-guess units via ts < 1e12.

const (
	pre2001MsMarch1993 int64 = 730_944_000_000 // 1993-03-01 UTC; ms but < 1e12
	unixMsThreshold    int64 = 1_000_000_000_000
)

func countKlinesDirect(t *testing.T, symbol, interval string, startMs, endMs int64) int {
	t.Helper()
	var n int
	err := db.QueryRow(`
SELECT COUNT(*) FROM historical_klines
WHERE symbol = ? AND interval = ? AND open_time >= ? AND open_time <= ?`,
		normalizeSymbol(symbol), interval, startMs, endMs).Scan(&n)
	if err != nil {
		t.Fatalf("direct count: %v", err)
	}
	return n
}

func insertKlineRaw(t *testing.T, symbol, interval string, openMs, closeMs int64) {
	t.Helper()
	_, err := db.Exec(`
INSERT INTO historical_klines
    (symbol, interval, open_time, open, high, low, close, volume, close_time)
VALUES (?, ?, ?, 1, 1, 1, 1, 1, ?)`,
		normalizeSymbol(symbol), interval, openMs, closeMs)
	if err != nil {
		t.Fatalf("raw insert open=%d: %v", openMs, err)
	}
}

// TestLoadKlines_Pre2001MillisecondBound_MatchesDirectSQL locks the known #83 failure:
// RetreatBarOpen(1M,400) yields 1993-03-01 ms; LoadKlines must not *1000 that bound.
func TestLoadKlines_Pre2001MillisecondBound_MatchesDirectSQL(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "ts_contract_1m.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}

	const symbol, interval = "BTCUSDT", "1M"
	// Three monthly opens around the poisoned bound (raw insert bypasses Save heuristic).
	opens := []int64{
		pre2001MsMarch1993,
		pre2001MsMarch1993 + 31*24*60*60*1000, // ~1993-04
		pre2001MsMarch1993 + 61*24*60*60*1000, // ~1993-05
	}
	for _, ot := range opens {
		insertKlineRaw(t, symbol, interval, ot, ot+30*24*60*60*1000-1)
	}

	startMs := pre2001MsMarch1993
	endMs := opens[len(opens)-1]
	if startMs >= unixMsThreshold {
		t.Fatalf("fixture start %d should be < 1e12 to trip the heuristic", startMs)
	}

	want := countKlinesDirect(t, symbol, interval, startMs, endMs)
	if want != 3 {
		t.Fatalf("fixture direct count=%d want 3", want)
	}

	got, err := LoadKlines(symbol, interval, startMs, endMs, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != want {
		t.Fatalf("LoadKlines rows=%d want %d (bound %d ms must not be coerced to year ~25132)",
			len(got), want, startMs)
	}
	if got[0].OpenTime != startMs {
		t.Fatalf("first OpenTime=%d want bound ms %d", got[0].OpenTime, startMs)
	}
}

// TestLoadKlines_BootLookbackBoundsNeverAltered verifies RetreatBarOpen ms bounds
// are used as-is for representative boot TFs (FrameBoot depth 400).
func TestLoadKlines_BootLookbackBoundsNeverAltered(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "ts_contract_boot.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}

	endWall := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC).UnixMilli()
	tfs := []string{"1m", "3m", "5m", "15m", "1h", "1d", "1w", "1M"}

	for _, tf := range tfs {
		t.Run(tf, func(t *testing.T) {
			endMs := endWall
			if capped, err := CapKlineEndToLastClosed(endMs, tf); err == nil {
				endMs = capped
			}
			startMs, err := RetreatBarOpen(endMs, 400, tf)
			if err != nil {
				t.Fatalf("RetreatBarOpen: %v", err)
			}
			if startMs <= 0 || endMs < startMs {
				t.Fatalf("bad window [%d..%d]", startMs, endMs)
			}

			// One bar at start and one near end so a correct window is non-empty.
			insertKlineRaw(t, "BTCUSDT", tf, startMs, startMs+1)
			mid := startMs + (endMs-startMs)/2
			if mid > startMs && mid < endMs {
				insertKlineRaw(t, "BTCUSDT", tf, mid, mid+1)
			}
			insertKlineRaw(t, "BTCUSDT", tf, endMs, endMs+1)

			want := countKlinesDirect(t, "BTCUSDT", tf, startMs, endMs)
			got, err := LoadKlines("BTCUSDT", tf, startMs, endMs, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != want {
				t.Fatalf("LoadKlines=%d direct=%d startMs=%d (<1e12=%v) — bound must stay Unix ms",
					len(got), want, startMs, startMs < unixMsThreshold)
			}
		})
	}
}

// TestLoadKlinesBeforeEnd_Pre2001MillisecondEnd_Unchanged: endTimeMs is documented ms.
// A widened (*1000) end would incorrectly include a synthetic "year 25132" row.
func TestLoadKlinesBeforeEnd_Pre2001MillisecondEnd_Unchanged(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "ts_contract_before_end.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	ot := pre2001MsMarch1993
	insertKlineRaw(t, "BTCUSDT", "1M", ot, ot+1)
	insertKlineRaw(t, "BTCUSDT", "1M", ot*1000, ot*1000+1)

	got, err := LoadKlinesBeforeEnd("BTCUSDT", "1M", ot, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].OpenTime != ot {
		t.Fatalf("got=%v want exactly one row open=%d (endMs must not be *1000)", got, ot)
	}
}

// TestSaveKlines_CanonicalMillisecondOpenTimeRoundTrip: storage path must not rewrite
// a legitimate pre-2001 ms open (debt #83 Patch B — Save is ms-only, no unit guess).
func TestSaveKlines_CanonicalMillisecondOpenTimeRoundTrip(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "ts_contract_save.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	ot := pre2001MsMarch1993
	row := Candle{
		OpenTime: ot, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, CloseTime: ot + 1,
	}
	if err := SaveKlines("BTCUSDT", "1M", []Candle{row}); err != nil {
		t.Fatal(err)
	}
	var storedOpen, storedClose int64
	err := db.QueryRow(`
SELECT open_time, close_time FROM historical_klines
WHERE symbol = ? AND interval = ?`, "BTCUSDT", "1M").Scan(&storedOpen, &storedClose)
	if err != nil {
		t.Fatal(err)
	}
	if storedOpen != ot || storedClose != ot+1 {
		t.Fatalf("stored open=%d close=%d want open=%d close=%d (no ×1000)",
			storedOpen, storedClose, ot, ot+1)
	}
	got, err := LoadKlines("BTCUSDT", "1M", ot, ot, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].OpenTime != ot {
		t.Fatalf("LoadKlines after Save: %#v want open=%d", got, ot)
	}
}

// TestInitDB_DoesNotDeletePre2001MillisecondRows: InitDB must not delete legitimate
// Unix-ms opens below 1e12 (debt #83 Patch D — obsolete magnitude purge removed).
func TestInitDB_DoesNotDeletePre2001MillisecondRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "init_no_purge.db")
	resetDBConnection(path)
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	const (
		pre2001Ms  int64 = 730_944_000_000   // 1993-03-01 ms
		post2017Ms int64 = 1_502_928_000_000 // spot genesis-shaped
	)
	insertKlineRaw(t, "BTCUSDT", "1M", pre2001Ms, pre2001Ms+1)
	insertKlineRaw(t, "BTCUSDT", "1M", post2017Ms, post2017Ms+1)

	// Cold re-open: InitDB runs full init path again (same as bot restart).
	resetDBConnection(path)
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}

	var n int
	err := db.QueryRow(`
SELECT COUNT(*) FROM historical_klines
WHERE symbol = ? AND interval = ? AND open_time IN (?, ?)`,
		"BTCUSDT", "1M", pre2001Ms, post2017Ms).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("after InitDB rows=%d want 2 (pre-2001 ms must survive)", n)
	}
	got, err := LoadKlines("BTCUSDT", "1M", pre2001Ms, pre2001Ms, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].OpenTime != pre2001Ms {
		t.Fatalf("LoadKlines pre-2001: %#v", got)
	}
}
