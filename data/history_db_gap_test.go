package data

import (
	"testing"
	"time"
)

func TestCapKlineEndToLastClosed_HistoricalEndUnchanged(t *testing.T) {
	t.Parallel()
	end := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC).UnixMilli()
	got, err := CapKlineEndToLastClosed(end, "15m")
	if err != nil {
		t.Fatal(err)
	}
	if got != end {
		t.Fatalf("historical end changed: got %d want %d", got, end)
	}
}

func TestCapKlineEndToLastClosed_ClampsFutureToLastClosedOpen(t *testing.T) {
	t.Parallel()
	stepMs := int64(4 * 60 * 60 * 1000)
	nowMs := time.Now().UnixMilli()
	currentOpen := (nowMs / stepMs) * stepMs
	want := currentOpen - stepMs

	got, err := CapKlineEndToLastClosed(nowMs+stepMs, "4h")
	if err != nil {
		t.Fatal(err)
	}
	if got > nowMs {
		t.Fatalf("end %d exceeds now %d", got, nowMs)
	}
	if got != want {
		t.Fatalf("got %d want last closed open %d", got, want)
	}
}
