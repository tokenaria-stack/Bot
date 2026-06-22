package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"trading_bot/strategy"
)

func TestHandleThresholds(t *testing.T) {
	origL, origS := strategy.LongScoreThreshold(), strategy.ShortScoreThreshold()
	t.Cleanup(func() { strategy.SetScoreThresholds(origL, origS) })

	d := &DashboardServer{}
	body, _ := json.Marshal(map[string]int{"long": 85, "short": 90})
	req := httptest.NewRequest(http.MethodPost, "/api/settings/thresholds", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	d.handleThresholds(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strategy.LongScoreThreshold() != 85 || strategy.ShortScoreThreshold() != 90 {
		t.Fatalf("thresholds = %d/%d, want 85/90", strategy.LongScoreThreshold(), strategy.ShortScoreThreshold())
	}

	var resp struct {
		Long  int `json:"long"`
		Short int `json:"short"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Long != 85 || resp.Short != 90 {
		t.Fatalf("response = %+v, want 85/90", resp)
	}
}

func TestHandleThresholds_MethodNotAllowed(t *testing.T) {
	d := &DashboardServer{}
	req := httptest.NewRequest(http.MethodGet, "/api/settings/thresholds", nil)
	rec := httptest.NewRecorder()
	d.handleThresholds(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
