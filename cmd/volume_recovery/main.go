package main

import (
	"fmt"
	"log"
	"os"

	"trading_bot/market"
)

func main() {
	dbPath := "history.db"
	if v := os.Getenv("VOLUME_RECOVERY_DB"); v != "" {
		dbPath = v
	}
	outDir := "research/volume"
	if v := os.Getenv("VOLUME_RECOVERY_OUT"); v != "" {
		outDir = v
	}
	apply := os.Getenv("VOLUME_RECOVERY_APPLY") == "1"
	live := os.Getenv("VOLUME_RECOVERY_LIVE") != "0"
	rep, err := market.RunVolumeTruthRecovery(dbPath, outDir, apply, live)
	if err != nil {
		log.Fatal(err)
	}
	os.Stdout.WriteString(market.FormatVolumeTruthRecovery(rep))
	switch rep.Verdict {
	case "VOLUME_TRUTH_GREEN_RESTORED", "VOLUME_TRUTH_GREEN_WITH_QUARANTINED_OUT_OF_SCOPE_GAPS", "VOLUME_TRUTH_REPAIR_READY_NEEDS_USER_STOP":
		if rep.Verdict == "VOLUME_TRUTH_REPAIR_READY_NEEDS_USER_STOP" {
			os.Exit(3)
		}
	default:
		fmt.Fprintln(os.Stderr, "recovery not green:", rep.Verdict)
		os.Exit(2)
	}
}
