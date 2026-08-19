package exchange

import "testing"

func TestChartTimeSec_UnixMillisecondsToSeconds(t *testing.T) {
	t.Parallel()
	const ms int64 = 1_700_000_040_000
	if got := ChartTimeSec(ms); got != 1_700_000_040 {
		t.Fatalf("ChartTimeSec(%d)=%d want %d", ms, got, int64(1_700_000_040))
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

// Debt #83 C3: pre-2001 Unix ms must divide to sec — never ×1000 first (year ~25132 landmine).
func TestChartTimeSec_Pre2001Millisecond(t *testing.T) {
	t.Parallel()
	const march1993Ms int64 = 730_944_000_000
	want := march1993Ms / 1000
	got := ChartTimeSec(march1993Ms)
	if got != want {
		t.Fatalf("ChartTimeSec(%d)=%d want %d (must not coerce via 1e12 heuristic)", march1993Ms, got, want)
	}
}

// Legacy: ChartTimeSec no longer accepts seconds as a compatibility input.
// Passing seconds yields wrong chart times (sec/1000); that is a caller bug, not a feature.
func TestChartTimeSec_DocumentedMsOnlyContract(t *testing.T) {
	t.Parallel()
	// Mistaken seconds input is NOT converted to ms — contract is ms-only.
	sec := int64(1_700_000_040)
	if got := ChartTimeSec(sec); got != sec/1000 {
		t.Fatalf("ms-only contract: ChartTimeSec(%d)=%d want floor division %d", sec, got, sec/1000)
	}
}
