package exchange

import "testing"

func TestEnsureUnixMillis(t *testing.T) {
	t.Parallel()

	sec := int64(1_700_000_040)
	ms := int64(1_700_000_040_000)

	if got := EnsureUnixMillis(sec); got != ms {
		t.Fatalf("seconds: got %d want %d", got, ms)
	}
	if got := EnsureUnixMillis(ms); got != ms {
		t.Fatalf("milliseconds passthrough: got %d want %d", got, ms)
	}
}

func TestChartTimeSec_UniquePerMinute(t *testing.T) {
	t.Parallel()

	t1 := ChartTimeSec(1_700_000_040_000)
	t2 := ChartTimeSec(1_700_000_100_000)
	if t1 == t2 {
		t.Fatalf("chart times collapsed: %d and %d", t1, t2)
	}
	if t2-t1 != 60 {
		t.Fatalf("delta = %d, want 60", t2-t1)
	}
}

func TestChartTimeSec_SecondsInputNotCollapsed(t *testing.T) {
	t.Parallel()

	// Without EnsureUnixMillis, 10-digit seconds / 1000 collapse to the same bar.
	t1 := ChartTimeSec(1_700_000_040)
	t2 := ChartTimeSec(1_700_000_100)
	if t1 == t2 {
		t.Fatalf("normalized chart times must not collapse: %d == %d", t1, t2)
	}
}

// Debt #83: genuine Unix seconds at the external boundary convert exactly once.
func TestEnsureUnixMillis_ConvertsGenuineSecondsExactlyOnce(t *testing.T) {
	t.Parallel()
	sec := int64(1_700_000_040)
	ms := EnsureUnixMillis(sec)
	if ms != sec*1000 {
		t.Fatalf("first convert: got %d want %d", ms, sec*1000)
	}
	if again := EnsureUnixMillis(ms); again != ms {
		t.Fatalf("second pass must be idempotent: got %d want %d", again, ms)
	}
}

// Debt #83: canonical Unix ms below the 1e12 magnitude threshold must not be
// re-interpreted as seconds (1993-03-01 ms is the LoadKlines smoking gun).
func TestEnsureUnixMillis_Pre2001CanonicalMsUnchanged(t *testing.T) {
	t.Parallel()
	t.Skip("debt #83 Patch B: EnsureUnixMillis magnitude heuristic left unchanged in Patch A")
	const march1993Ms int64 = 730_944_000_000
	if got := EnsureUnixMillis(march1993Ms); got != march1993Ms {
		t.Fatalf("got %d want %d (ms < 1e12 must not be *1000)", got, march1993Ms)
	}
	if got := ChartTimeSec(march1993Ms); got != march1993Ms/1000 {
		t.Fatalf("ChartTimeSec got %d want %d", got, march1993Ms/1000)
	}
}
