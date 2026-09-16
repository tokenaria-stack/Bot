package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"trading_bot/domain"
)

func TestHandleStats_ModeValidation(t *testing.T) {
	t.Parallel()

	d := &DashboardServer{tradeHistory: domain.NewTradeHistoryStore()}
	rec := httptest.NewRecorder()
	d.handleStats(rec, httptest.NewRequest(http.MethodGet, "/api/stats?mode=invalid", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleStats_PaperTrades(t *testing.T) {
	t.Parallel()

	store := domain.NewTradeHistoryStore()
	store.AppendVirtual(domain.ClosedTrade{
		Side:       "LONG",
		PnL:        1.5,
		PnLDollar:  150,
		EntryTime:  100,
		ExitTime:   200,
		EntryPrice: 100,
		ExitPrice:  101.5,
	})
	d := &DashboardServer{tradeHistory: store}

	rec := httptest.NewRecorder()
	d.handleStats(rec, httptest.NewRequest(http.MethodGet, "/api/stats?mode=paper", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp domain.SessionStats
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Mode != "paper" || resp.TotalTrades != 1 {
		t.Fatalf("resp=%+v", resp)
	}
}
