package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"trading_bot/strategy"
)

func TestHandleIndicatorSettings(t *testing.T) {
	strategy.ResetRSXSettings()
	t.Cleanup(strategy.ResetRSXSettings)

	d := &DashboardServer{}

	rec := httptest.NewRecorder()
	d.handleIndicatorSettings(rec, httptest.NewRequest(http.MethodGet, "/api/settings/indicators", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec.Code)
	}

	body, _ := json.Marshal(strategy.RSXSettings{
		Length:       21,
		DivLookback:  120,
		SignalLength: 14,
		Source:       "hlc3",
		DivMethod:    "fractal",
		PivotRadius:  2,
	})
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings/indicators", bytes.NewReader(body))
	d.handleIndicatorSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d", rec.Code)
	}

	var applied strategy.RSXSettings
	if err := json.Unmarshal(rec.Body.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if applied.Length != 21 || applied.DivLookback != 120 || applied.SignalLength != 14 {
		t.Fatalf("applied = %+v", applied)
	}
	cur := strategy.GetRSXSettings()
	if cur.Length != 21 || cur.DivLookback != 120 || cur.SignalLength != 14 {
		t.Fatalf("globals not updated: %+v", cur)
	}
}

func TestHandleHistoryChunk_BadRequest(t *testing.T) {
	t.Parallel()

	d := &DashboardServer{symbol: "BTCUSDT"}
	rec := httptest.NewRecorder()
	d.handleHistoryChunk(rec, httptest.NewRequest(http.MethodGet, "/api/history/chunk", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleRiskSettings(t *testing.T) {
	d := &DashboardServer{}

	rec := httptest.NewRecorder()
	d.handleRiskSettings(rec, httptest.NewRequest(http.MethodGet, "/api/settings/risk", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec.Code)
	}

	body, _ := json.Marshal(strategy.RiskSettings{
		RiskPerTrade:  2.0,
		MaxDrawdown:   8.0,
		Leverage:      5,
		StopLossType:  "fractal_atr",
		ATRMultiplier: 2.0,
	})
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings/risk", bytes.NewReader(body))
	d.handleRiskSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d", rec.Code)
	}

	var applied strategy.RiskSettings
	if err := json.Unmarshal(rec.Body.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if applied.RiskPerTrade != 2.0 || applied.ATRMultiplier != 2.0 || applied.StopLossType != "fractal_atr" {
		t.Fatalf("applied = %+v", applied)
	}
}

func TestParseRSXLookbackUsesGlobalSettings(t *testing.T) {
	strategy.ResetRSXSettings()
	t.Cleanup(strategy.ResetRSXSettings)

	strategy.ApplyRSXSettings(strategy.RSXSettings{
		DivLookback:  55,
		SignalLength: 9,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/state?rsxLookback=120", nil)
	if got := parseRSXLookback(req); got != 55 {
		t.Fatalf("parseRSXLookback() = %d, want 55 (global, not query)", got)
	}
}
