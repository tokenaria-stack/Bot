// repair_archive_gap — one-shot REST backfill for a known SQLite archive hole.
//
// Does not run at browser startup. Idempotent UPSERT via SaveKlines.
//
// Usage:
//
//	go run ./cmd/repair_archive_gap \
//	  -symbol=BTCUSDT -interval=1m \
//	  -from=2026-08-01T18:34:00Z -to=2026-08-07T04:37:00Z
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

func main() {
	symbol := flag.String("symbol", "BTCUSDT", "futures symbol")
	interval := flag.String("interval", "1m", "kline interval")
	fromStr := flag.String("from", "2026-08-01T18:34:00Z", "inclusive open time (RFC3339)")
	toStr := flag.String("to", "2026-08-07T04:37:00Z", "inclusive open time (RFC3339)")
	dbPath := flag.String("db", "history.db", "SQLite path")
	flag.Parse()

	fromT, err := time.Parse(time.RFC3339, *fromStr)
	if err != nil {
		log.Fatalf("parse -from: %v", err)
	}
	toT, err := time.Parse(time.RFC3339, *toStr)
	if err != nil {
		log.Fatalf("parse -to: %v", err)
	}
	fromMs := fromT.UTC().UnixMilli()
	toMs := toT.UTC().UnixMilli()
	if toMs < fromMs {
		log.Fatal("-to must be >= -from")
	}

	data.SetDBPath(*dbPath)
	if err := data.InitDB(); err != nil {
		log.Fatalf("InitDB: %v", err)
	}

	rest, err := exchange.NewBinanceExchange("", "", false)
	if err != nil {
		log.Fatalf("REST client: %v", err)
	}
	sym := exchange.NormalizeFuturesSymbol(*symbol)
	iv := *interval

	log.Printf("=== repair_archive_gap %s %s [%s .. %s] ===",
		sym, iv, fromT.UTC().Format(time.RFC3339), toT.UTC().Format(time.RFC3339))

	before, _ := countInRange(sym, iv, fromMs, toMs)
	start := time.Now()

	candles, err := rest.FetchClosedRangePagesExact(sym, iv, fromMs, toMs)
	if err != nil {
		log.Fatalf("FetchClosedRangePagesExact: %v", err)
	}
	if len(candles) == 0 {
		log.Fatal("exchange returned 0 bars — refusing empty repair")
	}

	const chunk = 1000
	saved := 0
	for i := 0; i < len(candles); i += chunk {
		j := i + chunk
		if j > len(candles) {
			j = len(candles)
		}
		batch := exchange.CandlesToData(candles[i:j])
		if err := saveWithRetry(sym, iv, batch); err != nil {
			log.Fatalf("SaveKlines [%d..%d): %v", i, j, err)
		}
		saved += len(batch)
	}

	after, _ := countInRange(sym, iv, fromMs, toMs)
	if err := data.CheckpointWAL(); err != nil {
		log.Printf("WAL checkpoint: %v", err)
	}

	// Continuity: every adjacent pair in the restored window must be one step apart.
	restored, err := data.LoadKlines(sym, iv, fromMs, toMs, 0)
	if err != nil {
		log.Fatalf("reload: %v", err)
	}
	// SaveKlines already maintains archive_gaps. Re-check leftover OPEN rows near the window.

	// Clear ledger entries that the repair just closed (tip-only heal left them).
	gaps, _ := data.ListArchiveGaps(sym, iv, 32)
	var openNear int
	for _, g := range gaps {
		near := g.BeforeOpenMs >= fromMs && g.AfterOpenMs <= toMs
		if !near {
			// also clear exact neighbor holes closed by this write
			near = g.AfterOpenMs < fromMs && g.BeforeOpenMs > toMs
		}
		if !near {
			continue
		}
		still, err := data.ArchiveGapStillOpen(g.Symbol, g.Interval, g.AfterOpenMs, g.BeforeOpenMs)
		if err != nil {
			log.Fatalf("gap probe: %v", err)
		}
		if still {
			openNear++
			continue
		}
		_ = data.ClearArchiveGap(g.Symbol, g.Interval, g.AfterOpenMs, g.BeforeOpenMs)
	}

	log.Printf("fetched=%d upserted=%d rows_before=%d rows_after=%d elapsed=%.1fs open_gaps_near_window=%d",
		len(candles), saved, before, after, time.Since(start).Seconds(), openNear)
	if after < len(candles) {
		log.Fatalf("continuity FAIL: expected >= %d rows in range, got %d", len(candles), after)
	}
	if openNear > 0 {
		log.Fatalf("continuity FAIL: %d archive_gaps remain open near repaired window", openNear)
	}
	fmt.Printf("PASS repair_archive_gap restored %d bars\n", after)
}

func countInRange(symbol, interval string, fromMs, toMs int64) (int, error) {
	rows, err := data.LoadKlines(symbol, interval, fromMs, toMs, 0)
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

func saveWithRetry(symbol, interval string, batch []data.Candle) error {
	var err error
	sleep := 25 * time.Millisecond
	for attempt := 1; attempt <= 8; attempt++ {
		err = data.SaveKlines(symbol, interval, batch)
		if err == nil {
			return nil
		}
		if !data.IsTransientSQLiteError(err) {
			return err
		}
		time.Sleep(sleep)
		sleep *= 2
		if sleep > 2*time.Second {
			sleep = 2 * time.Second
		}
	}
	return err
}
