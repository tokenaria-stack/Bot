package market

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/forecast"
	"trading_bot/indicators"
)

// 1-based LONG overlay indices Kari marked TOO_TIGHT / WRONG_PIVOT (+ 23 caveat).
var pivotDisagreementOrdinals = []int{1, 2, 5, 6, 7, 9, 11, 15, 17, 18, 23}

func TestPivotDisagreementAudit_ResearchArchive(t *testing.T) {
	if os.Getenv("PIVOT_DISAGREEMENT_AUDIT") != "1" {
		t.Skip("set PIVOT_DISAGREEMENT_AUDIT=1 to audit STOP-1 disagreements")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "research", "structuralstop1", "snapshot.json"))
	if err != nil {
		t.Fatalf("need STRUCTURAL-STOP-1 snapshot: %v", err)
	}
	var snap struct {
		LongSample  []int                               `json:"long_sample"`
		Assignments []forecast.StructuralStopAssignment `json:"assignments"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	primary := loadResearchClosedBarsTF(t, "15m")
	pCanon, err := klinesToCanonical(primary)
	if err != nil {
		t.Fatal(err)
	}
	rsx := make([]float64, len(primary))
	jurik := indicators.NewJurikRSX(forecast.Spec2RSXLength)
	for i, k := range primary {
		rsx[i] = jurik.Update((k.High + k.Low + k.Close) / 3)
	}
	s2 := ResearchStructuralStopFractalSettings()
	s6 := s2
	s6.PivotRadius = 6
	facts2 := RSTFractalFactsFromClosedSeries(primary, rsx, s2)
	facts6 := RSTFractalFactsFromClosedSeries(primary, rsx, s6)
	opens := make([]int64, len(primary))
	closes := make([]float64, len(primary))
	for i, k := range primary {
		opens[i] = k.OpenTime
		closes[i] = k.Close
	}
	tv := indicators.TVPivotFacts(closes, rsx, opens, forecast.Spec2TVLookback)

	var rows []forecast.PivotDisagreementSample
	for _, ord := range pivotDisagreementOrdinals {
		if ord < 1 || ord > len(snap.LongSample) {
			t.Fatalf("ordinal %d", ord)
		}
		ai := snap.LongSample[ord-1]
		if ai < 0 || ai >= len(snap.Assignments) {
			t.Fatalf("assignment %d", ai)
		}
		a := snap.Assignments[ai]
		if a.Side != forecast.GeomSideLong {
			t.Fatalf("sample %d not LONG", ord)
		}
		rows = append(rows, forecast.AuditPivotDisagreement(a, ord, pCanon, facts2, facts6, tv))
	}
	text := forecast.FormatPivotDisagreementAudit(rows)
	out := filepath.Join(root, "research", "structuralstop1", "pivot_disagreement_audit.txt")
	if err := os.WriteFile(out, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, text)
}
