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
	histSave(t, "BTCUSDT", "1m", []int64{end})
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
	if d.shouldEnsureHistory(spec, now, HistoryWindow{}) {
		t.Fatal("live end must not EnsureHistoryWindow")
	}
}

func TestShouldEnsureHistory_WarmupConstant(t *testing.T) {
	d := &DashboardServer{}
	if d.historyWantBars(3000) != 3000+market.IndicatorWarmupBars {
		t.Fatalf("wantBars=%d", d.historyWantBars(3000))
	}
}

func histQueryTF(tf string, endMs int64, limit int) HistoryWindowQuery {
	spec, err := ResolveTimeframe(tf)
	if err != nil {
		panic(err)
	}
	return HistoryWindowQuery{Spec: spec, EndTimeMs: endMs, CandleLimit: limit}
}

func histSave(t *testing.T, symbol, interval string, opens []int64) {
	t.Helper()
	rows := make([]data.Candle, len(opens))
	for i, open := range opens {
		rows[i] = data.Candle{
			OpenTime: open, CloseTime: open + 59_999,
			Open: 1, High: 2, Low: 1, Close: float64(i + 1), Volume: 1,
		}
	}
	if err := data.SaveKlines(symbol, interval, rows); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryTailCoversEnd_Predicate(t *testing.T) {
	end := histJan2023Ms
	win := HistoryWindow{Klines: []exchange.Kline{{OpenTime: end - 60_000}, {OpenTime: end}}}
	if !historyTailCoversEnd(win, end) {
		t.Fatal("equal tail must HIT")
	}
	stale := HistoryWindow{Klines: []exchange.Kline{{OpenTime: end - 60_000}}}
	if historyTailCoversEnd(stale, end) {
		t.Fatal("stale tail must not HIT")
	}
	newer := HistoryWindow{Klines: []exchange.Kline{{OpenTime: end + 60_000}}}
	if historyTailCoversEnd(newer, end) {
		t.Fatal("last > expected must not HIT")
	}
	if historyTailCoversEnd(HistoryWindow{}, end) {
		t.Fatal("empty must not HIT")
	}
}

func TestDeliverHistoryWindow_PreFuturesStaleTailNoREST(t *testing.T) {
	d := histServer(t)
	focus := int64(1_529_064_000_000) // 2018-06-15 12:00 UTC
	stale := int64(1_514_847_600_000) // 2018-01-01 23:00 UTC
	histSave(t, exchange.SpotStorageSymbol("BTCUSDT"), "15m", []int64{stale})
	var fetches atomic.Int32
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		fetches.Add(1)
		return nil, fmt.Errorf("futures REST must not run pre-futures")
	}
	got := d.deliverHistoryWindow(context.Background(), histQueryTF("15m", focus, 10))
	if got.Kind != historyDeliverNoData || got.Code != HistoryCodeWindowUnavailable {
		t.Fatalf("kind=%v code=%s want WINDOW_UNAVAILABLE", got.Kind, got.Code)
	}
	if fetches.Load() != 0 {
		t.Fatalf("fetches=%d want 0", fetches.Load())
	}
}

func TestDeliverHistoryWindow_Seam15mStaleThenUnavailable(t *testing.T) {
	d := histServer(t)
	focus := int64(1_567_958_400_000) // 2019-09-08 16:00 UTC futures 4h/15m aligned
	stale := int64(1_514_847_600_000)
	histSave(t, exchange.SpotStorageSymbol("BTCUSDT"), "15m", []int64{stale})
	var fetches atomic.Int32
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		fetches.Add(1)
		if symbol != "BTCUSDT" {
			t.Errorf("symbol=%s", symbol)
		}
		return nil, nil
	}
	got := d.deliverHistoryWindow(context.Background(), histQueryTF("15m", focus, 10))
	if fetches.Load() != 1 {
		t.Fatalf("fetches=%d want 1 Ensure", fetches.Load())
	}
	if got.Kind != historyDeliverNoData || got.Code != HistoryCodeWindowUnavailable {
		t.Fatalf("kind=%v code=%s want WINDOW_UNAVAILABLE (must not pack 2018)", got.Kind, got.Code)
	}
}

func TestDeliverHistoryWindow_Seam1hStaleThenUnavailable(t *testing.T) {
	d := histServer(t)
	focus := int64(1_567_958_400_000) // 2019-09-08 16:00
	stale := int64(1_567_897_200_000) // 2019-09-07 23:00
	histSave(t, exchange.SpotStorageSymbol("BTCUSDT"), "1h", []int64{stale})
	var fetches atomic.Int32
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		fetches.Add(1)
		return nil, nil
	}
	got := d.deliverHistoryWindow(context.Background(), histQueryTF("1h", focus, 10))
	if fetches.Load() != 1 {
		t.Fatalf("fetches=%d want 1", fetches.Load())
	}
	if got.Kind != historyDeliverNoData || got.Code != HistoryCodeWindowUnavailable {
		t.Fatalf("kind=%v code=%s", got.Kind, got.Code)
	}
}

func TestDeliverHistoryWindow_Seam4hExactHitNoREST(t *testing.T) {
	d := histServer(t)
	focus := int64(1_567_958_400_000) // 2019-09-08 16:00 4h open
	histSave(t, "BTCUSDT", "4h", []int64{focus})
	var fetches atomic.Int32
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		fetches.Add(1)
		return nil, fmt.Errorf("REST must not run on 4h seam HIT")
	}
	got := d.deliverHistoryWindow(context.Background(), histQueryTF("4h", focus, 10))
	if got.Kind != historyDeliverOK {
		t.Fatalf("kind=%v code=%s", got.Kind, got.Code)
	}
	if fetches.Load() != 0 {
		t.Fatalf("fetches=%d want 0", fetches.Load())
	}
	last := got.Win.Klines[len(got.Win.Klines)-1].OpenTime
	if last != focus {
		t.Fatalf("last=%d want %d", last, focus)
	}
}

func TestDeliverHistoryWindow_2023_15mLocalHit(t *testing.T) {
	d := histServer(t)
	histSave(t, "BTCUSDT", "15m", []int64{histJan2023Ms})
	var fetches atomic.Int32
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		fetches.Add(1)
		return nil, fmt.Errorf("REST must not run")
	}
	got := d.deliverHistoryWindow(context.Background(), histQueryTF("15m", histJan2023Ms, 10))
	if got.Kind != historyDeliverOK || fetches.Load() != 0 {
		t.Fatalf("kind=%v fetches=%d", got.Kind, fetches.Load())
	}
}

func TestWriteColumnarHistory_WindowUnavailableHTTP200(t *testing.T) {
	d := histServer(t)
	stale := int64(1_514_847_600_000)
	histSave(t, exchange.SpotStorageSymbol("BTCUSDT"), "15m", []int64{stale})
	d.ensureFetch = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		return nil, nil
	}
	req := httptest.NewRequest(http.MethodGet,
		"/api/history?tf=15m&endTime=1567958400&limit=10&format=columnar", nil)
	rec := httptest.NewRecorder()
	d.handleHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "no_data" || payload["code"] != HistoryCodeWindowUnavailable {
		t.Fatalf("payload=%v", payload)
	}
	if times, _ := payload["times"].([]any); len(times) != 0 {
		t.Fatalf("must not pack stale times: %v", payload["times"])
	}
}
