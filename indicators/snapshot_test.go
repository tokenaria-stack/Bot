package indicators_test

import (
	"testing"

	"trading_bot/indicators"
)

func TestMACD_SaveRestore_IntraBarRollback(t *testing.T) {
	t.Parallel()

	m := indicators.NewMACD(12, 26, 9)
	for i := 0; i < 40; i++ {
		m.Update(100 + float64(i)*0.5)
	}
	m.SaveState()

	wantLine := m.Value()
	wantSignal := m.Signal()
	wantHist := m.Histogram()

	m.Update(1e6) // open-bar garbage tick
	if m.Value() == wantLine && m.Signal() == wantSignal && m.Histogram() == wantHist {
		t.Fatal("expected garbage Update to mutate MACD cached outputs")
	}

	m.RestoreState()
	const eps = 1e-12
	if abs(m.Value()-wantLine) > eps || abs(m.Signal()-wantSignal) > eps || abs(m.Histogram()-wantHist) > eps {
		t.Fatalf("RestoreState did not restore cached outputs: got line=%v signal=%v hist=%v want %v %v %v",
			m.Value(), m.Signal(), m.Histogram(), wantLine, wantSignal, wantHist)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestRSI_SaveRestore_IntraBarRollback(t *testing.T) {
	t.Parallel()

	r := indicators.NewRSI(14)
	// Warm up with distinct closes.
	closes := []float64{100, 101, 99, 102, 98, 103, 97, 104, 96, 105, 95, 106, 94, 107, 93}
	for _, c := range closes {
		r.Update(c)
	}
	r.SaveState()

	openBarFirst := 108.0
	openBarFinal := 109.5
	_ = r.Update(openBarFirst)
	gotPoisoned := r.Update(openBarFinal)

	r.RestoreState()
	gotSingle := r.Update(openBarFinal)

	if gotPoisoned == gotSingle {
		t.Fatalf("expected rollback to change RSI: poisoned=%v single=%v", gotPoisoned, gotSingle)
	}
	const eps = 1e-9
	diff := gotSingle - gotPoisoned
	if diff < 0 {
		diff = -diff
	}
	if diff < eps {
		t.Fatalf("single eval %v should differ from poisoned %v", gotSingle, gotPoisoned)
	}
}

func TestATR_SaveRestore_IntraBarRollback(t *testing.T) {
	t.Parallel()

	atr := indicators.NewATR(14)
	candles := [][3]float64{
		{101, 99, 100}, {102, 98, 101}, {103, 97, 102}, {104, 96, 103},
		{105, 95, 104}, {106, 94, 105}, {107, 93, 106}, {108, 92, 107},
		{109, 91, 108}, {110, 90, 109}, {111, 89, 110}, {112, 88, 111},
		{113, 87, 112}, {114, 86, 113}, {115, 85, 114},
	}
	for _, c := range candles {
		atr.UpdateCandle(c[0], c[1], c[2])
	}
	atr.SaveState()

	_ = atr.UpdateCandle(118, 82, 116)
	gotPoisoned := atr.UpdateCandle(120, 80, 118)

	atr.RestoreState()
	gotSingle := atr.UpdateCandle(120, 80, 118)

	if gotPoisoned == gotSingle {
		t.Fatalf("expected rollback to change ATR: poisoned=%v single=%v", gotPoisoned, gotSingle)
	}
}

func TestAO_SaveRestore_IntraBarRollback(t *testing.T) {
	t.Parallel()

	ao := indicators.NewAO(5, 34)
	for i := 0; i < 40; i++ {
		hl2 := 100 + float64(i)*0.1
		ao.Update(hl2)
	}
	ao.SaveState()

	gotPoisoned := ao.Update(104.2)
	ao.RestoreState()
	gotSingle := ao.Update(104.5)

	if gotPoisoned == gotSingle {
		t.Fatalf("expected rollback to change AO: poisoned=%v single=%v", gotPoisoned, gotSingle)
	}
}
