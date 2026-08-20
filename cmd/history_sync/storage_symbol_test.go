package main

import "testing"

func TestPersistStorageSymbol_SpotUsesSpotStorageKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, market, want string
	}{
		{"BTCUSDT", "spot", "BTCUSDT_SPOT"},
		{"btcusdt", "SPOT", "BTCUSDT_SPOT"},
		{"BTCUSDT_SPOT", "spot", "BTCUSDT_SPOT"},
		{"BTCUSDT.P", "spot", "BTCUSDT_SPOT"},
	}
	for _, tc := range cases {
		got := persistStorageSymbol(tc.market, pairSymbol(tc.in))
		if got != tc.want {
			t.Fatalf("persistStorageSymbol(%q,%q)=%q want %q", tc.market, tc.in, got, tc.want)
		}
	}
}

func TestPersistStorageSymbol_FuturesKeyUnchanged(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, market, want string
	}{
		{"BTCUSDT", "futures", "BTCUSDT"},
		{"btcusdt", "futures", "BTCUSDT"},
		{"BTCUSDT.P", "futures", "BTCUSDT"},
		{"BTCUSDT_PERP", "futures", "BTCUSDT"},
		{"BTCUSDT_SPOT", "futures", "BTCUSDT"},
	}
	for _, tc := range cases {
		got := persistStorageSymbol(tc.market, pairSymbol(tc.in))
		if got != tc.want {
			t.Fatalf("persistStorageSymbol(%q,%q)=%q want %q", tc.market, tc.in, got, tc.want)
		}
	}
}

func TestPairSymbol_VisionUsesPlainPair(t *testing.T) {
	t.Parallel()
	if got := pairSymbol("BTCUSDT_SPOT"); got != "BTCUSDT" {
		t.Fatalf("pairSymbol(BTCUSDT_SPOT)=%q", got)
	}
}
