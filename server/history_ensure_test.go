package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/market"
	"trading_bot/server/wire"
	"trading_bot/ui_config"
)

const histJan2023Ms = int64(1_672_765_200_000) // 2023-01-03 17:00:00 UTC

func histTestCandle(openMs int64, closePx float64) exchange.Candle {
	return exchange.Candle{
		OpenTime:  openMs,
		CloseTime: openMs + 59_999,
		Open:      closePx,
		High:      closePx + 1,
		Low:       closePx - 1,
		Close:     closePx,
		Volume:    1,
	}
}

func histBars(endMs int64, n int) []exchange.Candle {
	out := make([]exchange.Candle, n)
	for i := 0; i < n; i++ {
		open := endMs - int64(n-1-i)*60_000
		out[i] = histTestCandle(open, float64(100+i))
	}
	return out
}

func histServer(t *testing.T) *DashboardServer {
	t.Helper()
	data.ResetDBForTest(filepath.Join(t.TempDir(), "hist_ensure.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}
	q := data.NewPersistenceQueue(64)
	q.SetSaveFunc(data.SaveKlines)
	reg, err := ui_config.BuildUIRegistry()
	if err != nil {
		t.Fatal(err)
	}
	return &DashboardServer{
		symbol:    "BTCUSDT",
		persistQ:  q,
		projector: wire.NewProjector(reg),
	}
}

func histQuery(endMs int64, limit int) HistoryWindowQuery {
	spec, err := ResolveTimeframe("1m")
	if err != nil {
		panic(err)
	}
	return HistoryWindowQuery{Spec: spec, EndTimeMs: endMs, CandleLimit: limit}
}

func TestHistoricalAcquireRange_FuturesEraLocal(t *testing.T) {
	from, to, code, err := historicalAcquireRange(histJan2023Ms, 3300, "1m")
	if err != nil {
		t.Fatal(err)
	}
	if code != HistoryCodeOK {
		t.Fatalf("code=%s want OK", code)
	}
	if to != histJan2023Ms {
		t.Fatalf("to=%d want %d", to, histJan2023Ms)
	}
	if from < exchange.BinanceFuturesGenesisMs {
		t.Fatalf("from=%d crossed futures genesis", from)
	}
	if from >= to {
		t.Fatalf("from=%d to=%d", from, to)
	}
	spanBars := (to-from)/60_000 + 1
	if spanBars > 3300+2 || spanBars < 3200 {
		t.Fatalf("spanBars=%d want ~3300", spanBars)
	}
}

func TestHistoricalAcquireRange_PreFuturesNoData(t *testing.T) {
	_, _, code, err := historicalAcquireRange(exchange.BinanceFuturesGenesisMs-1, 100, "1m")
	if err != nil {
		t.Fatal(err)
	}
	if code != HistoryCodeNoData {
		t.Fatalf("code=%s want NO_DATA", code)
	}
}

func TestDeliverHistoryWindow_ArchiveHitNoREST(t *testing.T) {
	d := histServer(t)
	end := histJan2023Ms
	rows := make([]data.Candle, 40)
	for i := range rows {
		open := end - int64(len(rows)-1-i)*60_000
		rows[i] = data.Candle{
			OpenTime: open, CloseTime: open + 59_999,
			Open: 1, High: 2, Low: 1, Close: float64(i), Volume: 1,
		}
	}
	if err := data.SaveKlines("BTCUSDT", "1m", rows); err != nil {
		t.Fatal(err)
	}
	var fetches atomic.Int32
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		fetches.Add(1)
		return nil, fmt.Errorf("REST must not run on archive hit")
	}
	got := d.deliverHistoryWindow(context.Background(), histQuery(end, 10))
	if got.Kind != historyDeliverOK {
		t.Fatalf("kind=%v want OK", got.Kind)
	}
	if len(got.Win.Klines) == 0 {
		t.Fatal("expected bars")
	}
	if fetches.Load() != 0 {
		t.Fatalf("fetches=%d want 0", fetches.Load())
	}
}

func TestDeliverHistoryWindow_MissThenRESTPersistReread(t *testing.T) {
	d := histServer(t)
	end := histJan2023Ms
	bars := histBars(end, 12)
	var fetches atomic.Int32
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		fetches.Add(1)
		if symbol != "BTCUSDT" {
			t.Errorf("symbol=%s want BTCUSDT futures", symbol)
		}
		if fromMs < exchange.BinanceFuturesGenesisMs {
			t.Errorf("fromMs=%d pre-genesis", fromMs)
		}
		if toMs < end {
			t.Errorf("toMs=%d < end %d", toMs, end)
		}
		return bars, nil
	}
	got := d.deliverHistoryWindow(context.Background(), histQuery(end, 10))
	if got.Kind != historyDeliverOK {
		t.Fatalf("kind=%v err=%v", got.Kind, got.Err)
	}
	if fetches.Load() != 1 {
		t.Fatalf("fetches=%d want 1", fetches.Load())
	}
	if len(got.Win.Klines) < 12 {
		t.Fatalf("klines=%d want >=12", len(got.Win.Klines))
	}
	last := got.Win.Klines[len(got.Win.Klines)-1].OpenTime
	if last != end {
		t.Fatalf("tip open=%d want %d (must be Jan 2023, not live)", last, end)
	}

	got2 := d.deliverHistoryWindow(context.Background(), histQuery(end, 10))
	if got2.Kind != historyDeliverOK {
		t.Fatalf("repeat kind=%v", got2.Kind)
	}
	if fetches.Load() != 1 {
		t.Fatalf("repeat fetches=%d want 1 (idempotent)", fetches.Load())
	}
}

func TestDeliverHistoryWindow_ExchangeEmptyIsNoData(t *testing.T) {
	d := histServer(t)
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		return nil, nil
	}
	got := d.deliverHistoryWindow(context.Background(), histQuery(histJan2023Ms, 10))
	if got.Kind != historyDeliverNoData {
		t.Fatalf("kind=%v want NO_DATA", got.Kind)
	}
}

func TestDeliverHistoryWindow_ExchangeError(t *testing.T) {
	d := histServer(t)
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		return nil, fmt.Errorf("binance down")
	}
	got := d.deliverHistoryWindow(context.Background(), histQuery(histJan2023Ms, 10))
	if got.Kind != historyDeliverExchange {
		t.Fatalf("kind=%v want exchange", got.Kind)
	}
}

func TestDeliverHistoryWindow_EraDoesNotRequestSpot(t *testing.T) {
	d := histServer(t)
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		if symbol != "BTCUSDT" {
			return nil, fmt.Errorf("must not fetch %s", symbol)
		}
		if fromMs < exchange.BinanceFuturesGenesisMs {
			return nil, fmt.Errorf("fromMs %d is pre-futures", fromMs)
		}
		return histBars(histJan2023Ms, 3), nil
	}
	got := d.deliverHistoryWindow(context.Background(), histQuery(histJan2023Ms, 5))
	if got.Kind != historyDeliverOK {
		t.Fatalf("kind=%v err=%v", got.Kind, got.Err)
	}
}

func TestEnsureHistoryWindow_SingleFlight(t *testing.T) {
	d := histServer(t)
	var fetches atomic.Int32
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		fetches.Add(1)
		time.Sleep(80 * time.Millisecond)
		return histBars(histJan2023Ms, 4), nil
	}
	var wg sync.WaitGroup
	wg.Add(2)
	var a, b HistoryEnsureResult
	go func() {
		defer wg.Done()
		a = d.EnsureHistoryWindow(context.Background(), "BTCUSDT", "1m", histJan2023Ms, 10)
	}()
	go func() {
		defer wg.Done()
		b = d.EnsureHistoryWindow(context.Background(), "BTCUSDT", "1m", histJan2023Ms, 10)
	}()
	wg.Wait()
	if fetches.Load() != 1 {
		t.Fatalf("fetches=%d want 1", fetches.Load())
	}
	if a.Code != HistoryCodeOK || b.Code != HistoryCodeOK {
		t.Fatalf("codes %s %s", a.Code, b.Code)
	}
}

func TestWriteColumnarHistory_NoDataHTTP200(t *testing.T) {
	d := histServer(t)
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		return nil, nil
	}
	req := httptest.NewRequest(http.MethodGet,
		"/api/history?tf=1m&endTime=1672765200&limit=10&format=columnar", nil)
	rec := httptest.NewRecorder()
	d.handleHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "no_data" || payload["code"] != HistoryCodeNoData {
		t.Fatalf("payload=%v", payload)
	}
}

func TestWriteColumnarHistory_ExchangeFailedHTTP502(t *testing.T) {
	d := histServer(t)
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		return nil, fmt.Errorf("timeout")
	}
	req := httptest.NewRequest(http.MethodGet,
		"/api/history?tf=1m&endTime=1672765200&limit=10&format=columnar", nil)
	rec := httptest.NewRecorder()
	d.handleHistory(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status=%d want 502 body=%s", rec.Code, rec.Body.String())
	}
}

func TestShouldEnsureHistory_LiveEndSkipped(t *testing.T) {
	d := &DashboardServer{symbol: "BTCUSDT"}
	spec, err := ResolveTimeframe("1m")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	if d.shouldEnsureHistory(spec, now, HistoryWindow{}, false) {
		t.Fatal("live end must not EnsureHistoryWindow")
	}
}

func TestShouldEnsureHistory_WarmupConstant(t *testing.T) {
	d := &DashboardServer{}
	if d.historyWantBars(3000) != 3000+market.IndicatorWarmupBars {
		t.Fatalf("wantBars=%d", d.historyWantBars(3000))
	}
}
