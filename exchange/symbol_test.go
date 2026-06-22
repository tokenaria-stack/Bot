package exchange

import "testing"

func TestNormalizeFuturesSymbol(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"BTCUSDT", "BTCUSDT"},
		{"btcusdt", "BTCUSDT"},
		{"BTCUSDT.P", "BTCUSDT"},
		{"BTCUSDT_PERP", "BTCUSDT"},
		{" ETHUSDT.P ", "ETHUSDT"},
	}
	for _, tt := range tests {
		if got := NormalizeFuturesSymbol(tt.in); got != tt.want {
			t.Errorf("NormalizeFuturesSymbol(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
