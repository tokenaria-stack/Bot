package data

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestPersistenceQueue_BusyRetryThenPersist(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "busy_retry.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := NewPersistenceQueue(16)
	var calls atomic.Int32
	q.SetSaveFunc(func(symbol, interval string, klines []Candle) error {
		n := calls.Add(1)
		if n <= 2 {
			return errSQLiteBusySentinel
		}
		return SaveKlines(symbol, interval, klines)
	})
	q.Start(ctx)

	ot := int64(1_700_000_000_000)
	if !q.Enqueue("BTCUSDT", "1m", Candle{
		OpenTime: ot, Open: 1, High: 2, Low: 1, Close: 1.5, Volume: 3, CloseTime: ot + 59_999,
	}) {
		t.Fatal("Enqueue rejected")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := LoadKlines("BTCUSDT", "1m", ot, ot, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == 1 {
			if calls.Load() < 3 {
				t.Fatalf("expected retries before success, calls=%d", calls.Load())
			}
			cancel()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("bar not persisted after busy retries; calls=%d failures=%d spill=%d last=%q",
		calls.Load(), q.FailuresCount(), q.SpillLen(), q.LastError())
}

func TestPersistenceQueue_EnqueueNeverDropsUnderPressure(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "enqueue_block.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := NewPersistenceQueue(4)
	// Slow saver so the channel backs up; Enqueue must block, not drop.
	q.SetSaveFunc(func(symbol, interval string, klines []Candle) error {
		time.Sleep(30 * time.Millisecond)
		return SaveKlines(symbol, interval, klines)
	})
	q.Start(ctx)

	open := int64(1_700_000_000_000)
	const n = 12
	done := make(chan struct{})
	go func() {
		for i := 0; i < n; i++ {
			ot := open + int64(i)*60_000
			if !q.Enqueue("BTCUSDT", "1m", Candle{
				OpenTime: ot, Open: 1, High: 2, Low: 1, Close: 1.5, Volume: float64(i + 1), CloseTime: ot + 59_999,
			}) {
				t.Errorf("Enqueue returned false")
			}
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Enqueue stalled under pressure")
	}
	if q.Dropped.Load() != 0 {
		t.Fatalf("Dropped=%d want 0", q.Dropped.Load())
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, err := LoadKlines("BTCUSDT", "1m", open, open+int64(n)*60_000, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == n {
			cancel()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("not all closed bars persisted")
}

func TestArchiveGap_RecordAndNoteFromOpens(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "archive_gaps.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	const step int64 = 60_000
	a := (int64(1_700_000_000_000) / step) * step
	b := a + 5*step // 4 missing 1m bars between
	if err := RecordArchiveGap("BTCUSDT", "1m", a, b); err != nil {
		t.Fatal(err)
	}
	gaps, err := ListArchiveGaps("BTCUSDT", "1m", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 1 || gaps[0].AfterOpenMs != a || gaps[0].BeforeOpenMs != b {
		t.Fatalf("gaps=%v", gaps)
	}
	start, end, ok, err := GapHealWindow(gaps[0])
	if err != nil || !ok {
		t.Fatalf("heal window ok=%v err=%v", ok, err)
	}
	if start != a+step || end != b-step {
		t.Fatalf("window [%d..%d] want [%d..%d]", start, end, a+step, b-step)
	}

	NoteGapsFromOpenTimes("BTCUSDT", "1m", []int64{a, a + step, a + 3*step})
	gaps, err = ListArchiveGaps("BTCUSDT", "1m", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) < 2 {
		t.Fatalf("expected additional gap from opens, got %v", gaps)
	}
}

func TestNotePersistEdges_RecordsNeighborHole(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "persist_edges.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	const step int64 = 60_000
	a := (int64(1_700_000_000_000) / step) * step
	if err := SaveKlines("BTCUSDT", "1m", []Candle{
		{OpenTime: a, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, CloseTime: a + step - 1},
		{OpenTime: a + 5*step, Open: 2, High: 2, Low: 2, Close: 2, Volume: 1, CloseTime: a + 5*step + step - 1},
	}); err != nil {
		t.Fatal(err)
	}
	NotePersistEdges("BTCUSDT", "1m", []Candle{
		{OpenTime: a + 5*step, Open: 2, High: 2, Low: 2, Close: 2, Volume: 1, CloseTime: a + 5*step + step - 1},
	})
	gaps, err := ListArchiveGaps("BTCUSDT", "1m", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) == 0 {
		t.Fatal("expected gap ledger entry for neighbor hole")
	}
}
