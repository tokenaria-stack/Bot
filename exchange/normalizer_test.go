package exchange

import "testing"

func TestNormalizer_FormatQuantity_truncatesDown(t *testing.T) {
	t.Parallel()

	n := NewNormalizer()
	n.mu.Lock()
	n.limits["BTCUSDT"] = SymbolLimits{TickSize: 0.01, StepSize: 0.00001}
	n.mu.Unlock()

	got, err := n.FormatQuantity("BTCUSDT", 0.010009)
	if err != nil {
		t.Fatalf("FormatQuantity: %v", err)
	}
	if got != "0.01000" {
		t.Fatalf("FormatQuantity = %q, want %q", got, "0.01000")
	}
}

func TestNormalizer_FormatPrice(t *testing.T) {
	t.Parallel()

	n := NewNormalizer()
	n.mu.Lock()
	n.limits["BTCUSDT"] = SymbolLimits{TickSize: 0.01, StepSize: 0.00001}
	n.mu.Unlock()

	got, err := n.FormatPrice("BTCUSDT", 65432.129)
	if err != nil {
		t.Fatalf("FormatPrice: %v", err)
	}
	if got != "65432.12" {
		t.Fatalf("FormatPrice = %q, want %q", got, "65432.12")
	}
}

func TestTruncateToStep(t *testing.T) {
	t.Parallel()

	got := truncateToStep(1.234567, 0.001)
	if got != 1.234 {
		t.Fatalf("truncateToStep = %v, want 1.234", got)
	}
}
