package market

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"

	"trading_bot/data"
	"trading_bot/exchange"
)

func gapHealRuntime(t *testing.T) *Runtime {
	t.Helper()
	data.ResetDBForTest(filepath.Join(t.TempDir(), "gap_heal.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}
	q := data.NewPersistenceQueue(16)
	q.SetSaveFunc(data.SaveKlines)
	rt := NewRuntime(nil, &exchange.BinanceExchange{}, nil, true, false, "BTCUSDT", "1m")
	rt.SetPersistenceQueue(q)
	return rt
}

func gapHealBar(ot, step int64) data.Candle {
	return data.Candle{
		OpenTime: ot, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, CloseTime: ot + step - 1,
	}
}

func TestHealArchiveGaps_RealGapCallsREST(t *testing.T) {
	rt := gapHealRuntime(t)
	const step int64 = 60_000
	a := (int64(1_700_000_000_000) / step) * step
	z := a + 5*step
	if err := data.SaveKlines("BTCUSDT", "1m", []data.Candle{gapHealBar(a, step), gapHealBar(z, step)}); err != nil {
		t.Fatal(err)
	}
	var rest atomic.Int32
	rt.healClosedFetcher = func(string, string, int64, int64) ([]exchange.Candle, error) {
		rest.Add(1)
		return []exchange.Candle{{
			OpenTime: a + step, CloseTime: a + 2*step - 1,
			Open: 1, High: 1, Low: 1, Close: 1, Volume: 1,
		}}, nil
	}
	if err := rt.healSQLiteArchiveGaps(context.Background(), "BTCUSDT", "1m"); err != nil {
		t.Fatal(err)
	}
	if rest.Load() == 0 {
		t.Fatal("current physical gap must remain REST-eligible")
	}
}

func TestHealArchiveGaps_StalePhantomClearedWithoutREST(t *testing.T) {
	rt := gapHealRuntime(t)
	const step int64 = 60_000
	a := (int64(1_700_000_000_000) / step) * step
	mid := a + 2*step
	z := a + 5*step
	if err := data.SaveKlines("BTCUSDT", "1m", []data.Candle{
		gapHealBar(a, step), gapHealBar(a+step, step), gapHealBar(mid, step),
		gapHealBar(a+3*step, step), gapHealBar(a+4*step, step), gapHealBar(z, step),
	}); err != nil {
		t.Fatal(err)
	}
	if err := data.RecordArchiveGap("BTCUSDT", "1m", a, z); err != nil {
		t.Fatal(err)
	}
	rt.healClosedFetcher = func(string, string, int64, int64) ([]exchange.Candle, error) {
		t.Fatal("stale phantom must not REST")
		return nil, nil
	}
	if err := rt.healSQLiteArchiveGaps(context.Background(), "BTCUSDT", "1m"); err != nil {
		t.Fatal(err)
	}
	gaps, err := data.ListArchiveGaps("BTCUSDT", "1m", 8)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range gaps {
		if g.AfterOpenMs == a && g.BeforeOpenMs == z {
			t.Fatal("stale (A,Z) OPEN row must be cleared")
		}
	}
}

func TestHealArchiveGaps_ContiguousNeighborsClearedWithoutREST(t *testing.T) {
	rt := gapHealRuntime(t)
	const step int64 = 60_000
	a := (int64(1_700_000_000_000) / step) * step
	b := a + step
	if err := data.SaveKlines("BTCUSDT", "1m", []data.Candle{gapHealBar(a, step), gapHealBar(b, step)}); err != nil {
		t.Fatal(err)
	}
	if err := data.TestingForceOpenArchiveGap("BTCUSDT", "1m", a, b); err != nil {
		t.Fatal(err)
	}
	rt.healClosedFetcher = func(string, string, int64, int64) ([]exchange.Candle, error) {
		t.Fatal("contiguous neighbors must not REST")
		return nil, nil
	}
	if err := rt.healSQLiteArchiveGaps(context.Background(), "BTCUSDT", "1m"); err != nil {
		t.Fatal(err)
	}
	gaps, err := data.ListArchiveGaps("BTCUSDT", "1m", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 0 {
		t.Fatalf("contiguous lie should be cleared, got %#v", gaps)
	}
}

func TestHealArchiveGaps_ExhaustedUntouched(t *testing.T) {
	rt := gapHealRuntime(t)
	const step int64 = 60_000
	a := (int64(1_700_000_000_000) / step) * step
	z := a + 5*step
	if err := data.SaveKlines("BTCUSDT", "1m", []data.Candle{gapHealBar(a, step), gapHealBar(z, step)}); err != nil {
		t.Fatal(err)
	}
	if err := data.MarkArchiveGapExhausted("BTCUSDT", "1m", a, z, "empty_rest"); err != nil {
		t.Fatal(err)
	}
	rt.healClosedFetcher = func(string, string, int64, int64) ([]exchange.Candle, error) {
		t.Fatal("exhausted row must not enter REST heal")
		return nil, nil
	}
	if err := rt.healSQLiteArchiveGaps(context.Background(), "BTCUSDT", "1m"); err != nil {
		t.Fatal(err)
	}
	open, err := data.ListArchiveGaps("BTCUSDT", "1m", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 0 {
		t.Fatalf("exhausted should stay off heal queue, got %#v", open)
	}
	ok, err := data.ArchiveGapIsCurrentNeighbor("BTCUSDT", "1m", a, z)
	if err != nil || !ok {
		t.Fatalf("physical hole still exists under exhausted tombstone: ok=%v err=%v", ok, err)
	}
}

func TestHealArchiveGaps_NoSuccessorClearedWithoutREST(t *testing.T) {
	rt := gapHealRuntime(t)
	const step int64 = 60_000
	a := (int64(1_700_000_000_000) / step) * step
	z := a + 5*step
	if err := data.SaveKlines("BTCUSDT", "1m", []data.Candle{gapHealBar(a, step)}); err != nil {
		t.Fatal(err)
	}
	if err := data.TestingForceOpenArchiveGap("BTCUSDT", "1m", a, z); err != nil {
		t.Fatal(err)
	}
	rt.healClosedFetcher = func(string, string, int64, int64) ([]exchange.Candle, error) {
		t.Fatal("missing successor must not REST (not a fetch-to-tip policy)")
		return nil, nil
	}
	if err := rt.healSQLiteArchiveGaps(context.Background(), "BTCUSDT", "1m"); err != nil {
		t.Fatal(err)
	}
	gaps, err := data.ListArchiveGaps("BTCUSDT", "1m", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 0 {
		t.Fatalf("no-successor OPEN should be cleared, got %#v", gaps)
	}
}
