package data

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

const defaultDBPath = "history.db"

// Candle is a persisted OHLCV bar (times in milliseconds).
type Candle struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

var (
	mu     sync.Mutex
	db     *sql.DB
	dbErr  error
	dbPath = defaultDBPath
)

// SetDBPath overrides the SQLite file path (intended for tests). Must be called before InitDB.
func SetDBPath(path string) {
	mu.Lock()
	defer mu.Unlock()
	dbPath = path
}

func resetDBConnection(path string) {
	mu.Lock()
	defer mu.Unlock()
	if db != nil {
		_ = db.Close()
	}
	db = nil
	dbErr = nil
	dbPath = path
}

// ResetDBForTest closes any open DB handle and points SQLite at path (tests only).
func ResetDBForTest(path string) {
	resetDBConnection(path)
}

// InitDB opens history.db and ensures schema exists.
func InitDB() error {
	mu.Lock()
	defer mu.Unlock()

	if db != nil {
		return dbErr
	}

	db, dbErr = sql.Open("sqlite", dbPath)
	if dbErr != nil {
		return dbErr
	}
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA synchronous=NORMAL`,
	} {
		if _, dbErr = db.Exec(pragma); dbErr != nil {
			return fmt.Errorf("%s: %w", pragma, dbErr)
		}
	}

	_, dbErr = db.Exec(`
CREATE TABLE IF NOT EXISTS historical_klines (
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
CREATE INDEX IF NOT EXISTS idx_klines_lookup ON historical_klines(symbol, interval, open_time);
CREATE INDEX IF NOT EXISTS idx_klines_time ON historical_klines(symbol, interval, open_time, close_time);
`)
	if dbErr != nil {
		return dbErr
	}

	logDBStatsLocked()
	purgeLegacySecondTimestampsLocked()
	return nil
}

// purgeLegacySecondTimestampsLocked removes rows stored in seconds (10-digit timestamps).
// Binance klines use milliseconds; mixed data causes permanent cache misses.
func purgeLegacySecondTimestampsLocked() {
	if db == nil {
		return
	}
	var legacy int
	if err := db.QueryRow(`SELECT COUNT(*) FROM historical_klines WHERE open_time > 0 AND open_time < 1000000000000`).Scan(&legacy); err != nil {
		log.Printf("[DEBUG] legacy timestamp scan failed: %v", err)
		return
	}
	if legacy == 0 {
		return
	}
	res, err := db.Exec(`DELETE FROM historical_klines WHERE open_time > 0 AND open_time < 1000000000000`)
	if err != nil {
		log.Printf("[DEBUG] failed to purge legacy second timestamps: %v", err)
		return
	}
	n, _ := res.RowsAffected()
	log.Printf("[DEBUG] purged %d klines with second-based open_time (expected ms); delete history.db manually if issues persist", n)
}

func logDBStatsLocked() {
	if db == nil {
		return
	}

	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		absPath = dbPath
	}

	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM historical_klines`).Scan(&total); err != nil {
		log.Printf("[DEBUG] history DB stats failed (path=%s): %v", absPath, err)
		return
	}

	log.Printf("[DEBUG] history DB path=%s total_klines=%d", absPath, total)

	var minT, maxT sql.NullInt64
	if err := db.QueryRow(`SELECT MIN(open_time), MAX(open_time) FROM historical_klines`).Scan(&minT, &maxT); err != nil {
		log.Printf("[DEBUG] history DB open_time range query failed: %v", err)
		return
	}
	if !minT.Valid {
		log.Printf("[DEBUG] historical_klines table is empty")
		return
	}

	log.Printf(
		"[DEBUG] stored open_time range: min=%d (%s) max=%d (%s)",
		minT.Int64, describeTimeUnit(minT.Int64),
		maxT.Int64, describeTimeUnit(maxT.Int64),
	)
}

func describeTimeUnit(ts int64) string {
	switch {
	case ts >= 1_000_000_000_000:
		return "milliseconds"
	case ts >= 1_000_000_000:
		return "SECONDS — cache will miss against ms queries"
	default:
		return "unexpected scale"
	}
}

func warnIfTimeLooksLikeSeconds(label string, ts int64) {
	if ts > 0 && ts < 1_000_000_000_000 {
		log.Printf("[DEBUG] WARNING: %s=%d looks like seconds (10 digits); expected ms (13 digits)", label, ts)
	}
}

func ensureUnixMillis(ts int64) int64 {
	if ts > 0 && ts < 1_000_000_000_000 {
		log.Printf("[DEBUG] coercing timestamp %d from seconds to milliseconds on save", ts)
		return ts * 1000
	}
	return ts
}

// SaveKlines inserts candles in a single transaction (INSERT OR IGNORE on PK).
// open_time and close_time are stored as Unix milliseconds (Binance native format).
func SaveKlines(symbol, interval string, klines []Candle) error {
	if err := InitDB(); err != nil {
		return err
	}
	if len(klines) == 0 {
		return nil
	}

	symbol = normalizeSymbol(symbol)
	interval = strings.TrimSpace(interval)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
INSERT OR IGNORE INTO historical_klines
    (symbol, interval, open_time, open, high, low, close, volume, close_time)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, k := range klines {
		openTime := ensureUnixMillis(k.OpenTime)
		closeTime := ensureUnixMillis(k.CloseTime)
		if _, err := stmt.Exec(
			symbol, interval, openTime,
			k.Open, k.High, k.Low, k.Close, k.Volume, closeTime,
		); err != nil {
			return fmt.Errorf("insert kline open_time=%d: %w", openTime, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// LoadKlines returns cached candles for [startTime, endTime] inclusive (Unix ms).
func LoadKlines(symbol, interval string, startTime, endTime int64) ([]Candle, error) {
	if err := InitDB(); err != nil {
		return nil, err
	}

	symbol = normalizeSymbol(symbol)
	interval = strings.TrimSpace(interval)

	startTime = ensureUnixMillis(startTime)
	endTime = ensureUnixMillis(endTime)

	fmt.Printf("Querying: %d to %d\n", startTime, endTime)
	log.Printf(
		"[DEBUG] Executing: SELECT * FROM historical_klines WHERE symbol='%s' AND interval='%s' AND open_time >= %d AND open_time <= %d",
		symbol, interval, startTime, endTime,
	)
	warnIfTimeLooksLikeSeconds("startTime", startTime)
	warnIfTimeLooksLikeSeconds("endTime", endTime)

	rows, err := db.Query(`
SELECT open_time, open, high, low, close, volume, close_time
FROM historical_klines
WHERE symbol = ? AND interval = ? AND open_time >= ? AND open_time <= ?
ORDER BY open_time ASC`,
		symbol, interval, startTime, endTime,
	)
	if err != nil {
		return nil, fmt.Errorf("query klines: %w", err)
	}
	defer rows.Close()

	out := make([]Candle, 0, 1024)
	for rows.Next() {
		var c Candle
		if err := rows.Scan(&c.OpenTime, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume, &c.CloseTime); err != nil {
			return nil, fmt.Errorf("scan kline: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate klines: %w", err)
	}

	log.Printf("[DEBUG] LoadKlines returned %d rows for %s %s", len(out), symbol, interval)
	if len(out) > 0 {
		first := out[0]
		last := out[len(out)-1]
		log.Printf(
			"[DEBUG] result open_time range: first=%d (%s) last=%d (%s)",
			first.OpenTime, describeTimeUnit(first.OpenTime),
			last.OpenTime, describeTimeUnit(last.OpenTime),
		)
	}

	return out, nil
}

// KlineCacheBounds describes stored open_time range for a symbol/interval pair.
type KlineCacheBounds struct {
	Count   int
	MinTime int64
	MaxTime int64
	HasData bool
}

// QueryKlineCacheBounds returns row count and min/max open_time (ms) in SQLite.
func QueryKlineCacheBounds(symbol, interval string) (KlineCacheBounds, error) {
	if err := InitDB(); err != nil {
		return KlineCacheBounds{}, err
	}

	symbol = normalizeSymbol(symbol)
	interval = strings.TrimSpace(interval)

	var bounds KlineCacheBounds
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM historical_klines WHERE symbol = ? AND interval = ?`,
		symbol, interval,
	).Scan(&bounds.Count); err != nil {
		return KlineCacheBounds{}, fmt.Errorf("count klines: %w", err)
	}
	if bounds.Count == 0 {
		return bounds, nil
	}

	var minT, maxT sql.NullInt64
	if err := db.QueryRow(
		`SELECT MIN(open_time), MAX(open_time) FROM historical_klines WHERE symbol = ? AND interval = ?`,
		symbol, interval,
	).Scan(&minT, &maxT); err != nil {
		return KlineCacheBounds{}, fmt.Errorf("min/max klines: %w", err)
	}
	if minT.Valid {
		bounds.MinTime = minT.Int64
	}
	if maxT.Valid {
		bounds.MaxTime = maxT.Int64
	}
	bounds.HasData = bounds.Count > 0 && minT.Valid && maxT.Valid
	return bounds, nil
}

// ExpectedKlineCount estimates how many bars fit in [startTime, endTime] for an interval.
func ExpectedKlineCount(interval string, startTime, endTime int64) (int, error) {
	if endTime <= startTime {
		return 0, fmt.Errorf("invalid range")
	}
	step, err := intervalDurationMs(interval)
	if err != nil {
		return 0, err
	}
	span := endTime - startTime
	count := span / step
	if count < 1 {
		count = 1
	}
	return int(count), nil
}

// IntervalDurationMs returns candle duration in milliseconds for a Binance interval string.
func IntervalDurationMs(interval string) (int64, error) {
	return intervalDurationMs(interval)
}

func intervalDurationMs(interval string) (int64, error) {
	switch strings.TrimSpace(interval) {
	case "1m":
		return 60_000, nil
	case "2m":
		return 2 * 60_000, nil
	case "3m":
		return 3 * 60_000, nil
	case "5m":
		return 5 * 60_000, nil
	case "15m":
		return 15 * 60_000, nil
	case "30m":
		return 30 * 60_000, nil
	case "1h":
		return 60 * 60_000, nil
	case "2h":
		return 2 * 60 * 60_000, nil
	case "4h":
		return 4 * 60 * 60_000, nil
	case "6h":
		return 6 * 60 * 60_000, nil
	case "8h":
		return 8 * 60 * 60_000, nil
	case "12h":
		return 12 * 60 * 60_000, nil
	case "1d":
		return 24 * 60 * 60_000, nil
	case "3d":
		return 3 * 24 * 60 * 60_000, nil
	case "1w":
		return 7 * 24 * 60 * 60_000, nil
	case "1M":
		return 30 * 24 * 60 * 60_000, nil
	default:
		return 0, fmt.Errorf("unsupported interval %q", interval)
	}
}

func normalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}
