package data

import (
	"testing"
	"time"
)

func TestBarBoundary_FixedBitIdenticalToStepFloor(t *testing.T) {
	t.Parallel()
	intervals := []string{"1m", "3m", "5m", "15m", "30m", "1h", "4h", "1d"}
	samples := []int64{
		time.Date(2024, 6, 15, 12, 34, 56, 0, time.UTC).UnixMilli(),
		time.Date(2026, 7, 21, 15, 57, 30, 0, time.UTC).UnixMilli(),
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
	}
	for _, iv := range intervals {
		step, err := IntervalDurationMs(iv)
		if err != nil {
			t.Fatalf("%s: %v", iv, err)
		}
		for _, ms := range samples {
			want := (ms / step) * step
			got, err := CurrentBarOpen(ms, iv)
			if err != nil {
				t.Fatalf("%s CurrentBarOpen: %v", iv, err)
			}
			if got != want {
				t.Fatalf("%s CurrentBarOpen(%d)=%d want step-floor %d", iv, ms, got, want)
			}
			prev, err := PreviousBarOpen(got, iv)
			if err != nil {
				t.Fatal(err)
			}
			if prev != got-step {
				t.Fatalf("%s PreviousBarOpen=%d want %d", iv, prev, got-step)
			}
			next, err := NextBarOpen(got, iv)
			if err != nil {
				t.Fatal(err)
			}
			if next != got+step {
				t.Fatalf("%s NextBarOpen=%d want %d", iv, next, got+step)
			}
			ct, err := BarCloseTimeMs(got, iv)
			if err != nil {
				t.Fatal(err)
			}
			if ct != got+step-1 {
				t.Fatalf("%s BarCloseTimeMs=%d want %d", iv, ct, got+step-1)
			}
		}
	}
}

func TestBarBoundary_WeekMondayUTC(t *testing.T) {
	t.Parallel()
	// Wednesday mid-week → Monday 2026-07-20
	wed := time.Date(2026, 7, 22, 15, 0, 0, 0, time.UTC).UnixMilli()
	wantMon := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC).UnixMilli()
	got, err := CurrentBarOpen(wed, "1w")
	if err != nil {
		t.Fatal(err)
	}
	if got != wantMon {
		t.Fatalf("1w CurrentBarOpen=%v want Monday %v",
			time.UnixMilli(got).UTC(), time.UnixMilli(wantMon).UTC())
	}
	// Must NOT be Thursday epoch grid
	step7 := int64(7 * 24 * 60 * 60 * 1000)
	epochFloor := (wed / step7) * step7
	if got == epochFloor {
		t.Fatal("1w CurrentBarOpen must not equal Unix-epoch week floor")
	}
	prev, err := PreviousBarOpen(got, "1w")
	if err != nil {
		t.Fatal(err)
	}
	wantPrev := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC).UnixMilli()
	if prev != wantPrev {
		t.Fatalf("1w Previous=%v want %v", time.UnixMilli(prev).UTC(), time.UnixMilli(wantPrev).UTC())
	}
	next, err := NextBarOpen(got, "1w")
	if err != nil {
		t.Fatal(err)
	}
	wantNext := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC).UnixMilli()
	if next != wantNext {
		t.Fatalf("1w Next=%v want %v", time.UnixMilli(next).UTC(), time.UnixMilli(wantNext).UTC())
	}
}

func TestBarBoundary_MonthFirstUTC(t *testing.T) {
	t.Parallel()
	mid := time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC).UnixMilli()
	want := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	got, err := CurrentBarOpen(mid, "1M")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("1M Current=%v want %v", time.UnixMilli(got).UTC(), time.UnixMilli(want).UTC())
	}
	prev, err := PreviousBarOpen(got, "1M")
	if err != nil {
		t.Fatal(err)
	}
	wantPrev := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	if prev != wantPrev {
		t.Fatalf("1M Previous=%v want Jan 1", time.UnixMilli(prev).UTC())
	}
	next, err := NextBarOpen(got, "1M")
	if err != nil {
		t.Fatal(err)
	}
	wantNext := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	if next != wantNext {
		t.Fatalf("1M Next=%v want Mar 1", time.UnixMilli(next).UTC())
	}
	// Feb close = Feb 28 23:59:59.999 UTC (2026 not leap) = Mar1 - 1ms
	ct, err := BarCloseTimeMs(got, "1M")
	if err != nil {
		t.Fatal(err)
	}
	if ct != wantNext-1 {
		t.Fatalf("1M CloseTime=%d want %d (not Open+30d)", ct, wantNext-1)
	}
	legacy30 := got + 30*24*60*60*1000 - 1
	if ct == legacy30 {
		t.Fatal("1M CloseTime must not equal Open+30d-1")
	}
}

func TestBarBoundary_LeapFebruary(t *testing.T) {
	t.Parallel()
	feb := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC).UnixMilli() // leap year
	next, err := NextBarOpen(feb, "1M")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	if next != want {
		t.Fatalf("leap Feb Next=%v want Mar 1", time.UnixMilli(next).UTC())
	}
	ct, err := BarCloseTimeMs(feb, "1M")
	if err != nil {
		t.Fatal(err)
	}
	// Feb 2024 has 29 days
	if ct != want-1 {
		t.Fatalf("leap Feb close=%d want %d", ct, want-1)
	}
}

func TestBarBoundary_AlgebraicInvariants(t *testing.T) {
	t.Parallel()
	intervals := []string{"1m", "15m", "1h", "4h", "1d", "1w", "1M"}
	anchors := []int64{
		time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC).UnixMilli(), // leap
		time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC).UnixMilli(),   // Monday
		time.Date(2026, 7, 22, 15, 30, 0, 0, time.UTC).UnixMilli(), // mid-week
		time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC).UnixMilli(),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
	}
	for _, iv := range intervals {
		for _, ms := range anchors {
			cur, err := CurrentBarOpen(ms, iv)
			if err != nil {
				t.Fatalf("%s: %v", iv, err)
			}
			cur2, err := CurrentBarOpen(cur, iv)
			if err != nil || cur2 != cur {
				t.Fatalf("%s Current idempotent: %d vs %d", iv, cur2, cur)
			}
			next, err := NextBarOpen(cur, iv)
			if err != nil {
				t.Fatal(err)
			}
			back, err := PreviousBarOpen(next, iv)
			if err != nil {
				t.Fatal(err)
			}
			if back != cur {
				t.Fatalf("%s Previous(Next(open))=%d want %d", iv, back, cur)
			}
			prev, err := PreviousBarOpen(cur, iv)
			if err != nil {
				t.Fatal(err)
			}
			fwd, err := NextBarOpen(prev, iv)
			if err != nil {
				t.Fatal(err)
			}
			if fwd != cur {
				t.Fatalf("%s Next(Previous(open))=%d want %d", iv, fwd, cur)
			}
		}
	}
}

func TestCapKlineEndToLastClosed_FixedBitIdentical(t *testing.T) {
	t.Parallel()
	nowMs := time.Now().UnixMilli()
	settledNow := nowMs - KlineSettleGraceMs
	for _, iv := range []string{"1m", "15m", "1h", "4h", "1d"} {
		step, err := IntervalDurationMs(iv)
		if err != nil {
			t.Fatal(err)
		}
		legacyCurrent := (settledNow / step) * step
		legacyWant := legacyCurrent - step
		got, err := CapKlineEndToLastClosed(nowMs+step, iv)
		if err != nil {
			t.Fatal(err)
		}
		if got != legacyWant {
			t.Fatalf("%s Cap=%d want legacy step Cap %d (fixed TF regression)", iv, got, legacyWant)
		}
	}
}

func TestCapKlineEndToLastClosed_WeekIsMondayNotThursday(t *testing.T) {
	t.Parallel()
	// Cap uses wall clock; assert Cap open is a Monday and ≠ epoch Thursday floor.
	got, err := CapKlineEndToLastClosed(time.Now().UnixMilli(), "1w")
	if err != nil {
		t.Fatal(err)
	}
	gt := time.UnixMilli(got).UTC()
	if gt.Weekday() != time.Monday {
		t.Fatalf("1w Cap weekday=%s want Monday (got %v)", gt.Weekday(), gt)
	}
	if gt.Hour() != 0 || gt.Minute() != 0 || gt.Second() != 0 {
		t.Fatalf("1w Cap must be 00:00 UTC, got %v", gt)
	}
	step7 := int64(7 * 24 * 60 * 60 * 1000)
	settled := time.Now().UnixMilli() - KlineSettleGraceMs
	epochLast := (settled/step7)*step7 - step7
	if got == epochLast {
		t.Fatalf("1w Cap must not equal epoch-Thursday last closed %v", time.UnixMilli(epochLast).UTC())
	}
}

func TestCapKlineEndToLastClosed_MonthIsFirstOfMonth(t *testing.T) {
	t.Parallel()
	got, err := CapKlineEndToLastClosed(time.Now().UnixMilli(), "1M")
	if err != nil {
		t.Fatal(err)
	}
	gt := time.UnixMilli(got).UTC()
	if gt.Day() != 1 || gt.Hour() != 0 {
		t.Fatalf("1M Cap=%v want 1st 00:00 UTC", gt)
	}
}
