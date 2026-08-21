package data

import (
	"path/filepath"
	"testing"
	"time"
)

func TestArchiveGapIsCurrentNeighbor_RealHole(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "gap_neighbor_real.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	const step int64 = 60_000
	a := (int64(1_700_000_000_000) / step) * step
	z := a + 5*step
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(a), gapTestBar(z)}); err != nil {
		t.Fatal(err)
	}
	ok, err := ArchiveGapIsCurrentNeighbor("BTCUSDT", "1m", a, z)
	if err != nil || !ok {
		t.Fatalf("real neighbor hole: ok=%v err=%v", ok, err)
	}
}

func TestArchiveGapIsCurrentNeighbor_PhantomInterior(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "gap_neighbor_phantom.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	const step int64 = 60_000
	a := (int64(1_700_000_000_000) / step) * step
	z := a + 5*step
	if err := SaveKlines("BTCUSDT", "1m", gapTestBars(a, 6)); err != nil {
		t.Fatal(err)
	}
	if err := RecordArchiveGap("BTCUSDT", "1m", a, z); err != nil {
		t.Fatal(err)
	}
	ok, err := ArchiveGapIsCurrentNeighbor("BTCUSDT", "1m", a, z)
	if err != nil || ok {
		t.Fatalf("interior bar should stale the (A,Z) pair: ok=%v err=%v", ok, err)
	}
}

func TestArchiveGapIsCurrentNeighbor_Contiguous(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "gap_neighbor_contig.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	const step int64 = 60_000
	a := (int64(1_700_000_000_000) / step) * step
	b := a + step
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(a), gapTestBar(b)}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO archive_gaps (symbol, interval, after_open_ms, before_open_ms, status, reason)
VALUES ('BTCUSDT', '1m', ?, ?, 'open', '')`, a, b); err != nil {
		t.Fatal(err)
	}
	ok, err := ArchiveGapIsCurrentNeighbor("BTCUSDT", "1m", a, b)
	if err != nil || ok {
		t.Fatalf("adjacent bars are not a gap: ok=%v err=%v", ok, err)
	}
}

func TestArchiveGapIsCurrentNeighbor_NoSuccessor(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "gap_neighbor_tip.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	const step int64 = 60_000
	a := (int64(1_700_000_000_000) / step) * step
	z := a + 5*step
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(a)}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO archive_gaps (symbol, interval, after_open_ms, before_open_ms, status, reason)
VALUES ('BTCUSDT', '1m', ?, ?, 'open', '')`, a, z); err != nil {
		t.Fatal(err)
	}
	ok, err := ArchiveGapIsCurrentNeighbor("BTCUSDT", "1m", a, z)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("missing successor is not a current neighbor pair (no REST-to-tip policy)")
	}
}

func TestArchiveGapIsCurrentNeighbor_WeekUsesNextBarOpen(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "gap_neighbor_1w.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli() // Monday
	week, err := NextBarOpen(a, "1w")
	if err != nil {
		t.Fatal(err)
	}
	skip, err := NextBarOpen(week, "1w")
	if err != nil {
		t.Fatal(err)
	}
	bar := func(ot int64) Candle {
		closeMs, err := BarCloseTimeMs(ot, "1w")
		if err != nil {
			t.Fatal(err)
		}
		return Candle{OpenTime: ot, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, CloseTime: closeMs}
	}
	if err := SaveKlines("BTCUSDT", "1w", []Candle{bar(a), bar(skip)}); err != nil {
		t.Fatal(err)
	}
	ok, err := ArchiveGapIsCurrentNeighbor("BTCUSDT", "1w", a, skip)
	if err != nil || !ok {
		t.Fatalf("1w hole A→A+14d should be current: ok=%v err=%v nextWeek=%d", ok, err, week)
	}
	if err := SaveKlines("BTCUSDT", "1w", []Candle{bar(week)}); err != nil {
		t.Fatal(err)
	}
	if err := RecordArchiveGap("BTCUSDT", "1w", a, skip); err != nil {
		t.Fatal(err)
	}
	ok, err = ArchiveGapIsCurrentNeighbor("BTCUSDT", "1w", a, skip)
	if err != nil || ok {
		t.Fatalf("after inserting the in-between week, (A,skip) is stale: ok=%v err=%v", ok, err)
	}
}
