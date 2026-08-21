package data

import "fmt"

// ArchiveGapCensusRow is one (symbol, interval, status) count.
type ArchiveGapCensusRow struct {
	Symbol   string
	Interval string
	Status   string
	N        int
}

// ReconcileOpenGapsResult is the outcome of one explicit OPEN-ledger drain.
type ReconcileOpenGapsResult struct {
	Examined int
	Deleted  int
	Retained int
}

const defaultReconcileOpenBatch = 500

// CensusArchiveGaps groups the ledger by storage symbol, interval, and status.
func CensusArchiveGaps() ([]ArchiveGapCensusRow, error) {
	if err := InitDB(); err != nil {
		return nil, err
	}
	rows, err := db.Query(`
SELECT symbol, interval, status, COUNT(*)
FROM archive_gaps
GROUP BY symbol, interval, status
ORDER BY symbol, interval, status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArchiveGapCensusRow
	for rows.Next() {
		var r ArchiveGapCensusRow
		if err := rows.Scan(&r.Symbol, &r.Interval, &r.Status, &r.N); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReconcileOpenArchiveGaps deletes OPEN rows that are not current physical
// neighbor discontinuities. Exhausted rows are never selected. Ops-only;
// not called from InitDB or the live healer.
func ReconcileOpenArchiveGaps(batchSize int) (ReconcileOpenGapsResult, error) {
	if err := InitDB(); err != nil {
		return ReconcileOpenGapsResult{}, err
	}
	if batchSize <= 0 {
		batchSize = defaultReconcileOpenBatch
	}
	var res ReconcileOpenGapsResult
	cur := archiveGapKey{}
	for {
		batch, err := listOpenArchiveGapsAfter(cur, batchSize)
		if err != nil {
			return res, err
		}
		if len(batch) == 0 {
			return res, nil
		}
		for _, g := range batch {
			res.Examined++
			ok, err := ArchiveGapIsCurrentNeighbor(g.Symbol, g.Interval, g.AfterOpenMs, g.BeforeOpenMs)
			if err != nil {
				return res, err
			}
			if ok {
				res.Retained++
			} else {
				if err := ClearArchiveGap(g.Symbol, g.Interval, g.AfterOpenMs, g.BeforeOpenMs); err != nil {
					return res, fmt.Errorf("clear stale %s %s [%d..%d]: %w",
						g.Symbol, g.Interval, g.AfterOpenMs, g.BeforeOpenMs, err)
				}
				res.Deleted++
			}
			cur = archiveGapKey{g.Symbol, g.Interval, g.AfterOpenMs, g.BeforeOpenMs}
		}
	}
}

// VerifyRemainingOpenArchiveGaps reports OPEN rows that fail the neighbor invariant.
// Read-only; used after an explicit drain, not as runtime GC.
func VerifyRemainingOpenArchiveGaps() ([]ArchiveGap, error) {
	if err := InitDB(); err != nil {
		return nil, err
	}
	var bad []ArchiveGap
	cur := archiveGapKey{}
	for {
		batch, err := listOpenArchiveGapsAfter(cur, defaultReconcileOpenBatch)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return bad, nil
		}
		for _, g := range batch {
			ok, err := ArchiveGapIsCurrentNeighbor(g.Symbol, g.Interval, g.AfterOpenMs, g.BeforeOpenMs)
			if err != nil {
				return nil, err
			}
			if !ok {
				bad = append(bad, g)
			}
			cur = archiveGapKey{g.Symbol, g.Interval, g.AfterOpenMs, g.BeforeOpenMs}
		}
	}
}

type archiveGapKey struct {
	symbol, interval string
	after, before    int64
}

func listOpenArchiveGapsAfter(cur archiveGapKey, limit int) ([]ArchiveGap, error) {
	rows, err := db.Query(`
SELECT symbol, interval, after_open_ms, before_open_ms, status, reason
FROM archive_gaps
WHERE status = ?
  AND (symbol, interval, after_open_ms, before_open_ms) > (?, ?, ?, ?)
ORDER BY symbol, interval, after_open_ms, before_open_ms
LIMIT ?`,
		ArchiveGapStatusOpen, cur.symbol, cur.interval, cur.after, cur.before, limit)
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
