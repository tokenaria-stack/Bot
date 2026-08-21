// reconcile_archive_gaps — explicit one-shot drain of stale OPEN archive_gaps rows.
//
// Does not run at bot startup. No REST. Exhausted diagnostics are left untouched.
//
// Usage:
//
//	go run ./cmd/reconcile_archive_gaps
//	go run ./cmd/reconcile_archive_gaps -db=history.db -batch=500
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"trading_bot/data"
)

func main() {
	dbPath := flag.String("db", "history.db", "SQLite database file path")
	batch := flag.Int("batch", 500, "OPEN rows per pagination batch")
	flag.Parse()

	data.SetDBPath(*dbPath)
	if err := data.InitDB(); err != nil {
		log.Fatalf("init database: %v", err)
	}

	before, err := data.CensusArchiveGaps()
	if err != nil {
		log.Fatalf("census before: %v", err)
	}
	fmt.Println("=== archive_gaps census BEFORE ===")
	printCensus(before)

	log.Printf("reconciling OPEN rows (batch=%d); exhausted untouched", *batch)
	res, err := data.ReconcileOpenArchiveGaps(*batch)
	if err != nil {
		log.Fatalf("reconcile: %v", err)
	}
	fmt.Printf("examined=%d deleted_stale=%d retained_real_open=%d\n",
		res.Examined, res.Deleted, res.Retained)

	after, err := data.CensusArchiveGaps()
	if err != nil {
		log.Fatalf("census after: %v", err)
	}
	fmt.Println("=== archive_gaps census AFTER ===")
	printCensus(after)

	bad, err := data.VerifyRemainingOpenArchiveGaps()
	if err != nil {
		log.Fatalf("verify: %v", err)
	}
	if len(bad) > 0 {
		fmt.Fprintf(os.Stderr, "VERIFY FAIL: %d remaining OPEN rows are not current neighbors\n", len(bad))
		for i, g := range bad {
			if i >= 8 {
				break
			}
			fmt.Fprintf(os.Stderr, "  %s %s after=%d before=%d\n", g.Symbol, g.Interval, g.AfterOpenMs, g.BeforeOpenMs)
		}
		os.Exit(1)
	}
	fmt.Println("VERIFY: every remaining OPEN row is a current physical neighbor discontinuity")
}

func printCensus(rows []data.ArchiveGapCensusRow) {
	if len(rows) == 0 {
		fmt.Println("(empty)")
		return
	}
	fmt.Printf("%-16s %-8s %-12s %s\n", "symbol", "tf", "status", "n")
	for _, r := range rows {
		fmt.Printf("%-16s %-8s %-12s %d\n", r.Symbol, r.Interval, r.Status, r.N)
	}
}
