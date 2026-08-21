package data

import (
	"path/filepath"
	"testing"
	"time"
)

func TestReconcileOpenArchiveGaps_PhantomDeletedGenuineRetained(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "reconcile_open.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := (int64(1_700_000_000_000) / gapTestStep) * gapTestStep
	z := a + 5*gapTestStep
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(a), gapTestBar(z)}); err != nil {
		t.Fatal(err)
	}
	p := a + 200*gapTestStep
	if err := SaveKlines("BTCUSDT_SPOT", "1m", gapTestBars(p, 6)); err != nil {
		t.Fatal(err)
	}
	if err := TestingForceOpenArchiveGap("BTCUSDT_SPOT", "1m", p, p+5*gapTestStep); err != nil {
		t.Fatal(err)
	}

	res, err := ReconcileOpenArchiveGaps(8)
	if err != nil {
		t.Fatal(err)
	}
	if res.Deleted != 1 || res.Retained != 1 {
		t.Fatalf("want 1 phantom deleted and 1 genuine retained, got %+v", res)
	}
	gaps := gapTestOpen(t, "BTCUSDT", "1m")
	if len(gaps) != 1 || gaps[0].AfterOpenMs != a || gaps[0].BeforeOpenMs != z {
		t.Fatalf("retained genuine A→Z, got %#v", gaps)
	}
}

func TestReconcileOpenArchiveGaps_ContiguousAndNoSuccessorDeleted(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "reconcile_lies.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := (int64(1_700_000_000_000) / gapTestStep) * gapTestStep
	b := a + gapTestStep
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(a), gapTestBar(b)}); err != nil {
		t.Fatal(err)
	}
	if err := TestingForceOpenArchiveGap("BTCUSDT", "1m", a, b); err != nil {
		t.Fatal(err)
	}
	tip := a + 20*gapTestStep
	const step3 int64 = 180_000
	if err := SaveKlines("BTCUSDT", "3m", []Candle{{
		OpenTime: tip, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, CloseTime: tip + step3 - 1,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := TestingForceOpenArchiveGap("BTCUSDT", "3m", tip, tip+5*step3); err != nil {
		t.Fatal(err)
	}

	res, err := ReconcileOpenArchiveGaps(8)
	if err != nil {
		t.Fatal(err)
	}
	if res.Deleted != 2 || res.Retained != 0 {
		t.Fatalf("both lies should delete, got %+v", res)
	}
}

func TestReconcileOpenArchiveGaps_ExhaustedUntouched(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "reconcile_exh.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := (int64(1_700_000_000_000) / gapTestStep) * gapTestStep
	z := a + 5*gapTestStep
	if err := SaveKlines("BTCUSDT", "1m", []Candle{gapTestBar(a), gapTestBar(z)}); err != nil {
		t.Fatal(err)
	}
	if err := MarkArchiveGapExhausted("BTCUSDT", "1m", a, z, "empty_rest"); err != nil {
		t.Fatal(err)
	}
	res, err := ReconcileOpenArchiveGaps(8)
	if err != nil {
		t.Fatal(err)
	}
	if res.Examined != 0 || res.Deleted != 0 {
		t.Fatalf("exhausted must not be examined, got %+v", res)
	}
	status, reason, ok := gapTestStatus(t, a, z)
	if !ok || status != ArchiveGapStatusExhausted || reason != "empty_rest" {
		t.Fatalf("tombstone mutated: ok=%v %s %s", ok, status, reason)
	}
}

func TestReconcileOpenArchiveGaps_WeekAdjacencyAndSpotIsolation(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "reconcile_iso.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
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

	spotA := (int64(1_600_000_000_000) / gapTestStep) * gapTestStep
	spotZ := spotA + 5*gapTestStep
	if err := SaveKlines("BTCUSDT_SPOT", "1m", []Candle{gapTestBar(spotA), gapTestBar(spotZ)}); err != nil {
		t.Fatal(err)
	}
	if err := TestingForceOpenArchiveGap("BTCUSDT", "1m", spotA, spotZ); err != nil {
		t.Fatal(err)
	}

	res, err := ReconcileOpenArchiveGaps(16)
	if err != nil {
		t.Fatal(err)
	}
	if res.Retained != 2 {
		t.Fatalf("want futures 1w hole + spot 1m hole retained, got %+v", res)
	}
	fut := gapTestOpen(t, "BTCUSDT", "1w")
	if len(fut) != 1 || fut[0].AfterOpenMs != a || fut[0].BeforeOpenMs != skip {
		t.Fatalf("1w genuine lost: %#v", fut)
	}
	spot := gapTestOpen(t, "BTCUSDT_SPOT", "1m")
	if len(spot) != 1 || spot[0].AfterOpenMs != spotA {
		t.Fatalf("spot genuine lost: %#v", spot)
	}
	if n := gapTestOpen(t, "BTCUSDT", "1m"); len(n) != 0 {
		t.Fatalf("futures 1m must not keep a spot-keyed lie: %#v", n)
	}
}

func TestReconcileOpenArchiveGaps_VerifyRemaining(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "reconcile_verify.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := (int64(1_700_000_000_000) / gapTestStep) * gapTestStep
	if err := SaveKlines("BTCUSDT", "1m", gapTestBars(a, 4)); err != nil {
		t.Fatal(err)
	}
	if err := RecordArchiveGap("BTCUSDT", "1m", a, a+3*gapTestStep); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileOpenArchiveGaps(8); err != nil {
		t.Fatal(err)
	}
	bad, err := VerifyRemainingOpenArchiveGaps()
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 0 {
		t.Fatalf("post-drain OPEN must all be current neighbors, got %#v", bad)
	}
}
