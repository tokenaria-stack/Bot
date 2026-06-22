package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"trading_bot/strategy"
)

func TestHandleBacktestRun_NoExchangeClient(t *testing.T) {
	d := &DashboardServer{}
	body, _ := json.Marshal(BacktestRequest{
		Symbol:    "BTCUSDT",
		Interval:  "15m",
		StartDate: "2025-01-01",
		EndDate:   "2025-06-01",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/backtest/run", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	d.handleBacktestRun(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestHandleBacktestRun_MethodNotAllowed(t *testing.T) {
	d := &DashboardServer{}
	req := httptest.NewRequest(http.MethodGet, "/api/backtest/run", nil)
	rec := httptest.NewRecorder()
	d.handleBacktestRun(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestBacktestResultFromStrategy_Convert(t *testing.T) {
	run := &strategy.BacktestRunResult{
		TotalTrades: 1,
		WinRate:     100,
		NetProfit:   2.5,
		Trades: []strategy.BacktestTradeResult{
			{Time: 1700003600, EntryTime: 1700000000, Side: "LONG", EntryPrice: 100, ExitPrice: 102, PnL: 2, Duration: "1h"},
		},
		EquityCurve: []strategy.BacktestEquityPoint{
			{Time: 1700000000, Value: 10000},
		},
		ChartData: []strategy.BacktestChartPoint{
			{Time: 1700000000, Open: 99, High: 101, Low: 98, Close: 100, RSX: 55, WozduhUp: 40, WozduhDown: 35},
		},
	}

	result := backtestResultFromStrategy(run)
	if result.TotalTrades != 1 || result.Trades[0].Side != "LONG" {
		t.Fatalf("unexpected conversion: %+v", result)
	}
	if len(result.EquityCurve) != 1 {
		t.Fatalf("equity curve len = %d, want 1", len(result.EquityCurve))
	}
	if len(result.ChartData) != 1 || result.ChartData[0].RSX != 55 {
		t.Fatalf("chart data: %+v", result.ChartData)
	}
	if result.Trades[0].EntryTime != 1700000000 {
		t.Fatalf("entryTime = %d, want 1700000000", result.Trades[0].EntryTime)
	}

	empty := backtestResultFromStrategy(nil)
	if empty.TotalTrades != 0 {
		t.Fatalf("nil run should produce empty result")
	}
}
