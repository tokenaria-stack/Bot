package data

import (
	"path/filepath"
	"testing"
)

// seedGapArchive writes contiguous bars, a multi-day hole, then a single boundary candle.
// Mirrors the live 1m defect: time-span lookback sees only the boundary; count-based sees older rows.
func seedGapArchive(t *testing.T, symbol, interval string, stepMs int64) (boundaryOpen int64, olderCount int) {
	t.Helper()
	resetDBConnection(filepath.Join(t.TempDir(), "gap_history.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}

	const olderN = 40
	base := int64(1_700_000_000_000)
	older := make([]Candle, olderN)
	for i := 0; i < olderN; i++ {
		open := base + int64(i)*stepMs
		older[i] = Candle{
			OpenTime: open, Open: 100, High: 101, Low: 99, Close: float64(100 + i),
			Volume: 1, CloseTime: open + stepMs - 1,
		}
	}
	if err := SaveKlines(symbol, interval, older); err != nil {
		t.Fatal(err)
	}

	// Gap >> N×interval so assumed time-span window contains only the boundary.
	boundaryOpen = older[olderN-1].OpenTime + stepMs*10_000
	boundary := []Candle{{
		OpenTime: boundaryOpen, Open: 200, High: 201, Low: 199, Close: 200.5,
		Volume: 2, CloseTime: boundaryOpen + stepMs - 1,
	}}
	if err := SaveKlines(symbol, interval, boundary); err != nil {
		t.Fatal(err)
	}
	return boundaryOpen, olderN
}

func TestLoadKlinesBeforeEnd_ContiguousReturnsN(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "contiguous.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	step := int64(60_000)
	rows := make([]Candle, 20)
	for i := range rows {
		open := int64(1_700_000_000_000 + int64(i)*step)
		rows[i] = Candle{
			OpenTime: open, Open: 1, High: 2, Low: 1, Close: float64(i),
			Volume: 1, CloseTime: open + step - 1,
		}
	}
	if err := SaveKlines("BTCUSDT", "1m", rows); err != nil {
		t.Fatal(err)
	}
	got, err := LoadKlinesBeforeEnd("BTCUSDT", "1m", rows[19].OpenTime, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("len=%d want 5", len(got))
	}
	if got[0].OpenTime != rows[15].OpenTime || got[4].OpenTime != rows[19].OpenTime {
		t.Fatalf("unexpected range %d..%d", got[0].OpenTime, got[4].OpenTime)
	}
}

func TestLoadKlinesBeforeEnd_GapReturnsOlderRowsNotOnlyBoundary(t *testing.T) {
	step := int64(60_000)
	boundary, olderN := seedGapArchive(t, "BTCUSDT", "1m", step)

	// Time-span formula (old GetWindow path): end - N×interval → only boundary.
	wantBars := 10
	spanStart := boundary - step*int64(wantBars)
	spanRows, err := LoadKlines("BTCUSDT", "1m", spanStart, boundary, wantBars)
	if err != nil {
		t.Fatal(err)
	}
	if len(spanRows) != 1 || spanRows[0].OpenTime != boundary {
		t.Fatalf("time-span control: got %d rows want sole boundary", len(spanRows))
	}

	// Count-based: previous N actual rows cross the gap.
	got, err := LoadKlinesBeforeEnd("BTCUSDT", "1m", boundary, wantBars)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != wantBars {
		t.Fatalf("count-based len=%d want %d (older=%d + boundary)", len(got), wantBars, olderN)
	}
	if got[len(got)-1].OpenTime != boundary {
		t.Fatalf("last must be boundary %d got %d", boundary, got[len(got)-1].OpenTime)
	}
	if got[0].OpenTime >= boundary-step*int64(wantBars) {
		t.Fatalf("oldest %d still inside broken time-span — gap not crossed", got[0].OpenTime)
	}
}

func TestLoadKlinesBeforeEnd_TrueBeginningReturnsRemainder(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "eof.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	step := int64(60_000)
	rows := make([]Candle, 3)
	for i := range rows {
		open := int64(1_700_000_000_000 + int64(i)*step)
		rows[i] = Candle{
			OpenTime: open, Open: 1, High: 2, Low: 1, Close: float64(i),
			Volume: 1, CloseTime: open + step - 1,
		}
	}
	if err := SaveKlines("BTCUSDT", "1m", rows); err != nil {
		t.Fatal(err)
	}
	got, err := LoadKlinesBeforeEnd("BTCUSDT", "1m", rows[2].OpenTime, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3 remainder", len(got))
	}
}

func TestLoadKlinesBeforeEnd_BoundaryOverlapIsSameOpen(t *testing.T) {
	step := int64(60_000)
	boundary, _ := seedGapArchive(t, "BTCUSDT", "1m", step)
	got, err := LoadKlinesBeforeEnd("BTCUSDT", "1m", boundary, 5)
	if err != nil {
		t.Fatal(err)
	}
	// Overlap semantics: newest returned open equals the request boundary (already in FE store).
	if got[len(got)-1].OpenTime != boundary {
		t.Fatalf("boundary open missing from payload")
	}
	newBars := 0
	for _, c := range got {
		if c.OpenTime < boundary {
			newBars++
		}
	}
	if newBars == 0 {
		t.Fatal("zero progress: only boundary candle — gap retrieval failed")
	}
}
