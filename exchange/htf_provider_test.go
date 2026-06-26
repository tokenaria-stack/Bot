package exchange

import (
	"path/filepath"
	"testing"
	"time"

	"trading_bot/data"
)

func TestHTFProvider_GetKlines_CachesAndSlices(t *testing.T) {
	data.ResetDBForTest(filepath.Join(t.TempDir(), "htf.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}

	symbol := "BTCUSDT"
	interval := "1d"
	step := int64(24 * 60 * 60 * 1000)
	start := BinanceFuturesGenesisMs

	rows := []data.Candle{
		{OpenTime: start, CloseTime: start + step - 1, Open: 9000, High: 9100, Low: 8900, Close: 9050, Volume: 1},
		{OpenTime: start + step, CloseTime: start + 2*step - 1, Open: 9050, High: 9150, Low: 9000, Close: 9100, Volume: 2},
		{OpenTime: start + 2*step, CloseTime: start + 3*step - 1, Open: 9100, High: 9200, Low: 9050, Close: 9150, Volume: 3},
	}
	if err := data.SaveKlines(symbol, interval, rows); err != nil {
		t.Fatal(err)
	}

	p := NewHTFProvider()
	first, err := p.GetKlines(symbol, interval, start)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 3 {
		t.Fatalf("len(first) = %d, want 3", len(first))
	}

	laterStart := start + step
	second, err := p.GetKlines(symbol, interval, laterStart)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 {
		t.Fatalf("len(second) = %d, want 2 (cached slice)", len(second))
	}
	if second[0].OpenTime != laterStart {
		t.Fatalf("second[0].OpenTime = %d, want %d", second[0].OpenTime, laterStart)
	}

	// Mutating returned slice must not corrupt cache.
	second[0].Close = -1
	third, err := p.GetKlines(symbol, interval, start)
	if err != nil {
		t.Fatal(err)
	}
	if third[0].Close == -1 {
		t.Fatal("cache was mutated by caller")
	}
}

func TestHTFProvider_GetKlines_ReloadsEarlierStart(t *testing.T) {
	data.ResetDBForTest(filepath.Join(t.TempDir(), "htf-reload.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}

	symbol := "BTCUSDT"
	interval := "1h"
	step := int64(60 * 60 * 1000)
	base := BinanceFuturesGenesisMs

	rows := []data.Candle{
		{OpenTime: base, CloseTime: base + step - 1, Close: 1},
		{OpenTime: base + step, CloseTime: base + 2*step - 1, Close: 2},
		{OpenTime: base + 2*step, CloseTime: base + 3*step - 1, Close: 3},
	}
	if err := data.SaveKlines(symbol, interval, rows); err != nil {
		t.Fatal(err)
	}

	p := NewHTFProvider()
	if _, err := p.GetKlines(symbol, interval, base+step); err != nil {
		t.Fatal(err)
	}
	got, err := p.GetKlines(symbol, interval, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3 after earlier-start reload", len(got))
	}
}

func TestHTFProvider_GetKlines_NilProvider(t *testing.T) {
	var p *HTFProvider
	if _, err := p.GetKlines("BTCUSDT", "1h", 0); err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func seedHTFProviderDB(t *testing.T, dbName string) (symbol, interval string, start int64) {
	t.Helper()
	data.ResetDBForTest(filepath.Join(t.TempDir(), dbName))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}
	symbol = "BTCUSDT"
	interval = "1d"
	step := int64(24 * 60 * 60 * 1000)
	start = BinanceFuturesGenesisMs
	rows := []data.Candle{
		{OpenTime: start, CloseTime: start + step - 1, Close: 1},
		{OpenTime: start + step, CloseTime: start + 2*step - 1, Close: 2},
	}
	if err := data.SaveKlines(symbol, interval, rows); err != nil {
		t.Fatal(err)
	}
	return symbol, interval, start
}

func TestHTFProvider_PinKlines_NoOpWithoutCache(t *testing.T) {
	p := NewHTFProvider()
	p.PinKlines("BTCUSDT", "1h")
	if p.ClearCache(true) != 0 {
		t.Fatal("expected empty cache after PinKlines on missing entry")
	}
}

func TestHTFProvider_ClearCache_SoftKeepsPinned(t *testing.T) {
	symbol, interval, start := seedHTFProviderDB(t, "htf-pin.db")
	p := NewHTFProvider()
	if _, err := p.GetKlines(symbol, interval, start); err != nil {
		t.Fatal(err)
	}
	p.PinKlines(symbol, interval)

	if _, err := p.GetKlines(symbol, "1h", BinanceFuturesGenesisMs); err != nil {
		t.Fatal(err)
	}

	if removed := p.ClearCache(false); removed != 1 {
		t.Fatalf("soft clear removed %d, want 1 unpinned entry", removed)
	}
	if _, err := p.GetKlines(symbol, interval, start); err != nil {
		t.Fatal(err)
	}
	if removed := p.ClearCache(false); removed != 0 {
		t.Fatalf("pinned entry should survive soft clear, removed %d", removed)
	}
	if removed := p.ClearCache(true); removed != 1 {
		t.Fatalf("force clear removed %d, want 1 pinned entry", removed)
	}
}

func TestHTFProvider_CleanupIdle_EvictsOnlyUnpinnedStale(t *testing.T) {
	symbol, interval, start := seedHTFProviderDB(t, "htf-idle.db")
	p := NewHTFProvider()
	if _, err := p.GetKlines(symbol, interval, start); err != nil {
		t.Fatal(err)
	}
	p.PinKlines(symbol, interval)

	if _, err := p.GetKlines(symbol, "1h", start); err != nil {
		t.Fatal(err)
	}

	key1h := htfCacheKey(symbol, "1h")
	raw, ok := p.cache.Load(key1h)
	if !ok {
		t.Fatal("expected 1h cache entry")
	}
	entry := raw.(htfCacheEntry)
	entry.LastUsed = time.Now().Add(-2 * time.Hour)
	p.cache.Store(key1h, entry)

	if evicted := p.CleanupIdle(time.Hour); evicted != 1 {
		t.Fatalf("CleanupIdle evicted %d, want 1 stale unpinned entry", evicted)
	}
	if _, ok := p.cache.Load(htfCacheKey(symbol, interval)); !ok {
		t.Fatal("pinned entry should survive idle cleanup")
	}
}

func TestHTFProvider_GetKlines_UpdatesLastUsedOnHit(t *testing.T) {
	symbol, interval, start := seedHTFProviderDB(t, "htf-touch.db")
	p := NewHTFProvider()
	if _, err := p.GetKlines(symbol, interval, start); err != nil {
		t.Fatal(err)
	}

	key := htfCacheKey(symbol, interval)
	raw, ok := p.cache.Load(key)
	if !ok {
		t.Fatal("expected cache entry")
	}
	firstUsed := raw.(htfCacheEntry).LastUsed

	time.Sleep(2 * time.Millisecond)
	if _, err := p.GetKlines(symbol, interval, start); err != nil {
		t.Fatal(err)
	}
	raw, ok = p.cache.Load(key)
	if !ok {
		t.Fatal("expected cache entry after second GetKlines")
	}
	secondUsed := raw.(htfCacheEntry).LastUsed
	if !secondUsed.After(firstUsed) {
		t.Fatalf("LastUsed not updated: first=%v second=%v", firstUsed, secondUsed)
	}
}

func TestParseIntervalToSeconds(t *testing.T) {
	t.Parallel()
	if got := ParseIntervalToSeconds("4h"); got != 14400 {
		t.Fatalf("4h = %d, want 14400", got)
	}
	if got := ParseIntervalToSeconds("1d"); got != 86400 {
		t.Fatalf("1d = %d, want 86400", got)
	}
}

func TestHTFProvider_GetCandlesStrictlyBefore(t *testing.T) {
	step := int64(4 * 60 * 60 * 1000)
	base := int64(1_700_000_000_000)
	klines := []Kline{
		{OpenTime: base, CloseTime: base + step - 1, Close: 1},
		{OpenTime: base + step, CloseTime: base + 2*step - 1, Close: 2},
		{OpenTime: base + 2*step, CloseTime: base + 3*step - 1, Close: 3},
	}
	p := NewHTFProvider()
	key := htfCacheKey("BTCUSDT", "4h")
	p.cache.Store(key, htfCacheEntry{startMs: base, klines: klines})

	// After second candle close, third is still open.
	maxSec := (base + 2*step - 1) / 1000
	got := p.GetCandlesStrictlyBefore("BTCUSDT", "4h", maxSec)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 strictly closed 4h bars", len(got))
	}
	if got[1].Close != 2 {
		t.Fatalf("last closed bar close = %v, want 2", got[1].Close)
	}

	future := p.GetCandlesStrictlyBefore("BTCUSDT", "4h", base/1000-1)
	if len(future) != 0 {
		t.Fatalf("expected no bars before series start, got %d", len(future))
	}
}
