package indicators

import (
	"math"
	"testing"
)

func TestATRSpec_Validation(t *testing.T) {
	t.Parallel()
	ok := CanonicalATRSpec()
	if err := ValidateATRSpec(ok); err != nil {
		t.Fatal(err)
	}
	if _, err := NewATRFromSpec(ATRSpec{Period: 0, Method: ATRMethodWilderRMA, Logic: ATRLogicWilderRMAFirstTRV1}); err == nil {
		t.Fatal("Period<=0 must refuse")
	}
	if _, err := NewATRFromSpec(ATRSpec{Period: 14, Method: "sma", Logic: ATRLogicWilderRMAFirstTRV1}); err == nil {
		t.Fatal("unknown method must refuse")
	}
	if _, err := NewATRFromSpec(ATRSpec{Period: 14, Method: ATRMethodWilderRMA, Logic: "atr:wilder-sma-seed-v2"}); err == nil {
		t.Fatal("unknown logic must refuse")
	}
	legacy := NewATR(0)
	if legacy.period != DefaultATRPeriod {
		t.Fatalf("legacy NewATR(0) still defaults, got %d", legacy.period)
	}
}

func TestATR_FirstBarAndReady(t *testing.T) {
	t.Parallel()
	a, err := NewATRFromSpec(CanonicalATRSpec())
	if err != nil {
		t.Fatal(err)
	}
	if a.Ready() {
		t.Fatal("Ready before first closed bar")
	}
	v, err := a.UpdateClosed(12, 10, 11)
	if err != nil {
		t.Fatal(err)
	}
	if math.Float64bits(v) != math.Float64bits(2) {
		t.Fatalf("ATR[0]=TR[0]=2, got %v", v)
	}
	if !a.Ready() {
		t.Fatal("Ready after first successful closed update")
	}
}

func TestATR_WilderRecurrencePeriod2(t *testing.T) {
	t.Parallel()
	spec := ATRSpec{Period: 2, Method: ATRMethodWilderRMA, Logic: ATRLogicWilderRMAFirstTRV1}
	a, err := NewATRFromSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	v0, err := a.UpdateClosed(12, 10, 11)
	if err != nil {
		t.Fatal(err)
	}
	if v0 != 2 {
		t.Fatalf("bar0 %v", v0)
	}
	// TR=max(15-11, |15-11|, |11-11|)=4; ATR=2+(4-2)/2=3
	v1, err := a.UpdateClosed(15, 11, 14)
	if err != nil {
		t.Fatal(err)
	}
	if math.Float64bits(v1) != math.Float64bits(3) {
		t.Fatalf("want 3 got %v", v1)
	}
}

func TestATR_GapTrueRangeDominatesHL(t *testing.T) {
	t.Parallel()
	spec := ATRSpec{Period: 2, Method: ATRMethodWilderRMA, Logic: ATRLogicWilderRMAFirstTRV1}
	a, _ := NewATRFromSpec(spec)
	_, _ = a.UpdateClosed(10, 10, 10)
	// H-L=1, |H-prevC|=|11-10|=1, |L-prevC|=|10-10|=0 → TR=1
	// gap down: H=8 L=7 C=7.5 prev=10 → HL=1, |8-10|=2, |7-10|=3 → TR=3
	v, err := a.UpdateClosed(8, 7, 7.5)
	if err != nil {
		t.Fatal(err)
	}
	want := 0 + (3-0)/2 // ATR0=0 (flat), ATR1=1.5
	_ = want
	// ATR[0]=0, ATR[1]=0+(3-0)/2=1.5
	if math.Float64bits(v) != math.Float64bits(1.5) {
		t.Fatalf("gap TR ATR got %v want 1.5", v)
	}
}

func TestATR_ZeroRangeLegal(t *testing.T) {
	t.Parallel()
	a, _ := NewATRFromSpec(CanonicalATRSpec())
	v, err := a.UpdateClosed(100, 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Ready() || v != 0 {
		t.Fatalf("zero ATR must be Ready value=0, got ready=%v v=%v", a.Ready(), v)
	}
}

func TestATR_RejectNonfiniteAndMalformed(t *testing.T) {
	t.Parallel()
	a, _ := NewATRFromSpec(CanonicalATRSpec())
	if _, err := a.UpdateClosed(math.NaN(), 1, 1); err == nil {
		t.Fatal("NaN")
	}
	if _, err := a.UpdateClosed(math.Inf(1), 1, 1); err == nil {
		t.Fatal("+Inf")
	}
	if _, err := a.UpdateClosed(1, math.Inf(-1), 1); err == nil {
		t.Fatal("-Inf")
	}
	if _, err := a.UpdateClosed(10, 12, 11); err == nil {
		t.Fatal("High<Low")
	}
	if a.Ready() {
		t.Fatal("rejects must not Ready")
	}
}

func TestATR_InvalidUpdateDoesNotMutate(t *testing.T) {
	t.Parallel()
	spec := ATRSpec{Period: 2, Method: ATRMethodWilderRMA, Logic: ATRLogicWilderRMAFirstTRV1}
	control, _ := NewATRFromSpec(spec)
	branch, _ := NewATRFromSpec(spec)
	bars := [][3]float64{{12, 10, 11}, {15, 11, 14}, {16, 12, 15}}
	_, _ = control.UpdateClosed(bars[0][0], bars[0][1], bars[0][2])
	_, _ = control.UpdateClosed(bars[1][0], bars[1][1], bars[1][2])
	want, err := control.UpdateClosed(bars[2][0], bars[2][1], bars[2][2])
	if err != nil {
		t.Fatal(err)
	}

	_, _ = branch.UpdateClosed(bars[0][0], bars[0][1], bars[0][2])
	_, _ = branch.UpdateClosed(bars[1][0], bars[1][1], bars[1][2])
	if _, err := branch.UpdateClosed(math.NaN(), 0, 0); err == nil {
		t.Fatal("expected reject")
	}
	if _, err := branch.UpdateClosed(5, 9, 6); err == nil {
		t.Fatal("expected High<Low reject")
	}
	got, err := branch.UpdateClosed(bars[2][0], bars[2][1], bars[2][2])
	if err != nil {
		t.Fatal(err)
	}
	if math.Float64bits(got) != math.Float64bits(want) {
		t.Fatalf("poisoned IIR: got %v want %v", got, want)
	}
}

func TestATRSeries_MatchesStreaming(t *testing.T) {
	t.Parallel()
	spec := ATRSpec{Period: 3, Method: ATRMethodWilderRMA, Logic: ATRLogicWilderRMAFirstTRV1}
	high := []float64{10, 12, 9, 15, 15, 15, 14, 20, 8}
	low := []float64{9, 10, 8, 11, 15, 15, 13, 16, 7}
	close := []float64{9.5, 11, 8.5, 14, 15, 15, 13.5, 18, 7.5}
	series, err := ATRSeries(spec, high, low, close)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := NewATRFromSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	for i := range close {
		v, err := stream.UpdateClosed(high[i], low[i], close[i])
		if err != nil {
			t.Fatal(err)
		}
		if math.Float64bits(v) != math.Float64bits(series[i]) {
			t.Fatalf("bar %d stream=%v series=%v", i, v, series[i])
		}
		if !stream.Ready() {
			t.Fatal("ready")
		}
	}
}

func TestATR_SaveRestoreParityABCD(t *testing.T) {
	t.Parallel()
	spec := ATRSpec{Period: 4, Method: ATRMethodWilderRMA, Logic: ATRLogicWilderRMAFirstTRV1}
	bars := [][3]float64{
		{10, 8, 9}, {12, 9, 11}, {11, 7, 8}, {14, 10, 13},
	}
	control, _ := NewATRFromSpec(spec)
	var want float64
	for _, b := range bars {
		v, err := control.UpdateClosed(b[0], b[1], b[2])
		if err != nil {
			t.Fatal(err)
		}
		want = v
	}

	branch, _ := NewATRFromSpec(spec)
	_, _ = branch.UpdateClosed(bars[0][0], bars[0][1], bars[0][2])
	_, _ = branch.UpdateClosed(bars[1][0], bars[1][1], bars[1][2])
	branch.SaveState()
	_, _ = branch.UpdateClosed(bars[2][0], bars[2][1], bars[2][2])
	branch.RestoreState()
	_, _ = branch.UpdateClosed(bars[2][0], bars[2][1], bars[2][2])
	got, err := branch.UpdateClosed(bars[3][0], bars[3][1], bars[3][2])
	if err != nil {
		t.Fatal(err)
	}
	if math.Float64bits(got) != math.Float64bits(want) {
		t.Fatalf("save/restore got %v want %v", got, want)
	}
}

func TestATRValues_ShortInputUnchanged(t *testing.T) {
	t.Parallel()
	high := []float64{12, 13, 14}
	low := []float64{10, 11, 12}
	close := []float64{11, 12, 13}
	if ATRValues(high, low, close, 14) != nil {
		t.Fatal("legacy ATRValues must stay nil when len<=period")
	}
	longH := make([]float64, 16)
	longL := make([]float64, 16)
	longC := make([]float64, 16)
	for i := range longH {
		longH[i] = 20 + float64(i)
		longL[i] = 18 + float64(i)
		longC[i] = 19 + float64(i)
	}
	got := ATRValues(longH, longL, longC, 14)
	if len(got) != 16 {
		t.Fatalf("len %d", len(got))
	}
}
