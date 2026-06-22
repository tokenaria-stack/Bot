package indicators

import (
	"math"
	"testing"
)

func TestCalculatePriceZones_Uptrend(t *testing.T) {
	engine := NewFibonacciEngine()
	start, end := 100.0, 200.0
	atr := 10.0
	padding := atr * priceATRPaddingMult

	zones := engine.CalculatePriceZones(start, end, 138.2, atr)
	if len(zones) != 7 {
		t.Fatalf("expected 7 zones, got %d", len(zones))
	}

	// 0.618 retracement: 200 - 100*0.618 = 138.2
	var found618 bool
	for _, z := range zones {
		if z.Type == Retracement && z.Ratio == 0.618 {
			found618 = true
			want := 138.2
			if math.Abs(z.TargetValue-want) > 1e-9 {
				t.Fatalf("0.618 target = %v, want %v", z.TargetValue, want)
			}
			if math.Abs(z.UpperBound-(want+padding)) > 1e-9 || math.Abs(z.LowerBound-(want-padding)) > 1e-9 {
				t.Fatalf("unexpected bounds for 0.618: [%v, %v]", z.LowerBound, z.UpperBound)
			}
			if !z.IsActive {
				t.Fatal("expected 0.618 zone to be active at price 138.2")
			}
		}
	}
	if !found618 {
		t.Fatal("missing 0.618 retracement zone")
	}

	// 1.618 extension: 200 + 100*(1.618-1) = 261.8
	var foundExt bool
	for _, z := range zones {
		if z.Type == Extension && z.Ratio == 1.618 {
			foundExt = true
			want := 261.8
			if math.Abs(z.TargetValue-want) > 1e-9 {
				t.Fatalf("1.618 extension target = %v, want %v", z.TargetValue, want)
			}
		}
	}
	if !foundExt {
		t.Fatal("missing 1.618 extension zone")
	}
}

func TestCalculatePriceZones_Downtrend(t *testing.T) {
	engine := NewFibonacciEngine()
	start, end := 200.0, 100.0
	atr := 4.0

	zones := engine.CalculatePriceZones(start, end, 150.0, atr)
	activeCount := 0
	for _, z := range zones {
		if z.IsActive {
			activeCount++
		}
	}
	if activeCount == 0 {
		t.Fatal("expected at least one active zone for price inside the wave")
	}
}

func TestCalculateTimeZones(t *testing.T) {
	engine := NewFibonacciEngine()
	start, end, current := 10, 20, 32

	zones := engine.CalculateTimeZones(start, end, current)
	if len(zones) != 4 {
		t.Fatalf("expected 4 time zones, got %d", len(zones))
	}

	// diff=10, ratio=1.618 -> target=20+16=36, active window [34,38], current=32 inactive
	var ratio1618 FibZone
	for _, z := range zones {
		if z.Ratio == 1.618 {
			ratio1618 = z
		}
	}
	if ratio1618.TargetValue != 36 {
		t.Fatalf("1.618 time target = %v, want 36", ratio1618.TargetValue)
	}
	if ratio1618.IsActive {
		t.Fatal("expected 1.618 time zone inactive at index 32")
	}

	// ratio=1.0 -> target=30, window [28,32], current=32 active
	var ratio10 FibZone
	for _, z := range zones {
		if z.Ratio == 1.0 {
			ratio10 = z
		}
	}
	if !ratio10.IsActive {
		t.Fatal("expected 1.0 time zone active at index 32")
	}
}

func TestFindConfluence(t *testing.T) {
	zonesA := []FibZone{
		{Type: Retracement, Ratio: 0.618, TargetValue: 150.0, LowerBound: 145, UpperBound: 155, IsActive: true},
		{Type: Extension, Ratio: 1.618, TargetValue: 300.0, LowerBound: 295, UpperBound: 305},
	}
	zonesB := []FibZone{
		{Type: Retracement, Ratio: 0.618, TargetValue: 152.0, LowerBound: 147, UpperBound: 157, IsActive: false},
		{Type: Retracement, Ratio: 0.5, TargetValue: 200.0, LowerBound: 195, UpperBound: 205},
	}

	clusters := FindConfluence(zonesA, zonesB, 5.0)
	if len(clusters) != 1 {
		t.Fatalf("expected 1 confluence cluster, got %d", len(clusters))
	}
	if math.Abs(clusters[0].TargetValue-151.0) > 1e-9 {
		t.Fatalf("cluster target = %v, want 151", clusters[0].TargetValue)
	}
	if !clusters[0].IsActive {
		t.Fatal("expected merged cluster to be active when either side is active")
	}
}
