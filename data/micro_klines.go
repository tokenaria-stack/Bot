package data

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// MicroKlineInterval is the only interval stored in micro_klines (MICRO-2A).
const MicroKlineInterval = "1s"

// MicroKlineRetention is the exclusive age cutoff for 1s rows (24h).
const MicroKlineRetention = 24 * time.Hour

// IsMicroKlineInterval reports the dedicated sparse 1s archive (not historical_klines).
func IsMicroKlineInterval(interval string) bool {
	return strings.TrimSpace(interval) == MicroKlineInterval
}

func ensureMicroKlinesTableLocked() error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS micro_klines (
    symbol TEXT NOT NULL,
    interval TEXT NOT NULL,
    open_time INTEGER NOT NULL,
    open REAL NOT NULL,
    high REAL NOT NULL,
    low REAL NOT NULL,
    close REAL NOT NULL,
    volume REAL NOT NULL,
    close_time INTEGER NOT NULL,
    PRIMARY KEY (symbol, interval, open_time)
);
CREATE INDEX IF NOT EXISTS idx_micro_klines_open_time ON micro_klines(open_time);
`)
	return err
}

// SaveMicroKlines UPSERTs closed 1s bars into micro_klines.
// Sparse holes are legal. No archive_gaps, no MAX/MIN merge, no REST.
func SaveMicroKlines(symbol, interval string, klines []Candle) error {
	if err := InitDB(); err != nil {
		return err
	}
	if len(klines) == 0 {
		return nil
	}
	symbol = normalizeSymbol(symbol)
	interval = strings.TrimSpace(interval)
	if !IsMicroKlineInterval(interval) {
		return fmt.Errorf("SaveMicroKlines: interval %s is not %s", interval, MicroKlineInterval)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin micro tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
INSERT INTO micro_klines
    (symbol, interval, open_time, open, high, low, close, volume, close_time)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(symbol, interval, open_time) DO UPDATE SET
    open=excluded.open,
    high=excluded.high,
    low=excluded.low,
    close=excluded.close,
    volume=excluded.volume,
    close_time=excluded.close_time`)
	if err != nil {
		return fmt.Errorf("prepare micro upsert: %w", err)
	}
	defer stmt.Close()

	for _, k := range klines {
		if _, err := stmt.Exec(
			symbol, interval, k.OpenTime,
			k.Open, k.High, k.Low, k.Close, k.Volume, k.CloseTime,
		); err != nil {
			return fmt.Errorf("upsert micro open_time=%d: %w", k.OpenTime, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit micro: %w", err)
	}
	return nil
}

// LoadMicroKlinesBeforeEnd returns at most limit bars with open_time <= endTimeMs, ascending.
func LoadMicroKlinesBeforeEnd(symbol, interval string, endTimeMs int64, limit int) ([]Candle, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("LoadMicroKlinesBeforeEnd: limit must be > 0")
	}
	if err := InitDB(); err != nil {
		return nil, err
	}
	symbol = normalizeSymbol(symbol)
	interval = strings.TrimSpace(interval)
	if endTimeMs <= 0 {
		return nil, fmt.Errorf("LoadMicroKlinesBeforeEnd: endTimeMs required")
	}

	rows, err := db.Query(`
SELECT open_time, open, high, low, close, volume, close_time
FROM (
	SELECT open_time, open, high, low, close, volume, close_time
	FROM micro_klines
	WHERE symbol = ? AND interval = ? AND open_time <= ?
	ORDER BY open_time DESC
	LIMIT ?
) sub
ORDER BY open_time ASC`,
		symbol, interval, endTimeMs, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query micro klines before end: %w", err)
	}
	defer rows.Close()
	return scanKlineRows(rows)
}

// LoadMicroKlinesAfterStart returns at most limit bars with open_time > startTimeMs, ascending.
func LoadMicroKlinesAfterStart(symbol, interval string, startTimeMs int64, limit int) ([]Candle, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("LoadMicroKlinesAfterStart: limit must be > 0")
	}
	if err := InitDB(); err != nil {
		return nil, err
	}
	symbol = normalizeSymbol(symbol)
	interval = strings.TrimSpace(interval)
	if startTimeMs <= 0 {
		return nil, fmt.Errorf("LoadMicroKlinesAfterStart: startTimeMs required")
	}

	rows, err := db.Query(`
SELECT open_time, open, high, low, close, volume, close_time
FROM micro_klines
WHERE symbol = ? AND interval = ? AND open_time > ?
ORDER BY open_time ASC
LIMIT ?`,
		symbol, interval, startTimeMs, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query micro klines after start: %w", err)
	}
	defer rows.Close()
	return scanKlineRows(rows)
}

// LoadLatestMicroKlines returns the newest limit 1s rows, ascending.
func LoadLatestMicroKlines(symbol, interval string, limit int) ([]Candle, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("LoadLatestMicroKlines: limit must be > 0")
	}
	end := time.Now().UnixMilli()
	if end <= 0 {
		end = 1
	}
	return LoadMicroKlinesBeforeEnd(symbol, interval, end, limit)
}

// HasOlderMicroKline is true when a row exists with open_time < beforeOpenMs.
func HasOlderMicroKline(symbol, interval string, beforeOpenMs int64) (bool, error) {
	if err := InitDB(); err != nil {
		return false, err
	}
	symbol = normalizeSymbol(symbol)
	interval = strings.TrimSpace(interval)
	var n int
	err := db.QueryRow(`
SELECT 1 FROM micro_klines
WHERE symbol = ? AND interval = ? AND open_time < ?
LIMIT 1`,
		symbol, interval, beforeOpenMs,
	).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("has older micro kline: %w", err)
	}
	return true, nil
}

// HasNewerMicroKline is true when a row exists with open_time > afterOpenMs.
func HasNewerMicroKline(symbol, interval string, afterOpenMs int64) (bool, error) {
	if err := InitDB(); err != nil {
		return false, err
	}
	symbol = normalizeSymbol(symbol)
	interval = strings.TrimSpace(interval)
	var n int
	err := db.QueryRow(`
SELECT 1 FROM micro_klines
WHERE symbol = ? AND interval = ? AND open_time > ?
LIMIT 1`,
		symbol, interval, afterOpenMs,
	).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("has newer micro kline: %w", err)
	}
	return true, nil
}

// PruneMicroKlinesBefore deletes micro_klines with open_time < cutoffMs.
func PruneMicroKlinesBefore(cutoffMs int64) (int64, error) {
	if err := InitDB(); err != nil {
		return 0, err
	}
	res, err := db.Exec(`DELETE FROM micro_klines WHERE open_time < ?`, cutoffMs)
	if err != nil {
		return 0, fmt.Errorf("prune micro_klines: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// CountMicroKlines returns row count for (symbol, interval) in micro_klines.
func CountMicroKlines(symbol, interval string) (int, error) {
	if err := InitDB(); err != nil {
		return 0, err
	}
	var n int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM micro_klines WHERE symbol = ? AND interval = ?`,
		normalizeSymbol(symbol), strings.TrimSpace(interval),
	).Scan(&n)
	return n, err
}

// CountHistoricalKlines returns row count for (symbol, interval) in historical_klines.
func CountHistoricalKlines(symbol, interval string) (int, error) {
	if err := InitDB(); err != nil {
		return 0, err
	}
	var n int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM historical_klines WHERE symbol = ? AND interval = ?`,
		normalizeSymbol(symbol), strings.TrimSpace(interval),
	).Scan(&n)
	return n, err
}
