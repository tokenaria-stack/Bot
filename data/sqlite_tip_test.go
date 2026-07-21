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
		// Boundary path: NextBarOpen(tip) ≡ tip+step for fixed TFs.
		next, nerr := NextBarOpen(tip, "1m")
		if nerr != nil || start != next {
			t.Fatalf("start=%d want %d (or NextBarOpen=%d err=%v)", start, wantStart, next, nerr)
		}
	}
	maxEnd, err := AdvanceBarOpen(start, SQLiteCatchUpMaxBars-1, "1m")
	if err != nil {
		t.Fatal(err)
	}
	if end != maxEnd {
		t.Fatalf("end=%d want capped AdvanceBarOpen %d", end, maxEnd)
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
	retreated, err := RetreatBarOpen(endMs, SQLiteCatchUpMaxBars-1, "15m")
	if err != nil {
		t.Fatal(err)
	}
	if start != retreated {
		t.Fatalf("start=%d want RetreatBarOpen %d (legacy %d)", start, retreated, wantStart)
	}
}

func TestSQLiteCatchUpWindow_WeeklyMissingMondays(t *testing.T) {
	t.Parallel()
	// Production tip freeze: last archived Monday 2026-06-22; Cap must pull
	// 2026-06-29, 2026-07-06, 2026-07-13 — never epoch-Thursday opens.
	const tipJun22 = int64(1782086400000)
	jun29 := int64(1782691200000)
	jul06 := int64(1783296000000)
	jul13 := int64(1783900800000)

	now := time.Now().UnixMilli()
	capEnd, err := CapKlineEndToLastClosed(now, "1w")
	if err != nil {
		t.Fatal(err)
	}
	if time.UnixMilli(capEnd).UTC().Weekday() != time.Monday {
		t.Fatalf("1w Cap=%v want Monday", time.UnixMilli(capEnd).UTC())
	}
	// Guard: this test assumes wall clock is still in/after the week of 2026-07-20
	// so Cap is at least Jul 13. If run far in the future Cap moves forward — still OK
	// as long as start is Jun 29 and path is Monday-only through Cap.
	if capEnd < jul13 {
		t.Skipf("wall Cap %v before Jul 13 fixture — skip", time.UnixMilli(capEnd).UTC())
	}

	start, end, ok, err := SQLiteCatchUpWindow(tipJun22, now, "1w")
	if err != nil || !ok {
		t.Fatalf("window err=%v ok=%v", err, ok)
	}
	if start != jun29 {
		t.Fatalf("catch-up start=%v want Jun 29 Monday", time.UnixMilli(start).UTC())
	}
	if end != capEnd {
		t.Fatalf("catch-up end=%d want Cap %d", end, capEnd)
	}

	var opens []int64
	cur := tipJun22
	for cur < capEnd {
		next, nerr := NextBarOpen(cur, "1w")
		if nerr != nil {
			t.Fatal(nerr)
		}
		cur = next
		opens = append(opens, cur)
		if time.UnixMilli(cur).UTC().Weekday() != time.Monday {
			t.Fatalf("repaired open %v is not Monday", time.UnixMilli(cur).UTC())
		}
	}
	if len(opens) < 3 || opens[0] != jun29 || opens[1] != jul06 || opens[2] != jul13 {
		t.Fatalf("expected leading repairs [Jun29,Jul06,Jul13], got %v", opens[:min(3, len(opens))])
	}
	// Must not look like Thursday epoch grid near Cap.
	step7 := int64(7 * 24 * 60 * 60 * 1000)
	settled := now - KlineSettleGraceMs
	epochThu := (settled/step7)*step7 - step7
	for _, ot := range opens {
		if ot == epochThu {
			t.Fatalf("repaired open equals epoch-Thursday Cap %v", time.UnixMilli(epochThu).UTC())
		}
	}
}

func TestSQLiteTipNeedsCatchUp_WeeklyStaleThenHealed(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "tip_1w.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	const tipJun22 = int64(1782086400000)
	jun29 := int64(1782691200000)
	jul06 := int64(1783296000000)
	jul13 := int64(1783900800000)

	now := time.Now().UnixMilli()
	capEnd, err := CapKlineEndToLastClosed(now, "1w")
	if err != nil {
		t.Fatal(err)
	}
	if capEnd < jul13 {
		t.Skipf("wall Cap before Jul 13 fixture")
	}

	ct, _ := BarCloseTimeMs(tipJun22, "1w")
	if err := SaveKlines("BTCUSDT", "1w", []Candle{{
		OpenTime: tipJun22, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, CloseTime: ct,
	}}); err != nil {
		t.Fatal(err)
	}
	needs, tip, err := SQLiteTipNeedsCatchUp("BTCUSDT", "1w", now)
	if err != nil {
		t.Fatal(err)
	}
	if !needs || tip != tipJun22 {
		t.Fatalf("stale 1w: needs=%v tip=%d", needs, tip)
	}

	var rows []Candle
	for _, ot := range []int64{tipJun22, jun29, jul06, jul13} {
		c, _ := BarCloseTimeMs(ot, "1w")
		rows = append(rows, Candle{OpenTime: ot, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, CloseTime: c})
	}
	// If Cap moved past Jul 13, fill through Cap with NextBarOpen.
	for ot := jul13; ot < capEnd; {
		ot, err = NextBarOpen(ot, "1w")
		if err != nil {
			t.Fatal(err)
		}
		c, _ := BarCloseTimeMs(ot, "1w")
		rows = append(rows, Candle{OpenTime: ot, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, CloseTime: c})
	}
	if err := SaveKlines("BTCUSDT", "1w", rows); err != nil {
		t.Fatal(err)
	}
	needs, tip, err = SQLiteTipNeedsCatchUp("BTCUSDT", "1w", now)
	if err != nil {
		t.Fatal(err)
	}
	if needs {
		t.Fatalf("healed 1w should not need catch-up; tip=%d Cap=%d", tip, capEnd)
	}
	if tip != capEnd {
		t.Fatalf("tip=%d want Cap %d", tip, capEnd)
	}
}
