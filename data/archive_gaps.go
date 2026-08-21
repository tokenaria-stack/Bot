package data

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

const (
	// ArchiveGapStatusOpen is eligible for catch-up heal.
	ArchiveGapStatusOpen = "open"
	// ArchiveGapStatusExhausted is retained for diagnostics but excluded from heal queue.
	ArchiveGapStatusExhausted = "exhausted"
)

// ArchiveGap is a known discontinuity in historical_klines:
// bars exist at AfterOpenMs and BeforeOpenMs, but at least one expected
// intermediate open is missing. Tip freshness does not clear these.
type ArchiveGap struct {
	Symbol       string
	Interval     string
	AfterOpenMs  int64 // last present bar before the hole
	BeforeOpenMs int64 // first present bar after the hole
	Status       string
	Reason       string
}

// ensureArchiveGapsTableLocked creates the gap ledger (called under InitDB lock).
// Order is mandatory for existing DBs: CREATE (no-op if present) → ALTER ADD columns → indexes.
// Creating an index on status before ALTER fails with "no such column: status" and aborts InitDB.
func ensureArchiveGapsTableLocked() error {
	if db == nil {
		return fmt.Errorf("db not open")
	}
	// Base table. Existing DBs keep their old shape; new DBs get status/reason here.
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS archive_gaps (
    symbol TEXT NOT NULL,
    interval TEXT NOT NULL,
    after_open_ms INTEGER NOT NULL,
    before_open_ms INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    reason TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (symbol, interval, after_open_ms, before_open_ms)
)`); err != nil {
		return err
	}
	// Existing DBs created before status/reason — additive migrate (ignore duplicate column).
	for _, stmt := range []string{
		`ALTER TABLE archive_gaps ADD COLUMN status TEXT NOT NULL DEFAULT 'open'`,
		`ALTER TABLE archive_gaps ADD COLUMN reason TEXT NOT NULL DEFAULT ''`,
	} {
		if _, alterErr := db.Exec(stmt); alterErr != nil {
			msg := strings.ToLower(alterErr.Error())
			if !strings.Contains(msg, "duplicate column") {
				return alterErr
			}
		}
	}
	// Indexes only after columns exist on both fresh and migrated DBs.
	if _, err := db.Exec(`
CREATE INDEX IF NOT EXISTS idx_archive_gaps_lookup
    ON archive_gaps(symbol, interval, after_open_ms);
CREATE INDEX IF NOT EXISTS idx_archive_gaps_open
    ON archive_gaps(symbol, interval, status, after_open_ms);
`); err != nil {
		return err
	}
	return nil
}

// RecordArchiveGap upserts a known internal hole as status=open. No-op when edges are invalid.
// Does not reopen an exhausted row (ON CONFLICT DO NOTHING).
func RecordArchiveGap(symbol, interval string, afterOpenMs, beforeOpenMs int64) error {
	if err := InitDB(); err != nil {
		return err
	}
	symbol = normalizeSymbol(symbol)
	interval = trimInterval(interval)
	if symbol == "" || interval == "" || afterOpenMs <= 0 || beforeOpenMs <= afterOpenMs {
		return nil
	}
	next, err := NextBarOpen(afterOpenMs, interval)
	if err != nil {
		return err
	}
	if next >= beforeOpenMs {
		return nil // contiguous or overlapping — not a hole
	}
	_, err = db.Exec(`
INSERT INTO archive_gaps (symbol, interval, after_open_ms, before_open_ms, status, reason)
VALUES (?, ?, ?, ?, ?, '')
ON CONFLICT(symbol, interval, after_open_ms, before_open_ms) DO NOTHING`,
		symbol, interval, afterOpenMs, beforeOpenMs, ArchiveGapStatusOpen)
	return err
}

// ClearArchiveGap removes a ledger entry after a successful heal (or if edges collapsed).
func ClearArchiveGap(symbol, interval string, afterOpenMs, beforeOpenMs int64) error {
	if err := InitDB(); err != nil {
		return err
	}
	symbol = normalizeSymbol(symbol)
	interval = trimInterval(interval)
	_, err := db.Exec(`
DELETE FROM archive_gaps
WHERE symbol = ? AND interval = ? AND after_open_ms = ? AND before_open_ms = ?`,
		symbol, interval, afterOpenMs, beforeOpenMs)
	return err
}

// MarkArchiveGapExhausted keeps the row for diagnostics but excludes it from heal catch-up.
func MarkArchiveGapExhausted(symbol, interval string, afterOpenMs, beforeOpenMs int64, reason string) error {
	if err := InitDB(); err != nil {
		return err
	}
	symbol = normalizeSymbol(symbol)
	interval = trimInterval(interval)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "exhausted"
	}
	_, err := db.Exec(`
UPDATE archive_gaps
SET status = ?, reason = ?
WHERE symbol = ? AND interval = ? AND after_open_ms = ? AND before_open_ms = ?`,
		ArchiveGapStatusExhausted, reason, symbol, interval, afterOpenMs, beforeOpenMs)
	return err
}

// ListArchiveGaps returns up to limit open (healable) holes for a series (oldest first).
func ListArchiveGaps(symbol, interval string, limit int) ([]ArchiveGap, error) {
	if err := InitDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 8
	}
	symbol = normalizeSymbol(symbol)
	interval = trimInterval(interval)
	rows, err := db.Query(`
SELECT symbol, interval, after_open_ms, before_open_ms, status, reason
FROM archive_gaps
WHERE symbol = ? AND interval = ? AND status = ?
ORDER BY after_open_ms ASC
LIMIT ?`, symbol, interval, ArchiveGapStatusOpen, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ArchiveGap, 0, limit)
	for rows.Next() {
		var g ArchiveGap
		if err := rows.Scan(&g.Symbol, &g.Interval, &g.AfterOpenMs, &g.BeforeOpenMs, &g.Status, &g.Reason); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// FuturesClampHealWindow mirrors exchange FetchClosedRange genesis clamp without importing exchange.
// ok=false means the futures REST path cannot fetch anything useful → mark exhausted.
func FuturesClampHealWindow(startMs, endMs, genesisMs int64) (fetchStart, fetchEnd int64, ok bool, exhaustReason string) {
	if startMs <= 0 || endMs <= 0 || startMs > endMs {
		return 0, 0, false, "invalid_heal_window"
	}
	if genesisMs > 0 && endMs < genesisMs {
		return 0, 0, false, "pre_futures_genesis"
	}
	fetchStart, fetchEnd = startMs, endMs
	if genesisMs > 0 && fetchStart < genesisMs {
		fetchStart = genesisMs
	}
	if fetchStart > fetchEnd {
		return 0, 0, false, "empty_after_genesis_clamp"
	}
	return fetchStart, fetchEnd, true, ""
}

// NoteGapsFromOpenTimes records discontinuities inside an ascending open_time slice.
// O(n) on the slice only — never scans the full archive.
func NoteGapsFromOpenTimes(symbol, interval string, opens []int64) {
	if len(opens) < 2 {
		return
	}
	symbol = normalizeSymbol(symbol)
	interval = trimInterval(interval)
	for i := 1; i < len(opens); i++ {
		prev, cur := opens[i-1], opens[i]
		if cur <= prev {
			continue
		}
		next, err := NextBarOpen(prev, interval)
		if err != nil || next <= 0 {
			continue
		}
		if cur == next {
			continue
		}
		if err := RecordArchiveGap(symbol, interval, prev, cur); err != nil {
			log.Printf("[ArchiveGap] record %s %s [%d..%d]: %v", symbol, interval, prev, cur, err)
		}
	}
}

// NotePersistEdges checks the two indexed neighbors of a just-written batch and
// records a gap if the write did not connect to the prior/next archived bar.
// Two primary-key lookups — not a table scan.
func NotePersistEdges(symbol, interval string, candles []Candle) {
	if len(candles) == 0 {
		return
	}
	symbol = normalizeSymbol(symbol)
	interval = trimInterval(interval)
	minOT, maxOT := candles[0].OpenTime, candles[0].OpenTime
	for _, c := range candles[1:] {
		if c.OpenTime < minOT {
			minOT = c.OpenTime
		}
		if c.OpenTime > maxOT {
			maxOT = c.OpenTime
		}
	}
	if prev, ok, err := queryOpenTimeBefore(symbol, interval, minOT); err == nil && ok {
		next, nerr := NextBarOpen(prev, interval)
		if nerr == nil && next > 0 && next < minOT {
			_ = RecordArchiveGap(symbol, interval, prev, minOT)
		}
	}
	if next, ok, err := queryOpenTimeAfter(symbol, interval, maxOT); err == nil && ok {
		expect, nerr := NextBarOpen(maxOT, interval)
		if nerr == nil && expect > 0 && expect < next {
			_ = RecordArchiveGap(symbol, interval, maxOT, next)
		}
	}
	// Contiguous batch internals.
	opens := make([]int64, len(candles))
	for i, c := range candles {
		opens[i] = c.OpenTime
	}
	// Sort not required if PersistJob batches are usually ascending; still safe to note pairwise after sort.
	sortInt64Asc(opens)
	NoteGapsFromOpenTimes(symbol, interval, opens)
}

func queryOpenTimeBefore(symbol, interval string, openMs int64) (int64, bool, error) {
	if err := InitDB(); err != nil {
		return 0, false, err
	}
	var ot int64
	err := db.QueryRow(`
SELECT open_time FROM historical_klines
WHERE symbol = ? AND interval = ? AND open_time < ?
ORDER BY open_time DESC LIMIT 1`, symbol, interval, openMs).Scan(&ot)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return ot, true, nil
}

func queryOpenTimeAfter(symbol, interval string, openMs int64) (int64, bool, error) {
	if err := InitDB(); err != nil {
		return 0, false, err
	}
	var ot int64
	err := db.QueryRow(`
SELECT open_time FROM historical_klines
WHERE symbol = ? AND interval = ? AND open_time > ?
ORDER BY open_time ASC LIMIT 1`, symbol, interval, openMs).Scan(&ot)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return ot, true, nil
}

type gapPair struct {
	after  int64
	before int64
}

func neighborDiscontinuitiesTx(tx *sql.Tx, symbol, interval string, minOT, maxOT int64) ([]gapPair, error) {
	prev, hasPrev, err := queryOpenTimeBeforeTx(tx, symbol, interval, minOT)
	if err != nil {
		return nil, err
	}
	next, hasNext, err := queryOpenTimeAfterTx(tx, symbol, interval, maxOT)
	if err != nil {
		return nil, err
	}
	mid, err := queryOpenTimesInclusiveTx(tx, symbol, interval, minOT, maxOT)
	if err != nil {
		return nil, err
	}
	opens := make([]int64, 0, len(mid)+2)
	if hasPrev {
		opens = append(opens, prev)
	}
	opens = append(opens, mid...)
	if hasNext {
		opens = append(opens, next)
	}
	return discontinuitiesFromOpens(interval, opens)
}

func queryOpenTimeBeforeTx(tx *sql.Tx, symbol, interval string, openMs int64) (int64, bool, error) {
	var ot int64
	err := tx.QueryRow(`
SELECT open_time FROM historical_klines
WHERE symbol = ? AND interval = ? AND open_time < ?
ORDER BY open_time DESC LIMIT 1`, symbol, interval, openMs).Scan(&ot)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return ot, true, nil
}

func queryOpenTimeAfterTx(tx *sql.Tx, symbol, interval string, openMs int64) (int64, bool, error) {
	var ot int64
	err := tx.QueryRow(`
SELECT open_time FROM historical_klines
WHERE symbol = ? AND interval = ? AND open_time > ?
ORDER BY open_time ASC LIMIT 1`, symbol, interval, openMs).Scan(&ot)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return ot, true, nil
}

func queryOpenTimesInclusiveTx(tx *sql.Tx, symbol, interval string, minOT, maxOT int64) ([]int64, error) {
	rows, err := tx.Query(`
SELECT open_time FROM historical_klines
WHERE symbol = ? AND interval = ? AND open_time >= ? AND open_time <= ?
ORDER BY open_time ASC`, symbol, interval, minOT, maxOT)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var ot int64
		if err := rows.Scan(&ot); err != nil {
			return nil, err
		}
		out = append(out, ot)
	}
	return out, rows.Err()
}

func discontinuitiesFromOpens(interval string, opens []int64) ([]gapPair, error) {
	if len(opens) < 2 {
		return nil, nil
	}
	out := make([]gapPair, 0)
	prev := opens[0]
	for i := 1; i < len(opens); i++ {
		cur := opens[i]
		if cur <= prev {
			continue
		}
		next, err := NextBarOpen(prev, interval)
		if err != nil {
			return nil, err
		}
		if next > 0 && next < cur {
			out = append(out, gapPair{after: prev, before: cur})
		}
		prev = cur
	}
	return out, nil
}

func applyArchiveGapDiffTx(tx *sql.Tx, symbol, interval string, oldPairs, newPairs []gapPair) error {
	oldSet := make(map[gapPair]struct{}, len(oldPairs))
	for _, p := range oldPairs {
		oldSet[p] = struct{}{}
	}
	newSet := make(map[gapPair]struct{}, len(newPairs))
	for _, p := range newPairs {
		newSet[p] = struct{}{}
	}
	for p := range oldSet {
		if _, ok := newSet[p]; ok {
			continue
		}
		if _, err := tx.Exec(`
DELETE FROM archive_gaps
WHERE symbol = ? AND interval = ? AND after_open_ms = ? AND before_open_ms = ?`,
			symbol, interval, p.after, p.before); err != nil {
			return fmt.Errorf("delete stale archive_gaps: %w", err)
		}
	}
	for p := range newSet {
		if _, ok := oldSet[p]; ok {
			continue
		}
		if _, err := tx.Exec(`
INSERT INTO archive_gaps (symbol, interval, after_open_ms, before_open_ms, status, reason)
VALUES (?, ?, ?, ?, ?, '')
ON CONFLICT(symbol, interval, after_open_ms, before_open_ms) DO NOTHING`,
			symbol, interval, p.after, p.before, ArchiveGapStatusOpen); err != nil {
			return fmt.Errorf("insert archive_gaps: %w", err)
		}
	}
	return nil
}

func trimInterval(interval string) string {
	return strings.TrimSpace(interval)
}

func sortInt64Asc(a []int64) {
	// Tiny insertion sort — batches are small (≤256) and often nearly sorted.
	for i := 1; i < len(a); i++ {
		v := a[i]
		j := i
		for j > 0 && a[j-1] > v {
			a[j] = a[j-1]
			j--
		}
		a[j] = v
	}
}

// GapHealWindow returns the closed inclusive [start,end] open times to REST-fetch for a gap.
// ok=false when the ledger entry is no longer a hole.
func GapHealWindow(gap ArchiveGap) (startMs, endMs int64, ok bool, err error) {
	startMs, err = NextBarOpen(gap.AfterOpenMs, gap.Interval)
	if err != nil {
		return 0, 0, false, err
	}
	endMs, err = PreviousBarOpen(gap.BeforeOpenMs, gap.Interval)
	if err != nil {
		return 0, 0, false, err
	}
	if startMs <= 0 || endMs <= 0 || startMs > endMs {
		return 0, 0, false, nil
	}
	return startMs, endMs, true, nil
}

// ArchiveGapIsCurrentNeighbor reports whether (after, before) is still the current
// physical neighbor discontinuity for this storage series. One indexed lookup:
// next stored open after `after` must equal `before`, and they must not be
// calendar-adjacent (NextBarOpen).
//
// If no stored bar exists after `after`, the pair is not a current neighbor
// (stale OPEN). Healer clears it and does not REST — we do not treat a missing
// successor as a fetch-to-tip repair.
func ArchiveGapIsCurrentNeighbor(symbol, interval string, afterOpenMs, beforeOpenMs int64) (bool, error) {
	if err := InitDB(); err != nil {
		return false, err
	}
	symbol = normalizeSymbol(symbol)
	interval = trimInterval(interval)
	nextExisting, ok, err := queryOpenTimeAfter(symbol, interval, afterOpenMs)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if nextExisting != beforeOpenMs {
		return false, nil
	}
	expect, err := NextBarOpen(afterOpenMs, interval)
	if err != nil {
		return false, err
	}
	return expect > 0 && beforeOpenMs != expect, nil
}

// ArchiveGapStillOpen reports whether the ledger edges still have missing intermediates.
func ArchiveGapStillOpen(symbol, interval string, afterOpenMs, beforeOpenMs int64) (bool, error) {
	if err := InitDB(); err != nil {
		return false, err
	}
	symbol = normalizeSymbol(symbol)
	interval = trimInterval(interval)
	start, end, ok, err := GapHealWindow(ArchiveGap{
		Symbol: symbol, Interval: interval,
		AfterOpenMs: afterOpenMs, BeforeOpenMs: beforeOpenMs,
	})
	if err != nil || !ok {
		return false, err
	}
	var n int
	err = db.QueryRow(`
SELECT COUNT(*) FROM historical_klines
WHERE symbol = ? AND interval = ? AND open_time >= ? AND open_time <= ?`,
		symbol, interval, start, end).Scan(&n)
	if err != nil {
		return false, err
	}
	// Any missing bar in the heal window keeps the gap open. Cheap count vs expected
	// would need BarStepsBetween (capped); presence of zero rows in a multi-bar hole is enough
	// for the common case. For partial fills, compare neighbor continuity:
	if n == 0 {
		return true, nil
	}
	next, err := NextBarOpen(afterOpenMs, interval)
	if err != nil {
		return false, err
	}
	var first int64
	err = db.QueryRow(`
SELECT open_time FROM historical_klines
WHERE symbol = ? AND interval = ? AND open_time > ?
ORDER BY open_time ASC LIMIT 1`, symbol, interval, afterOpenMs).Scan(&first)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return first != next, nil
}
