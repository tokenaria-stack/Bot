package exchange

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLimitsCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	orig := LimitsCachePath
	LimitsCachePath = filepath.Join(dir, "exchange_limits_cache.json")
	t.Cleanup(func() { LimitsCachePath = orig })

	limits := map[string]SymbolLimits{
		"BTCUSDT": {TickSize: 0.1, StepSize: 0.001},
	}
	if err := writeLimitsCache(limits, false); err != nil {
		t.Fatal(err)
	}

	got, age, err := readLimitsCache(false)
	if err != nil {
		t.Fatal(err)
	}
	if age < 0 || age > time.Minute {
		t.Fatalf("unexpected cache age %s", age)
	}
	if got["BTCUSDT"].TickSize != 0.1 {
		t.Fatalf("unexpected limits: %+v", got["BTCUSDT"])
	}
}

func TestReadLimitsCacheMissing(t *testing.T) {
	orig := LimitsCachePath
	LimitsCachePath = filepath.Join(t.TempDir(), "missing.json")
	t.Cleanup(func() { LimitsCachePath = orig })

	if _, _, err := readLimitsCache(false); err == nil {
		t.Fatal("expected error for missing cache")
	}
}

func TestWriteLimitsCacheCreatesDir(t *testing.T) {
	dir := t.TempDir()
	orig := LimitsCachePath
	LimitsCachePath = filepath.Join(dir, "nested", "exchange_limits_cache.json")
	t.Cleanup(func() { LimitsCachePath = orig })

	if err := writeLimitsCache(map[string]SymbolLimits{
		"ETHUSDT": {TickSize: 0.01, StepSize: 0.001},
	}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(LimitsCachePath); err != nil {
		t.Fatal(err)
	}
}
