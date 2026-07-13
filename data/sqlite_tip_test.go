package data

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteTipNeedsCatchUp_EmptyArchive(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "tip_empty.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	needs, tip, err := SQLiteTipNeedsCatchUp("BTCUSDT", "1m", now)
	if err != nil {
		t.Fatal(err)
	}
	if !needs || tip != 0 {
		t.Fatalf("empty archive: needs=%v tip=%d, want needs=true tip=0", needs, tip)
	}
}

func TestSQLiteTipNeedsCatchUp_FreshTip(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "tip_fresh.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	intervalMs, err := IntervalDurationMs("1m")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	endMs, err := CapKlineEndToLastClosed(now, "1m")
	if err != nil {
		t.Fatal(err)
	}
	// Tip at last closed — within lag threshold.
	row := []Candle{{
		OpenTime: endMs, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, CloseTime: endMs + intervalMs - 1,
	}}
	if err := SaveKlines("BTCUSDT", "1m", row); err != nil {
		t.Fatal(err)
	}
	needs, tip, err := SQLiteTipNeedsCatchUp("BTCUSDT", "1m", now)
	if err != nil {
		t.Fatal(err)
	}
	if needs {
		t.Fatalf("fresh tip should not need catch-up; tip=%d end=%d", tip, endMs)
	}
	if tip != endMs {
		t.Fatalf("tip=%d want %d", tip, endMs)
	}
}

func TestSQLiteTipNeedsCatchUp_StaleTip(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "tip_stale.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	intervalMs, err := IntervalDurationMs("1m")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	endMs, err := CapKlineEndToLastClosed(now, "1m")
	if err != nil {
		t.Fatal(err)
	}
	// Tip older than 2 closed bars → needs catch-up.
	staleTip := endMs - 3*intervalMs
	row := []Candle{{
		OpenTime: staleTip, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, CloseTime: staleTip + intervalMs - 1,
	}}
	if err := SaveKlines("BTCUSDT", "1m", row); err != nil {
		t.Fatal(err)
	}
	needs, tip, err := SQLiteTipNeedsCatchUp("BTCUSDT", "1m", now)
	if err != nil {
		t.Fatal(err)
	}
	if !needs {
		t.Fatalf("stale tip should need catch-up; tip=%d end=%d", tip, endMs)
	}
	if tip != staleTip {
		t.Fatalf("tip=%d want %d", tip, staleTip)
	}
}

func TestSQLiteCatchUpWindow_CapsHugeHole(t *testing.T) {
	intervalMs, err := IntervalDurationMs("1m")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	endMs, err := CapKlineEndToLastClosed(now, "1m")
	if err != nil {
		t.Fatal(err)
	}
	tip := endMs - int64(SQLiteCatchUpMaxBars+500)*intervalMs
	start, end, ok, err := SQLiteCatchUpWindow(tip, now, "1m")
	if err != nil || !ok {
		t.Fatalf("window err=%v ok=%v", err, ok)
	}
	wantStart := tip + intervalMs
	if start != wantStart {
		t.Fatalf("start=%d want %d", start, wantStart)
	}
	maxSpan := int64(SQLiteCatchUpMaxBars-1) * intervalMs
	if end-start != maxSpan {
		t.Fatalf("span=%d want %d (capped)", end-start, maxSpan)
	}
	if end >= endMs {
		// Cap should leave end before full wall tip for progressive fill.
		t.Fatalf("huge hole must be chunked: end=%d wallEnd=%d", end, endMs)
	}
}

func TestSQLiteCatchUpWindow_EmptySeedsTail(t *testing.T) {
	intervalMs, err := IntervalDurationMs("15m")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	endMs, err := CapKlineEndToLastClosed(now, "15m")
	if err != nil {
		t.Fatal(err)
	}
	start, end, ok, err := SQLiteCatchUpWindow(0, now, "15m")
	if err != nil || !ok {
		t.Fatalf("window err=%v ok=%v", err, ok)
	}
	if end != endMs {
		t.Fatalf("end=%d want %d", end, endMs)
	}
	wantStart := endMs - int64(SQLiteCatchUpMaxBars-1)*intervalMs
	if start != wantStart {
		t.Fatalf("start=%d want %d", start, wantStart)
	}
}
