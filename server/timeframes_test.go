package server

import (
	"strings"
	"testing"

	"trading_bot/exchange"
)

func TestResolveBacktestInterval(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in      string
		binance string
		wantErr bool
	}{
		{"1m", "1m", false},
		{"1D", "1d", false},
		{"D", "1d", false},
		{"1W", "1w", false},
		{"W", "1w", false},
		{"1M", "1M", false},
		{"2h", "2h", false},
		{"6h", "6h", false},
		{"3d", "3d", false},
		{"2m", "", true},
		{"10m", "", true},
		{"3h", "", true},
		{"3M", "", true},
		{"3M", "", true},
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
		{"2h", "2h", TFBinanceREST},
		{"6h", "6h", TFBinanceREST},
		{"8h", "8h", TFBinanceREST},
		{"12h", "12h", TFBinanceREST},
		{"3d", "3d", TFBinanceREST},
		{"2m", "2m", TFRAMOnly},
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
			if tc.kind == TFBinanceREST && spec.BinanceInterval != tc.id {
				t.Fatalf("binance=%q want %q", spec.BinanceInterval, tc.id)
			}
			if tc.id == "2m" && spec.BinanceInterval != "" {
				t.Fatal("2m must not resolve as Binance-native")
			}
		})
	}
}

func TestMenuTimeframesLiveChart(t *testing.T) {
	t.Parallel()
	menu := MenuTimeframes()
	if _, ok := menu["TICKS"]; ok {
		t.Fatal("TICKS must stay hidden until tick builder")
	}
	if _, ok := menu["SECONDS"]; ok {
		t.Fatal("SECONDS must stay hidden until seconds builder")
	}
	var ids []string
	for _, group := range []string{"MINUTES", "HOURS", "DAYS"} {
		for _, spec := range menu[group] {
			ids = append(ids, spec.ID)
			if !exchange.IsLiveChartTF(spec.ID) {
				t.Fatalf("menu %s %s not live-chart", group, spec.ID)
			}
		}
	}
	joined := strings.Join(ids, ",")
	for _, need := range []string{"2m", "10m", "45m", "3h", "1M", "2h"} {
		if !strings.Contains(joined, need) {
			t.Fatalf("menu missing %s: %v", need, ids)
		}
	}
	if strings.Contains(joined, "1s") || strings.Contains(joined, "1tick") {
		t.Fatal("seconds/ticks must stay hidden")
	}
}
