package forecast

import (
	"testing"

	"trading_bot/indicators"
)

func TestAuditPivotDisagreement_Cases(t *testing.T) {
	t.Parallel()
	bars := []CanonicalClosedBar{
		{OpenTime: 100, Low: 90, High: 110, Close: 100},
		{OpenTime: 200, Low: 91, High: 111, Close: 101},
		{OpenTime: 300, Low: 95, High: 112, Close: 102},
		{OpenTime: 400, Low: 96, High: 113, Close: 103},
	}
	a := StructuralStopAssignment{
		Side: GeomSideLong, At: 400, Entry: 103,
		PivotAnchorAt: 100, PivotConfirmedAt: 200, Stop: 90,
	}
	laterR2 := []indicators.IndicatorFactEvent{{
		Source: indicators.FactSourceRSXFractalPivot, Direction: indicators.FactDirPivotLow,
		AnchorAt: 200, ConfirmedAt: 300,
	}}
	got := AuditPivotDisagreement(a, 1, bars, laterR2, nil, nil)
	if got.Verdict != PivotCaseALaterR2LostToConfirmRank {
		t.Fatalf("A: %s", got.Verdict)
	}

	unconf := []indicators.IndicatorFactEvent{{
		Source: indicators.FactSourceRSXFractalPivot, Direction: indicators.FactDirPivotLow,
		AnchorAt: 300, ConfirmedAt: 500,
	}}
	got = AuditPivotDisagreement(a, 2, bars, unconf, nil, nil)
	if got.Verdict != PivotCaseBLaterR2Unconfirmed {
		t.Fatalf("B: %s", got.Verdict)
	}

	r6 := []indicators.IndicatorFactEvent{{
		Source: indicators.FactSourceRSXFractalPivot, Direction: indicators.FactDirPivotLow,
		AnchorAt: 200, ConfirmedAt: 350,
	}}
	got = AuditPivotDisagreement(a, 3, bars, nil, r6, nil)
	if got.Verdict != PivotCaseCLaterR6Only {
		t.Fatalf("C: %s", got.Verdict)
	}

	got = AuditPivotDisagreement(a, 4, bars, nil, nil, nil)
	if got.Verdict != PivotCaseDNoLaterFractalFact {
		t.Fatalf("D: %s", got.Verdict)
	}
}
