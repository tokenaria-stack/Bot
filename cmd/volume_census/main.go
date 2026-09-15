package main

import (
	"log"
	"os"

	"trading_bot/market"
)

func main() {
	dbPath := "history.db"
	if v := os.Getenv("VOLUME_INGEST_DB"); v != "" {
		dbPath = v
	}
	outDir := "research/volume"
	if v := os.Getenv("VOLUME_INGEST_OUT"); v != "" {
		outDir = v
	}
	rep, err := market.RunVolumeIngest1(dbPath, outDir)
	if err != nil {
		log.Fatal(err)
	}
	os.Stdout.WriteString(market.FormatVolumeIngest1(rep))
	if !stringsHasPrefixGreen(rep.Verdict) {
		os.Exit(2)
	}
}

func stringsHasPrefixGreen(v string) bool {
	return len(v) >= 18 && (v == "VOLUME_TRUTH_GREEN_NO_REPAIR" || v == "VOLUME_TRUTH_GREEN_REPAIRED")
}
