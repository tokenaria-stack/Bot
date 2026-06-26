package strategy

import (
	"math"
	"testing"
)

func TestApplyRSXSettings_Clamp(t *testing.T) {
	t.Parallel()
	ResetRSXSettings()
	t.Cleanup(ResetRSXSettings)

	got := ApplyRSXSettings(RSXSettings{
		DivLookback:  500,
		SignalLength: 99,
		Length:       200,
	})
	if got.DivLookback != MaxRSXDivLookback {
		t.Fatalf("DivLookback = %d, want %d", got.DivLookback, MaxRSXDivLookback)
	}
	if got.SignalLength != MaxRSXSignalLength {
		t.Fatalf("SignalLength = %d, want %d", got.SignalLength, MaxRSXSignalLength)
	}
	if got.Length != MaxRSXLength {
		t.Fatalf("Length = %d, want %d", got.Length, MaxRSXLength)
	}

	got = ApplyRSXSettings(RSXSettings{
		Length:       7,
		DivLookback:  45,
		SignalLength: 12,
		Source:       "hlc3",
		PivotRadius:  3,
		DivMethod:    "fractal",
	})
	if got.Length != 7 || got.DivLookback != 45 || got.SignalLength != 12 {
		t.Fatalf("applied = %+v", got)
	}
	if got.Source != "hlc3" || got.PivotRadius != 3 || got.DivMethod != "fractal" {
		t.Fatalf("source/pivot/method = %+v", got)
	}
	cur := GetRSXSettings()
	if cur.Length != 7 || cur.DivLookback != 45 || cur.SignalLength != 12 {
		t.Fatalf("globals = %+v", cur)
	}
}

func TestFalconEngine_SetRSXLength(t *testing.T) {
	t.Parallel()
	ResetRSXSettings()
	t.Cleanup(ResetRSXSettings)

	ApplyRSXSettings(RSXSettings{Length: 7, DivLookback: 90, SignalLength: 9})
	fast := NewFalconEngine()
	ApplyRSXSettings(RSXSettings{Length: 21, DivLookback: 90, SignalLength: 9})
	slow := NewFalconEngine()

	var vFast, vSlow float64
	for i := 0; i < 50; i++ {
		p := 100 + math.Sin(float64(i)*0.35)*5
		vFast = fast.Evaluate(p+1, p+2, p+0.5, 1000).JurikRSX
		vSlow = slow.Evaluate(p+1, p+2, p+0.5, 1000).JurikRSX
	}
	if math.Abs(vFast-vSlow) < 0.01 {
		t.Fatalf("expected different RSX with length 7 vs 21, fast=%f slow=%f", vFast, vSlow)
	}
}

func TestFalconEngine_RSXSourceClose(t *testing.T) {
	ResetRSXSettings()
	t.Cleanup(ResetRSXSettings)

	ApplyRSXSettings(RSXSettings{Length: 14, Source: "close"})
	closeEngine := NewFalconEngine()
	var vClose float64
	for i := 0; i < 80; i++ {
		h := 120.0 + float64(i%5)
		l := 100.0
		c := 118.0 + float64(i%3)
		vClose = closeEngine.Evaluate(h, l, c, 1000).JurikRSX
	}

	ApplyRSXSettings(RSXSettings{Length: 14, Source: "hlc3"})
	hlc3Engine := NewFalconEngine()
	var vHlc3 float64
	for i := 0; i < 80; i++ {
		h := 120.0 + float64(i%5)
		l := 100.0
		c := 118.0 + float64(i%3)
		vHlc3 = hlc3Engine.Evaluate(h, l, c, 1000).JurikRSX
	}
	if vClose <= 0 || vHlc3 <= 0 {
		t.Fatalf("RSX not warmed up: close=%f hlc3=%f", vClose, vHlc3)
	}
	if math.Abs(vClose-vHlc3) < 0.01 {
		t.Fatalf("close vs hlc3 RSX should differ, got %f vs %f", vClose, vHlc3)
	}
}

func TestFalconEngine_RSXSourceSnapshotIgnoresGlobalMutation(t *testing.T) {
	t.Parallel()

	ResetRSXSettings()
	t.Cleanup(ResetRSXSettings)

	ApplyRSXSettings(RSXSettings{Length: 14, Source: "close"})
	engine := NewFalconEngine()
	for i := 0; i < 40; i++ {
		engine.Evaluate(120, 100, 118, 1000)
	}
	baseline := engine.Evaluate(120, 100, 118, 1000).JurikRSX

	ApplyRSXSettings(RSXSettings{Length: 14, Source: "hlc3"})
	got := engine.Evaluate(120, 100, 118, 1000).JurikRSX
	if math.Abs(got-baseline) > 1e-9 {
		t.Fatalf("engine followed global source flip: baseline=%f got=%f", baseline, got)
	}
}

func TestFalconEngine_SetRSXSignalLength(t *testing.T) {
	t.Parallel()

	engine := NewFalconEngine()
	engine.SetRSXSignalLength(9)
	for i := 0; i < 20; i++ {
		engine.Evaluate(100+float64(i), 101+float64(i), 100.5+float64(i), 1000)
	}

	engine.SetRSXSignalLength(3)
	sig := engine.Evaluate(130, 131, 130.5, 1000)
	if sig.JurikRSXSignal <= 0 || sig.JurikRSXSignal > 100 {
		t.Fatalf("signal line out of range: %f", sig.JurikRSXSignal)
	}
}
