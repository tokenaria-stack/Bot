package exchange

import "testing"

func TestClassifyRecovery_LiveV(t *testing.T) {
	t.Parallel()
	rest := VolumeAuthority{OpenTime: 1, Open: 10, High: 11, Low: 9, Close: 10.5, Base: 100, TakerBuyBase: 40}
	cl, _ := ClassifyRecoveryRow(1, 10, 11, 9, 10.5, 40, rest, nil, 0, 0, false, false)
	if cl != RecoveryLiveVCollision {
		t.Fatalf("%s", cl)
	}
	if !RepairEligible(cl) {
		t.Fatal("live V must be repairable")
	}
	v, err := RepairNewVolume(cl, rest)
	if err != nil || v != 100 {
		t.Fatal(v, err)
	}
}

func TestClassifyRecovery_SourceConflict(t *testing.T) {
	t.Parallel()
	rest := VolumeAuthority{OpenTime: 1, Open: 10, High: 11, Low: 9, Close: 10.5, Base: 100, TakerBuyBase: 40}
	vis := VolumeAuthority{OpenTime: 1, Open: 10, High: 11, Low: 9, Close: 10.5, Base: 90}
	cl, _ := ClassifyRecoveryRow(1, 10, 11, 9, 10.5, 1663, rest, &vis, 0, 0, false, false)
	if cl != RecoverySourceConflict {
		t.Fatalf("%s", cl)
	}
	if RepairEligible(cl) {
		t.Fatal("must refuse")
	}
	if _, err := RepairNewVolume(cl, rest); err == nil {
		t.Fatal("expected refuse")
	}
}

func TestClassifyRecovery_AuthMismatchAndZero(t *testing.T) {
	t.Parallel()
	rest := VolumeAuthority{OpenTime: 1, Open: 10, High: 11, Low: 9, Close: 10.5, Base: 1392.806, TakerBuyBase: 500}
	vis := rest
	cl, _ := ClassifyRecoveryRow(1, 10, 11, 9, 10.5, 1663.097, rest, &vis, 0, 0, false, false)
	if cl != RecoveryAuthMismatch {
		t.Fatalf("%s", cl)
	}
	clz, _ := ClassifyRecoveryRow(1, 10, 11, 9, 10.5, 0, rest, &vis, 0, 0, false, false)
	if clz != RecoveryZeroHole {
		t.Fatalf("%s", clz)
	}
}

func TestClassifyRecovery_HighEnvelopeIsNotConflict(t *testing.T) {
	t.Parallel()
	rest := VolumeAuthority{OpenTime: 1, Open: 10, High: 12, Low: 8, Close: 10.5, Base: 100, TakerBuyBase: 40}
	vis := rest
	cl, _ := ClassifyRecoveryRow(1, 10, 11, 9, 10.5, 77, rest, &vis, 0, 0, false, false)
	if cl != RecoveryAuthMismatch {
		t.Fatalf("got %s want AUTH_MISMATCH", cl)
	}
}

func TestClassifyRecovery_UnknownWithoutVision(t *testing.T) {
	t.Parallel()
	rest := VolumeAuthority{OpenTime: 1, Open: 10, High: 11, Low: 9, Close: 10.5, Base: 100, TakerBuyBase: 40}
	cl, _ := ClassifyRecoveryRow(1, 10, 11, 9, 10.5, 77, rest, nil, 0, 0, false, false)
	if cl != RecoveryUnknown {
		t.Fatalf("%s", cl)
	}
}

func TestClassifyRecovery_IndexShiftTag(t *testing.T) {
	t.Parallel()
	rest := VolumeAuthority{OpenTime: 2, Open: 10, High: 11, Low: 9, Close: 10.5, Base: 100, TakerBuyBase: 40}
	vis := rest
	cl, shift := ClassifyRecoveryRow(2, 10, 11, 9, 10.5, 88, rest, &vis, 88, 0, true, false)
	if cl != RecoveryAuthMismatch || !shift {
		t.Fatalf("%s shift=%v", cl, shift)
	}
}

func TestMergeCandle_FinalVolumeAssignsLower(t *testing.T) {
	t.Parallel()
	p := NewIngressPipeline(0)
	old := Kline{OpenTime: 1, Open: 1, High: 3, Low: 1, Close: 2, Volume: 1663, CloseTime: 2}
	fin := Kline{OpenTime: 1, Open: 1, High: 2, Low: 1, Close: 2, Volume: 1392, CloseTime: 2}
	got := p.MergeCandle(old, fin, AuthorityFinal, AuthorityFinal)
	if got.Volume != 1392 {
		t.Fatalf("Volume=%v want 1392 (not MAX)", got.Volume)
	}
	if got.High != 3 {
		t.Fatalf("High envelope=%v", got.High)
	}
}

func TestMergeCandle_FinalVolumeAssignsRaise(t *testing.T) {
	t.Parallel()
	p := NewIngressPipeline(0)
	old := Kline{OpenTime: 1, Open: 1, High: 2, Low: 1, Close: 2, Volume: 100, CloseTime: 2}
	fin := Kline{OpenTime: 1, Open: 1, High: 2, Low: 1, Close: 2, Volume: 150, CloseTime: 2}
	got := p.MergeCandle(old, fin, AuthoritySettled, AuthoritySettled)
	if got.Volume != 150 {
		t.Fatalf("%v", got.Volume)
	}
}
