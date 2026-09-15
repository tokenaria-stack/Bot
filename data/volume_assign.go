package data

import (
	"fmt"
	"strings"
)

// AssignCanonicalVolume sets historical_klines.volume for one exact PK.
// It does not touch OHLC. Used by VOLUME-TRUTH-RECOVERY-1 and
// VOLUME-SOURCE-ARBITRATION-1 exact-PK repair only.
func AssignCanonicalVolume(symbol, interval string, openTime int64, volume float64) error {
	if err := InitDB(); err != nil {
		return err
	}
	symbol = normalizeSymbol(symbol)
	interval = strings.TrimSpace(interval)
	if openTime <= 0 {
		return fmt.Errorf("assign volume: bad open_time")
	}
	res, err := db.Exec(`UPDATE historical_klines SET volume = ? WHERE symbol = ? AND interval = ? AND open_time = ?`,
		volume, symbol, interval, openTime)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("assign volume %s %s %d: rows=%d want 1", symbol, interval, openTime, n)
	}
	return nil
}
