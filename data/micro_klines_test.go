package data

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func microBar(openMs int64, closePx float64) Candle {
	return Candle{
		OpenTime:  openMs,
		Open:      closePx,
		High:      closePx + 1,
		Low:       closePx - 1,
		Close:     closePx,
		Volume:    1,
		CloseTime: openMs + 999,
	}
}

func TestSaveMicroKlines_ClosedOnlySparseNoGaps(t *testing.T) {
	ResetDBForTest(filepath.Join(t.TempDir(), "micro.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := microBar(1_001, 10)
	b := microBar(3_001, 11)
	if err := SaveMicroKlines("BTCUSDT", "1s", []Candle{a, b}); err != nil {
		t.Fatal(err)
	}
	if err := SaveMicroKlines("BTCUSDT", "1s", []Candle{b}); err != nil {
		t.Fatal(err)
	}
	n, err := CountMicroKlines("BTCUSDT", "1s")
	if err != nil || n != 2 {
		t.Fatalf("micro count=%d err=%v want 2", n, err)
	}
	hist, err := CountHistoricalKlines("BTCUSDT", "1s")
	if err != nil || hist != 0 {
		t.Fatalf("historical 1s rows=%d err=%v", hist, err)
	}
	gaps, err := ListArchiveGaps("BTCUSDT", "1s", 32)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 0 {
		t.Fatalf("archive_gaps=%d want 0", len(gaps))
	}
	if err := SaveKlines("BTCUSDT", "1s", []Candle{a}); err == nil {
		t.Fatal("SaveKlines must refuse 1s")
	}
}

func TestLoadMicroKlines_PaginationHasMore(t *testing.T) {
	ResetDBForTest(filepath.Join(t.TempDir(), "micro_page.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	rows := make([]Candle, 5)
	for i := 0; i < 5; i++ {
		rows[i] = microBar(int64(i+1)*1000, float64(i))
	}
	if err := SaveMicroKlines("BTCUSDT", "1s", rows); err != nil {
		t.Fatal(err)
	}
	latest, err := LoadMicroKlinesBeforeEnd("BTCUSDT", "1s", 10_000, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 3 || latest[0].OpenTime != 3_000 || latest[2].OpenTime != 5_000 {
		t.Fatalf("latest=%v", latest)
	}
	older, err := HasOlderMicroKline("BTCUSDT", "1s", latest[0].OpenTime)
	if err != nil || !older {
		t.Fatalf("hasMore at head=%v err=%v", older, err)
	}
	prev, err := LoadMicroKlinesBeforeEnd("BTCUSDT", "1s", latest[0].OpenTime-1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(prev) != 2 || prev[0].OpenTime != 1_000 || prev[1].OpenTime != 2_000 {
		t.Fatalf("prev=%v", prev)
	}
	more, err := HasOlderMicroKline("BTCUSDT", "1s", prev[0].OpenTime)
	if err != nil || more {
		t.Fatalf("hasMore at db head=%v err=%v", more, err)
	}
}

func TestLoadMicroKlines_AfterStartAndHasNewer(t *testing.T) {
	ResetDBForTest(filepath.Join(t.TempDir(), "micro_fwd.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	rows := make([]Candle, 5)
	for i := 0; i < 5; i++ {
		rows[i] = microBar(int64(i+1)*1000, float64(i))
	}
	if err := SaveMicroKlines("BTCUSDT", "1s", rows); err != nil {
		t.Fatal(err)
	}
	got, err := LoadMicroKlinesAfterStart("BTCUSDT", "1s", 2_000, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].OpenTime != 3_000 || got[2].OpenTime != 5_000 {
		t.Fatalf("after=%v", got)
	}
	newer, err := HasNewerMicroKline("BTCUSDT", "1s", 4_000)
	if err != nil || !newer {
		t.Fatalf("hasNewer after 4s=%v err=%v", newer, err)
	}
	done, err := HasNewerMicroKline("BTCUSDT", "1s", 5_000)
	if err != nil || done {
		t.Fatalf("hasNewer at tail=%v err=%v", done, err)
	}
	empty, err := LoadMicroKlinesAfterStart("BTCUSDT", "1s", 5_000, 3)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty after tail=%v err=%v", empty, err)
	}
}

func TestPruneMicroKlines_RemovesOlderThan24hOnly(t *testing.T) {
	ResetDBForTest(filepath.Join(t.TempDir(), "micro_prune.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli() / 1000 * 1000
	old := microBar(now-25*time.Hour.Milliseconds(), 1)
	keep := microBar(now-time.Hour.Milliseconds(), 2)
	if err := SaveMicroKlines("BTCUSDT", "1s", []Candle{old, keep}); err != nil {
		t.Fatal(err)
	}
	n, err := PruneMicroKlinesBefore(now - MicroKlineRetention.Milliseconds())
	if err != nil || n != 1 {
		t.Fatalf("pruned=%d err=%v", n, err)
	}
	got, err := LoadLatestMicroKlines("BTCUSDT", "1s", 10)
	if err != nil || len(got) != 1 || got[0].OpenTime != keep.OpenTime {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestPersistenceQueue_Routes1sToMicroNotSaveKlines(t *testing.T) {
	ResetDBForTest(filepath.Join(t.TempDir(), "micro_q.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	q := NewPersistenceQueue(8)
	nativeCalls := 0
	q.SetSaveFunc(func(symbol, interval string, klines []Candle) error {
		nativeCalls++
		return SaveKlines(symbol, interval, klines)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)
	bar := microBar(2_000, 5)
	if !q.Enqueue("BTCUSDT", "1s", bar) {
		t.Fatal("enqueue")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		n, _ := CountMicroKlines("BTCUSDT", "1s")
		if n == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	n, _ := CountMicroKlines("BTCUSDT", "1s")
	if n != 1 {
		t.Fatalf("micro rows=%d", n)
	}
	if nativeCalls != 0 {
		t.Fatalf("SaveKlines calls=%d", nativeCalls)
	}
	h, _ := CountHistoricalKlines("BTCUSDT", "1s")
	if h != 0 {
		t.Fatalf("historical 1s=%d", h)
	}
}
