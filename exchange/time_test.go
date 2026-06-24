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
