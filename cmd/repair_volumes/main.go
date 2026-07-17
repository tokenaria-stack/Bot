// repair_volumes — one-shot healer for volume drift in history.db (Core 5.0 Phase E).
//
// Before Phase B the archive could store under-indexed REST snapshots (e.g. volume
// 21.257 instead of the final 48.470). This tool re-feeds clean, long-settled bars
// from Binance REST into SaveKlines; the monotonic UPSERT (volume=MAX, high=MAX,
// low=MIN) lifts stuck values and can never make a row worse. No diff logic needed.
//
// Usage: go run ./cmd/repair_volumes [-symbol BTCUSDT] [-days 7]
package main

import (
	"flag"
	"log"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

var repairTimeframes = []string{"1m", "3m", "5m", "15m"}

func main() {
	symbol := flag.String("symbol", "BTCUSDT", "futures symbol to repair")
	days := flag.Int("days", 7, "how many days back to re-fetch")
	flag.Parse()

	log.Printf("=== repair_volumes: %s, last %d day(s), TFs %v ===", *symbol, *days, repairTimeframes)

	if err := data.InitDB(); err != nil {
		log.Fatalf("[Repair] InitDB: %v", err)
	}

	// Public klines endpoint — no API keys required.
	rest, err := exchange.NewBinanceExchange("", "", false)
	if err != nil {
		log.Fatalf("[Repair] REST client: %v", err)
	}

	sym := exchange.NormalizeFuturesSymbol(*symbol)
	toMs := time.Now().UnixMilli()
	fromMs := toMs - int64(*days)*24*60*60*1000

	start := time.Now()
	totalSaved := 0
	for _, tf := range repairTimeframes {
		tfStart := time.Now()
		candles, err := rest.FetchClosedRangePages(sym, tf, fromMs, toMs)
		if err != nil {
			log.Printf("[Repair] %s %s: fetch failed: %v — skipping TF", sym, tf, err)
			continue
		}
		if len(candles) == 0 {
			log.Printf("[Repair] %s %s: exchange returned 0 bars — nothing to heal", sym, tf)
			continue
		}
		if err := data.SaveKlines(sym, tf, exchange.CandlesToData(candles)); err != nil {
			log.Printf("[Repair] %s %s: SaveKlines failed: %v", sym, tf, err)
			continue
		}
		totalSaved += len(candles)
		log.Printf("[Repair] %s %s: fetched %d bars [%s .. %s], upserted in %.1fs",
			sym, tf, len(candles),
			time.UnixMilli(candles[0].OpenTime).UTC().Format("2006-01-02 15:04"),
			time.UnixMilli(candles[len(candles)-1].OpenTime).UTC().Format("2006-01-02 15:04"),
			time.Since(tfStart).Seconds())
	}

	if err := data.CheckpointWAL(); err != nil {
		log.Printf("[Repair] WAL checkpoint: %v", err)
	}
	log.Printf("=== repair_volumes done: %d bars upserted across %d TFs in %.1fs ===",
		totalSaved, len(repairTimeframes), time.Since(start).Seconds())
}
