package data

import (
	"database/sql"
	"fmt"
	"strings"
)

// VolumeKey is one (symbol, interval) series in historical_klines.
type VolumeKey struct {
	Symbol   string
	Interval string
	N        int
}

// OpenReadOnlyHistory opens history.db without write pragmas (VOLUME-INGEST-1 census).
func OpenReadOnlyHistory(path string) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultDBPath
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("readonly history %s: %w", path, err)
	}
	return db, nil
}

// ListHistoricalVolumeKeys lists persisted (symbol, interval) series.
func ListHistoricalVolumeKeys(db *sql.DB) ([]VolumeKey, error) {
	rows, err := db.Query(`SELECT symbol, interval, COUNT(*) FROM historical_klines GROUP BY 1, 2 ORDER BY 1, 2`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VolumeKey
	for rows.Next() {
		var k VolumeKey
		if err := rows.Scan(&k.Symbol, &k.Interval, &k.N); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// ScanHistoricalVolumes loads open_time, volume for one series (ordered).
func ScanHistoricalVolumes(db *sql.DB, symbol, interval string) ([]int64, []float64, error) {
	rows, err := db.Query(
		`SELECT open_time, volume FROM historical_klines WHERE symbol = ? AND interval = ? ORDER BY open_time`,
		symbol, interval)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var ot []int64
	var vol []float64
	for rows.Next() {
		var t int64
		var v float64
		if err := rows.Scan(&t, &v); err != nil {
			return nil, nil, err
		}
		ot = append(ot, t)
		vol = append(vol, v)
	}
	return ot, vol, rows.Err()
}

// StoredOHLCV is one historical_klines row used by VOLUME-TRUTH-RECOVERY-1.
type StoredOHLCV struct {
	OpenTime int64
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

// ScanHistoricalOHLCV loads OHLCV for one series (ordered).
func ScanHistoricalOHLCV(db *sql.DB, symbol, interval string) ([]StoredOHLCV, error) {
	rows, err := db.Query(
		`SELECT open_time, open, high, low, close, volume FROM historical_klines WHERE symbol = ? AND interval = ? ORDER BY open_time`,
		symbol, interval)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoredOHLCV
	for rows.Next() {
		var r StoredOHLCV
		if err := rows.Scan(&r.OpenTime, &r.Open, &r.High, &r.Low, &r.Close, &r.Volume); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MicroVolumeStats is a 1s persistence summary (no aggTrade reconstruct).
type MicroVolumeStats struct {
	N     int
	MinOT int64
	MaxOT int64
}

// ScanMicroVolumeStats summarizes micro_klines without claiming Binance v parity.
func ScanMicroVolumeStats(db *sql.DB) (MicroVolumeStats, error) {
	var z MicroVolumeStats
	err := db.QueryRow(`SELECT COUNT(*), COALESCE(MIN(open_time),0), COALESCE(MAX(open_time),0) FROM micro_klines`).
		Scan(&z.N, &z.MinOT, &z.MaxOT)
	return z, err
}
