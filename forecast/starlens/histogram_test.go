package starlens

import (
	"math"
	"testing"
)

func TestDisplayHistogramHalfOpenNoOverlap(t *testing.T) {
	values := []float64{20, 25, 25, 29.9, 30, 34}
	bins, err := DisplayHistogram(values)
	if err != nil {
		t.Fatal(err)
	}
	if len(bins) != len(values) {
		t.Fatalf("tiny n should use n bins, got %d", len(bins))
	}
	sum := 0
	for i, b := range bins {
		sum += b.Count
		if i == 0 && !b.OpenLow {
			t.Fatal("first bin must be open below")
		}
		if i == len(bins)-1 && !b.OpenHigh {
			t.Fatal("last bin must be open above")
		}
		if i > 0 && i < len(bins)-1 && (b.OpenLow || b.OpenHigh) {
			t.Fatalf("interior %+v", b)
		}
		if i > 0 && bins[i-1].To != b.From {
			t.Fatalf("gap or overlap %v %v", bins[i-1], b)
		}
	}
	if sum != len(values) {
		t.Fatalf("sum %d want %d", sum, len(values))
	}
}

func TestBoundaryBelongsToExactlyOneBin(t *testing.T) {
	cuts := []float64{25, 30}
	counts, err := CountHalfOpen([]float64{20, 25, 30, 35}, cuts)
	if err != nil {
		t.Fatal(err)
	}
	if counts[0] != 1 || counts[1] != 1 || counts[2] != 2 {
		t.Fatalf("counts %v", counts)
	}
	right, err := CountBins([]float64{0, 5, 6}, []float64{0, 5})
	if err != nil {
		t.Fatal(err)
	}
	if right[0] != 1 || right[1] != 1 || right[2] != 1 {
		t.Fatalf("CountBins still right-closed %v", right)
	}
}

func TestDisplayBinCountIsPresentation(t *testing.T) {
	if DisplayBinCount(1) != 1 || DisplayBinCount(40) != 40 || DisplayBinCount(8783) != 64 {
		t.Fatalf("counts %d %d %d", DisplayBinCount(1), DisplayBinCount(40), DisplayBinCount(8783))
	}
	many := make([]float64, 200)
	for i := range many {
		many[i] = float64(i)
	}
	bins, err := DisplayHistogram(many)
	if err != nil {
		t.Fatal(err)
	}
	if len(bins) != 64 {
		t.Fatalf("display %d", len(bins))
	}
	sum := 0
	for _, b := range bins {
		sum += b.Count
	}
	if sum != 200 {
		t.Fatalf("sum %d", sum)
	}
	if math.IsNaN(bins[0].From) {
		t.Fatal("nan")
	}
}

func TestZeroIsObserved(t *testing.T) {
	bins, err := DisplayHistogram([]float64{0, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	sum := 0
	for _, b := range bins {
		sum += b.Count
	}
	if sum != 3 {
		t.Fatalf("sum %d", sum)
	}
}
