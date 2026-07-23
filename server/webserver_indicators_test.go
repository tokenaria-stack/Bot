package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"trading_bot/market"
)

func TestHandleIndicatorSettings(t *testing.T) {
	market.ResetRSXSettings()
	market.SetRSXSettingsPath(filepath.Join(t.TempDir(), "rsx_settings.json"))
	t.Cleanup(func() {
		market.ResetRSXSettings()
		market.SetRSXSettingsPath("")
	})

	d := &DashboardServer{}

	rec := httptest.NewRecorder()
	d.handleIndicatorSettings(rec, httptest.NewRequest(http.MethodGet, "/api/settings/indicators", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec.Code)
	}
	var getBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &getBody); err != nil {
		t.Fatal(err)
	}
	settingsRaw, _ := json.Marshal(getBody["settings"])
	var gotSettings market.RSXSettings
	_ = json.Unmarshal(settingsRaw, &gotSettings)
	if gotSettings.Source != "hlc3" {
		t.Fatalf("default source = %q, want hlc3", gotSettings.Source)
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

	var applied map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	settingsRaw, _ = json.Marshal(applied["settings"])
	var s market.RSXSettings
	_ = json.Unmarshal(settingsRaw, &s)
	if s.Length != 21 || s.DivLookback != 120 || s.SignalLength != 14 {
		t.Fatalf("applied = %+v", s)
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
