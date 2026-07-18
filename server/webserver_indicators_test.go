package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"trading_bot/market"
)

func TestHandleIndicatorSettings(t *testing.T) {
	market.ResetRSXSettings()
	t.Cleanup(market.ResetRSXSettings)

	d := &DashboardServer{}

	rec := httptest.NewRecorder()
	d.handleIndicatorSettings(rec, httptest.NewRequest(http.MethodGet, "/api/settings/indicators", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec.Code)
	}

	body, _ := json.Marshal(market.RSXSettings{
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

	var applied market.RSXSettings
	if err := json.Unmarshal(rec.Body.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if applied.Length != 21 || applied.DivLookback != 120 || applied.SignalLength != 14 {
		t.Fatalf("applied = %+v", applied)
	}
	cur := market.GetRSXSettings()
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

func TestParseRSXLookbackUsesGlobalSettings(t *testing.T) {
	market.ResetRSXSettings()
	t.Cleanup(market.ResetRSXSettings)

	market.ApplyRSXSettings(market.RSXSettings{
		DivLookback:  55,
		SignalLength: 9,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/state?rsxLookback=120", nil)
	if got := parseRSXLookback(req); got != 55 {
		t.Fatalf("parseRSXLookback() = %d, want 55 (global, not query)", got)
	}
}
