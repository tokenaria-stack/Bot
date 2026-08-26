package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/market"
)

// HIST-0 — historical window identity (interactive /api/history only).
//
// A request is (symbol, interval, endTimeMs, wantBars). The window is era-local:
//
//	endTimeMs >= BinanceFuturesGenesisMs → acquire BTCUSDT futures only
//	endTimeMs <  genesis                 → do not fetch futures; no spot REST
//	                                       in this chapter (DATA-1)
//
// startMs ≈ RetreatBarOpen(endTimeMs, wantBars-1), then clamp to the era floor.
// Never walk backward across an era boundary until N rows exist.
// Never use older spot bars as a count-filler for a wholly post-genesis request
// (a 2023 1m microscope must not become 2019 spot 1m).
//
// GetWindow remains a read (SQLite ∪ RAM). EnsureHistoryWindow acquires + persists.
// /api/history orchestrates: read → maybe ensure → read again → pack.

const (
	HistoryCodeOK                = "OK"
	HistoryCodeArchiveMiss       = "ARCHIVE_MISS"
	HistoryCodeNoData            = "NO_DATA"
	HistoryCodeWindowUnavailable = "WINDOW_UNAVAILABLE"
	HistoryCodeExchangeFailed    = "EXCHANGE_FETCH_FAILED"
	HistoryCodeSQLiteError       = "SQLITE_ERROR"
)

type historyEnsureCall struct {
	wg  sync.WaitGroup
	res HistoryEnsureResult
}

// HistoryEnsureResult is the typed outcome of EnsureHistoryWindow.
type HistoryEnsureResult struct {
	Code     string
	FromMs   int64
	ToMs     int64
	Acquired int
	Err      error
}

type closedRangeFetchFunc func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error)

func (d *DashboardServer) SetPersistenceQueue(q *data.PersistenceQueue) {
	if d == nil {
		return
	}
	d.persistQ = q
}

func (d *DashboardServer) historyWantBars(candleLimit int) int {
	limit := candleLimit
	if limit <= 0 {
		limit = defaultStateCandleLimit
	}
	return limit + market.IndicatorWarmupBars
}

// historicalAcquireRange is the era-local REST window (HIST-0).
func historicalAcquireRange(endTimeMs int64, wantBars int, interval string) (fromMs, toMs int64, code string, err error) {
	if endTimeMs <= 0 || wantBars <= 0 || interval == "" {
		return 0, 0, HistoryCodeNoData, nil
	}
	if endTimeMs < exchange.BinanceFuturesGenesisMs {
		return 0, 0, HistoryCodeNoData, nil
	}
	toMs = endTimeMs
	n := wantBars - 1
	if n < 0 {
		n = 0
	}
	fromMs, err = data.RetreatBarOpen(endTimeMs, n, interval)
	if err != nil {
		return 0, 0, HistoryCodeNoData, err
	}
	genesis := exchange.BinanceFuturesGenesisMs
	if fromMs < genesis {
		if aligned, aerr := data.CurrentBarOpen(genesis, interval); aerr == nil {
			fromMs = aligned
		} else {
			fromMs = genesis
		}
	}
	if fromMs > toMs {
		return 0, 0, HistoryCodeNoData, nil
	}
	return fromMs, toMs, HistoryCodeOK, nil
}

func historySpecInterval(spec TimeframeSpec) string {
	if spec.BinanceInterval != "" {
		return spec.BinanceInterval
	}
	return spec.ID
}

// expectedHistoryLastOpen is the canonical closed-bar open at/before cappedEnd.
func expectedHistoryLastOpen(cappedEndMs int64, interval string) (int64, error) {
	if cappedEndMs <= 0 || interval == "" {
		return 0, fmt.Errorf("expectedHistoryLastOpen: end and interval required")
	}
	return data.CurrentBarOpen(cappedEndMs, interval)
}

// historyTailCoversEnd is the HIST-1.1 HIT predicate: last GetWindow open
// equals expectedLastOpen. Empty and last > expected are not hits.
func historyTailCoversEnd(win HistoryWindow, expectedLastOpen int64) bool {
	if expectedLastOpen <= 0 || len(win.Klines) == 0 {
		return false
	}
	last := win.Klines[len(win.Klines)-1].OpenTime
	return last == expectedLastOpen
}

func (d *DashboardServer) shouldEnsureHistory(spec TimeframeSpec, endTimeMs int64, win HistoryWindow) bool {
	if d == nil {
		return false
	}
	if spec.Kind != TFBinanceREST || spec.BinanceInterval == "" {
		return false
	}
	if !d.isHistoricalKlineEnd(endTimeMs, spec.BinanceInterval) {
		return false
	}
	if endTimeMs < exchange.BinanceFuturesGenesisMs {
		return false
	}
	expected, err := expectedHistoryLastOpen(endTimeMs, spec.BinanceInterval)
	if err != nil {
		return true
	}
	return !historyTailCoversEnd(win, expected)
}

func (d *DashboardServer) fetchClosedRangePages(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
	if d != nil && d.ensureFetch != nil {
		return d.ensureFetch(symbol, interval, fromMs, toMs)
	}
	if d == nil || d.rest == nil {
		return nil, fmt.Errorf("exchange client unavailable")
	}
	return d.rest.FetchClosedRangePagesExact(symbol, interval, fromMs, toMs)
}

// EnsureHistoryWindow acquires one era-local missing window via existing REST
// helpers and PersistenceQueue. Process-local single-flight is keyed by
// symbol+interval. SQLite remains the durable hit/miss source.
func (d *DashboardServer) EnsureHistoryWindow(ctx context.Context, symbol, interval string, endTimeMs int64, wantBars int) HistoryEnsureResult {
	if d == nil {
		return HistoryEnsureResult{Code: HistoryCodeSQLiteError, Err: fmt.Errorf("dashboard server is nil")}
	}
	if !exchange.IsNativeBinance(interval) {
		return HistoryEnsureResult{Code: HistoryCodeNoData, Err: fmt.Errorf("EnsureHistoryWindow is native-only (got %q)", interval)}
	}
	symbol = exchange.NormalizeFuturesSymbol(symbol)
	key := symbol + "\x00" + interval

	d.ensureMu.Lock()
	if d.ensureInFlight == nil {
		d.ensureInFlight = make(map[string]*historyEnsureCall)
	}
	if call, ok := d.ensureInFlight[key]; ok {
		d.ensureMu.Unlock()
		call.wg.Wait()
		return call.res
	}
	call := &historyEnsureCall{}
	call.wg.Add(1)
	d.ensureInFlight[key] = call
	d.ensureMu.Unlock()

	res := d.ensureHistoryWindowUnshared(ctx, symbol, interval, endTimeMs, wantBars)
	call.res = res
	call.wg.Done()

	d.ensureMu.Lock()
	delete(d.ensureInFlight, key)
	d.ensureMu.Unlock()
	return res
}

func (d *DashboardServer) ensureHistoryWindowUnshared(ctx context.Context, symbol, interval string, endTimeMs int64, wantBars int) HistoryEnsureResult {
	if err := requestCtxErr(ctx); err != nil {
		return HistoryEnsureResult{Code: HistoryCodeExchangeFailed, Err: err}
	}
	fromMs, toMs, code, err := historicalAcquireRange(endTimeMs, wantBars, interval)
	if err != nil {
		return HistoryEnsureResult{Code: HistoryCodeNoData, FromMs: fromMs, ToMs: toMs, Err: err}
	}
	if code == HistoryCodeNoData {
		return HistoryEnsureResult{Code: HistoryCodeNoData, FromMs: fromMs, ToMs: toMs}
	}

	candles, ferr := d.fetchClosedRangePages(symbol, interval, fromMs, toMs)
	if ferr != nil {
		return HistoryEnsureResult{
			Code: HistoryCodeExchangeFailed, FromMs: fromMs, ToMs: toMs, Err: ferr,
		}
	}
	if len(candles) == 0 {
		return HistoryEnsureResult{Code: HistoryCodeNoData, FromMs: fromMs, ToMs: toMs}
	}

	if d.persistQ == nil {
		return HistoryEnsureResult{
			Code: HistoryCodeSQLiteError, FromMs: fromMs, ToMs: toMs, Acquired: len(candles),
			Err: fmt.Errorf("persistence queue not bound"),
		}
	}
	if err := d.persistQ.PersistClosedBarsNow(symbol, interval, exchange.CandlesToData(candles)); err != nil {
		return HistoryEnsureResult{
			Code: HistoryCodeSQLiteError, FromMs: fromMs, ToMs: toMs, Acquired: len(candles), Err: err,
		}
	}
	return HistoryEnsureResult{Code: HistoryCodeOK, FromMs: fromMs, ToMs: toMs, Acquired: len(candles)}
}

type historyDeliverKind int

const (
	historyDeliverOK historyDeliverKind = iota
	historyDeliverNoData
	historyDeliverExchange
	historyDeliverSQLite
)

type historyDeliverResult struct {
	Kind historyDeliverKind
	Code string
	Win  HistoryWindow
	Err  error
}

func historyUnavailableResult(win HistoryWindow) historyDeliverResult {
	if len(win.Klines) > 0 {
		return historyDeliverResult{Kind: historyDeliverNoData, Code: HistoryCodeWindowUnavailable}
	}
	return historyDeliverResult{Kind: historyDeliverNoData, Code: HistoryCodeNoData}
}

func (d *DashboardServer) packIfHistoricalTailHit(win HistoryWindow, expectedLastOpen int64) historyDeliverResult {
	if historyTailCoversEnd(win, expectedLastOpen) {
		return historyDeliverResult{Kind: historyDeliverOK, Code: HistoryCodeOK, Win: win}
	}
	return historyUnavailableResult(win)
}

// deliverHistoryWindow is the /api/history orchestration: GetWindow (read) →
// EnsureHistoryWindow on miss → GetWindow again. Never returns REST candles as the payload.
// HIST-1.1: historical HIT is last GetWindow OpenTime == CurrentBarOpen(cappedEnd).
func (d *DashboardServer) deliverHistoryWindow(ctx context.Context, q HistoryWindowQuery) historyDeliverResult {
	if exchange.IsDerivedTime(q.Spec.ID) {
		return d.deliverDerivedHistoryWindow(ctx, q)
	}
	if exchange.IsLiveSecond(q.Spec.ID) {
		win, ok := d.GetWindow(ctx, q)
		if err := requestCtxErr(ctx); err != nil {
			return historyDeliverResult{Kind: historyDeliverExchange, Err: err}
		}
		if q.StartTimeMs > 0 {
			if !ok {
				return historyDeliverResult{Kind: historyDeliverNoData, Code: HistoryCodeNoData}
			}
			return historyDeliverResult{Kind: historyDeliverOK, Code: HistoryCodeOK, Win: win}
		}
		if !ok || len(win.Klines) == 0 {
			return historyDeliverResult{Kind: historyDeliverNoData, Code: HistoryCodeNoData}
		}
		return historyDeliverResult{Kind: historyDeliverOK, Code: HistoryCodeOK, Win: win}
	}
	if exchange.IsSparseSecondChild(q.Spec.ID) {
		win, ok := d.GetWindow(ctx, q)
		if err := requestCtxErr(ctx); err != nil {
			return historyDeliverResult{Kind: historyDeliverExchange, Err: err}
		}
		if q.StartTimeMs > 0 {
			if !ok {
				return historyDeliverResult{Kind: historyDeliverNoData, Code: HistoryCodeNoData}
			}
			return historyDeliverResult{Kind: historyDeliverOK, Code: HistoryCodeOK, Win: win}
		}
		if !ok {
			return historyDeliverResult{Kind: historyDeliverNoData, Code: HistoryCodeNoData}
		}
		return historyDeliverResult{Kind: historyDeliverOK, Code: HistoryCodeOK, Win: win}
	}
	win, ok := d.GetWindow(ctx, q)
	if err := requestCtxErr(ctx); err != nil {
		return historyDeliverResult{Kind: historyDeliverExchange, Err: err}
	}
	interval := historySpecInterval(q.Spec)
	endMs := resolveClosedBarBoundary(q.EndTimeMs, interval)
	historical := d.isHistoricalKlineEnd(endMs, interval)

	if !historical {
		if !ok || len(win.Klines) == 0 {
			return historyDeliverResult{Kind: historyDeliverNoData, Code: HistoryCodeNoData}
		}
		return historyDeliverResult{Kind: historyDeliverOK, Code: HistoryCodeOK, Win: win}
	}

	expected, err := expectedHistoryLastOpen(endMs, interval)
	if err != nil {
		return historyDeliverResult{Kind: historyDeliverNoData, Code: HistoryCodeNoData, Err: err}
	}
	if historyTailCoversEnd(win, expected) {
		return historyDeliverResult{Kind: historyDeliverOK, Code: HistoryCodeOK, Win: win}
	}

	if !d.shouldEnsureHistory(q.Spec, endMs, win) {
		return historyUnavailableResult(win)
	}

	want := d.historyWantBars(q.CandleLimit)
	symbol := d.symbol
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	ens := d.EnsureHistoryWindow(ctx, symbol, q.Spec.BinanceInterval, endMs, want)
	switch ens.Code {
	case HistoryCodeExchangeFailed:
		log.Printf("[Dashboard] EnsureHistoryWindow exchange %s %s: %v", symbol, q.Spec.BinanceInterval, ens.Err)
		return historyDeliverResult{Kind: historyDeliverExchange, Code: HistoryCodeExchangeFailed, Err: ens.Err}
	case HistoryCodeSQLiteError:
		log.Printf("[Dashboard] EnsureHistoryWindow sqlite %s %s: %v", symbol, q.Spec.BinanceInterval, ens.Err)
		return historyDeliverResult{Kind: historyDeliverSQLite, Code: HistoryCodeSQLiteError, Err: ens.Err}
	}

	win2, ok2 := d.GetWindow(ctx, q)
	if err := requestCtxErr(ctx); err != nil {
		return historyDeliverResult{Kind: historyDeliverExchange, Err: err}
	}
	if !ok2 {
		win2 = HistoryWindow{}
	}
	return d.packIfHistoricalTailHit(win2, expected)
}

func (d *DashboardServer) deliverDerivedHistoryWindow(ctx context.Context, q HistoryWindowQuery) historyDeliverResult {
	e, ok := exchange.TimeframeByName(q.Spec.ID)
	if !ok || e.Parent == "" {
		return historyDeliverResult{Kind: historyDeliverNoData, Code: HistoryCodeNoData}
	}
	parentSpec, err := ResolveTimeframe(e.Parent)
	if err != nil {
		return historyDeliverResult{Kind: historyDeliverNoData, Code: HistoryCodeNoData, Err: err}
	}
	childWant := d.historyWantBars(q.CandleLimit)
	parentNeed, err := exchange.ParentBarsNeeded(childWant, e.Name)
	if err != nil {
		return historyDeliverResult{Kind: historyDeliverNoData, Code: HistoryCodeNoData, Err: err}
	}
	parentRes := d.deliverHistoryWindow(ctx, HistoryWindowQuery{
		Spec:        parentSpec,
		EndTimeMs:   q.EndTimeMs,
		CandleLimit: parentNeed,
	})
	if parentRes.Kind != historyDeliverOK {
		return parentRes
	}
	closed, _, ferr := exchange.FoldClosedChildren(parentRes.Win.Klines, e.Name, false)
	if ferr != nil {
		return historyDeliverResult{Kind: historyDeliverNoData, Code: HistoryCodeNoData, Err: ferr}
	}
	capEnd := resolveClosedBarBoundary(q.EndTimeMs, e.Name)
	closed = filterKlinesUntilOpenMs(closed, capEnd)
	if childWant > 0 && len(closed) > childWant {
		closed = closed[len(closed)-childWant:]
	}
	if len(closed) == 0 {
		return historyUnavailableResult(parentRes.Win)
	}
	return historyDeliverResult{
		Kind: historyDeliverOK,
		Code: HistoryCodeOK,
		Win:  HistoryWindow{Klines: closed, HasMore: parentRes.Win.HasMore},
	}
}

func writeHistoryNoData(w http.ResponseWriter, timeframe string, columnar bool, code string) {
	if code == "" {
		code = HistoryCodeNoData
	}
	if columnar {
		writeJSON(w, columnarHistoryResponse{
			Format:    "columnar",
			Status:    "no_data",
			Code:      code,
			Timeframe: timeframe,
			Times:     []int64{},
			Candles:   columnarCandles{},
			Plots:     map[string][]float64{},
			HasMore:   false,
		})
		return
	}
	writeJSON(w, historyResponse{
		Status:    "no_data",
		Code:      code,
		Timeframe: timeframe,
		HasMore:   false,
	})
}

func writeHistoryFault(w http.ResponseWriter, httpStatus int, code string, err error) {
	msg := code
	if err != nil && err.Error() != "" {
		msg = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "error",
		"code":   code,
		"error":  msg,
	})
}
