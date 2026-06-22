package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"trading_bot/strategy"
)

func TestHandleScoringMatrix(t *testing.T) {
	strategy.ResetScoringMatrix()
	t.Cleanup(strategy.ResetScoringMatrix)

	d := &DashboardServer{}
	body, _ := json.Marshal(map[string]bool{
		"useRSX":         false,
		"useWozduhCross": true,
		"useRedCross":    true,
		"useGeometry":    true,
		"useDivergence":  true,
		"useFib":         true,
		"useExpRegime":   true,
		"useJurikTrend":  true,
		"useWozduhSpike": true,
		"useAD":          true,
		"useAOCross":     true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/settings/matrix", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	d.handleScoringMatrix(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	m := strategy.GetScoringMatrix()
	if m.UseRSX {
		t.Fatal("UseRSX should be false")
	}
	if !m.UseWozduhCross {
		t.Fatal("UseWozduhCross should remain true")
	}
}

func TestHandleScoringMatrix_MethodNotAllowed(t *testing.T) {
	d := &DashboardServer{}
	req := httptest.NewRequest(http.MethodGet, "/api/settings/matrix", nil)
	rec := httptest.NewRecorder()
	d.handleScoringMatrix(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
