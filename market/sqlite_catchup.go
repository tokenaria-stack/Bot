package market

import (
	"context"
	"log"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

const (
	sqliteArchiveCatchUpInterval  = 5 * time.Minute
	sqliteArchiveCatchUpMaxChunks = 8
	sqliteArchiveCatchUpPause     = 250 * time.Millisecond
	sqliteArchiveGapHealMax       = 4
)

// StartSQLiteArchiveCatchUpLoop quietly heals SQLite tip lag and known internal gaps.
// FetchClosedRange → PersistenceQueue only — never SaveKlines, never LoadHistoricalKlines (Shot 9E).
// Tip freshness alone does not imply archive continuity (H3).
func (m *Runtime) StartSQLiteArchiveCatchUpLoop(ctx context.Context) {
	if m == nil {
		return
	}
	go func() {
		m.CatchUpAllSQLiteArchiveTips(ctx)
		ticker := time.NewTicker(sqliteArchiveCatchUpInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.CatchUpAllSQLiteArchiveTips(ctx)
			}
		}
	}()
}

// CatchUpAllSQLiteArchiveTips walks known chart intervals: tip lag first, then ledger gaps.
func (m *Runtime) CatchUpAllSQLiteArchiveTips(ctx context.Context) {
	if m == nil || m.exchangeClient == nil {
		return
	}
	m.mu.RLock()
	symbol := m.symbol
	intervals := make([]string, 0, len(m.frames))
	for interval := range m.frames {
		intervals = append(intervals, interval)
	}
	m.mu.RUnlock()

	nowMs := time.Now().UnixMilli()
	for _, interval := range intervals {
		if err := m.catchUpSQLiteArchiveTip(ctx, symbol, interval, nowMs); err != nil {
			log.Printf("[SQLiteArchive] catch-up %s %s: %v", symbol, interval, err)
		}
		if err := m.healSQLiteArchiveGaps(ctx, symbol, interval); err != nil {
			log.Printf("[SQLiteArchive] gap-heal %s %s: %v", symbol, interval, err)
		}
	}
}

func (m *Runtime) catchUpSQLiteArchiveTip(ctx context.Context, symbol, interval string, nowMs int64) error {
	if m == nil || m.exchangeClient == nil {
		return nil
	}
	m.mu.RLock()
	q := m.persistQ
	m.mu.RUnlock()
	if q == nil {
		log.Printf("[SQLiteArchive] skip %s %s: PersistenceQueue not bound", symbol, interval)
		return nil
	}

	for chunk := 0; chunk < sqliteArchiveCatchUpMaxChunks; chunk++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		needs, tipOpenMs, err := data.SQLiteTipNeedsCatchUp(symbol, interval, nowMs)
		if err != nil {
			return err
		}
		if !needs {
			return nil
		}

		startMs, endMs, ok, err := data.SQLiteCatchUpWindow(tipOpenMs, nowMs, interval)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		candles, err := m.exchangeClient.FetchClosedRange(symbol, interval, startMs, endMs)
		if err != nil {
			return err
		}
		if len(candles) == 0 {
			return nil
		}
		if err := q.AppendClosedBars(ctx, symbol, interval, exchange.CandlesToData(candles)); err != nil {
			return err
		}

		if chunk+1 < sqliteArchiveCatchUpMaxChunks {
			time.Sleep(sqliteArchiveCatchUpPause)
		}
	}
	log.Printf("[SQLiteArchive] tip still behind %s %s after %d chunks (will retry next tick)",
		symbol, interval, sqliteArchiveCatchUpMaxChunks)
	return nil
}

// healSQLiteArchiveGaps REST-fills known internal holes from the archive_gaps ledger.
// Bounded work per tick — does not scan the full 1.4M-row table.
func (m *Runtime) healSQLiteArchiveGaps(ctx context.Context, symbol, interval string) error {
	if m == nil || m.exchangeClient == nil {
		return nil
	}
	m.mu.RLock()
	q := m.persistQ
	m.mu.RUnlock()
	if q == nil {
		return nil
	}

	gaps, err := data.ListArchiveGaps(symbol, interval, sqliteArchiveGapHealMax)
	if err != nil {
		return err
	}
	for _, gap := range gaps {
		if err := ctx.Err(); err != nil {
			return err
		}
		still, err := data.ArchiveGapStillOpen(gap.Symbol, gap.Interval, gap.AfterOpenMs, gap.BeforeOpenMs)
		if err != nil {
			return err
		}
		if !still {
			_ = data.ClearArchiveGap(gap.Symbol, gap.Interval, gap.AfterOpenMs, gap.BeforeOpenMs)
			continue
		}
		startMs, endMs, ok, err := data.GapHealWindow(gap)
		if err != nil {
			return err
		}
		if !ok {
			_ = data.ClearArchiveGap(gap.Symbol, gap.Interval, gap.AfterOpenMs, gap.BeforeOpenMs)
			continue
		}
		candles, err := m.exchangeClient.FetchClosedRangePagesExact(symbol, interval, startMs, endMs)
		if err != nil {
			return err
		}
		if len(candles) == 0 {
			log.Printf("[SQLiteArchive] gap-heal empty REST %s %s [%d..%d]", symbol, interval, startMs, endMs)
			continue
		}
		if err := q.AppendClosedBars(ctx, symbol, interval, exchange.CandlesToData(candles)); err != nil {
			return err
		}
		still, err = data.ArchiveGapStillOpen(gap.Symbol, gap.Interval, gap.AfterOpenMs, gap.BeforeOpenMs)
		if err != nil {
			return err
		}
		if !still {
			_ = data.ClearArchiveGap(gap.Symbol, gap.Interval, gap.AfterOpenMs, gap.BeforeOpenMs)
			log.Printf("[SQLiteArchive] gap healed %s %s after=%d before=%d bars=%d",
				symbol, interval, gap.AfterOpenMs, gap.BeforeOpenMs, len(candles))
		} else {
			log.Printf("[SQLiteArchive] gap partial %s %s after=%d before=%d wrote=%d (will retry)",
				symbol, interval, gap.AfterOpenMs, gap.BeforeOpenMs, len(candles))
		}
	}
	return nil
}
