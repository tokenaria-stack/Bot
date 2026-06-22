package server

import "testing"

func TestResolveBacktestInterval(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in       string
		binance  string
		wantErr  bool
	}{
		{"1m", "1m", false},
		{"1D", "1d", false},
		{"D", "1d", false},
		{"1W", "1w", false},
		{"W", "1w", false},
		{"1M", "1M", false},
		{"3M", "1M", false},
		{"35m", "", true},
	}
	for _, tc := range cases {
		spec, err := ResolveBacktestInterval(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("ResolveBacktestInterval(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ResolveBacktestInterval(%q): %v", tc.in, err)
		}
		if spec.BinanceInterval != tc.binance {
			t.Fatalf("ResolveBacktestInterval(%q) binance=%q want %q", tc.in, spec.BinanceInterval, tc.binance)
		}
	}
}

func TestResolveTimeframe(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		id   string
		kind TimeframeKind
	}{
		{"1m", "1m", TFBinanceREST},
		{"1M", "1M", TFBinanceREST},
		{"1 tick", "1tick", TFRAMOnly},
		{"100 ticks", "100ticks", TFRAMOnly},
		{"5 seconds", "5s", TFRAMOnly},
		{"3m", "3m", TFBinanceREST},
		{"35m", "35m", TFRAMOnly},
		{"1h", "1h", TFBinanceREST},
		{"3M", "3M", TFRAMOnly},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			spec, err := ResolveTimeframe(tc.in)
			if err != nil {
				t.Fatalf("ResolveTimeframe(%q): %v", tc.in, err)
			}
			if spec.ID != tc.id {
				t.Fatalf("id = %q, want %q", spec.ID, tc.id)
			}
			if spec.Kind != tc.kind {
				t.Fatalf("kind = %v, want %v", spec.Kind, tc.kind)
			}
		})
	}
}
