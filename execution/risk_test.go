package execution

import "testing"

func TestCalculateTargetQuantity_basic(t *testing.T) {
	t.Parallel()

	// 10k balance, 1% risk, 1% SL distance → ~10k notional → qty 0.1 @ 100k
	qty, lev := CalculateTargetQuantity(10000, 1.0, 100000, 99000, 1.0, 10)
	if qty <= 0 {
		t.Fatalf("qty = %v, want > 0", qty)
	}
	if lev < 1 {
		t.Fatalf("leverage = %v, want >= 1", lev)
	}
}

func TestCalculateTargetQuantity_lotMod(t *testing.T) {
	t.Parallel()

	base, _ := CalculateTargetQuantity(10000, 1.0, 100000, 99000, 1.0, 10)
	half, _ := CalculateTargetQuantity(10000, 1.0, 100000, 99000, 0.5, 10)
	if half >= base {
		t.Fatalf("lotMod 0.5 qty %v should be less than base %v", half, base)
	}
}

func TestCalculateTargetQuantity_leverageCap(t *testing.T) {
	t.Parallel()

	_, lev := CalculateTargetQuantity(1000, 5.0, 100, 95, 1.0, 2)
	if lev > 2 {
		t.Fatalf("leverage = %v, want capped at 2", lev)
	}
}

func TestCalculateTargetQuantity_invalid(t *testing.T) {
	t.Parallel()

	if qty, _ := CalculateTargetQuantity(0, 1, 100, 99, 1, 10); qty != 0 {
		t.Fatalf("zero balance should return 0 qty, got %v", qty)
	}
	if qty, _ := CalculateTargetQuantity(1000, 1, 100, 100, 1, 10); qty != 0 {
		t.Fatalf("equal entry/stop should return 0 qty, got %v", qty)
	}
}
