package strategy

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
)

// StartSQLiteArchiveCatchUpLoop quietly heals SQLite tip lag independent of Analyst RAM.
// FetchClosedRange → PersistenceQueue only — never SaveKlines, never LoadHistoricalKlines (Shot 9E).
func (m *MasterGeneral) StartSQLiteArchiveCatchUpLoop(ctx context.Context) {
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

// CatchUpAllSQLiteArchiveTips walks known chart intervals and REST-fills SQLite tip windows only.
func (m *MasterGeneral) CatchUpAllSQLiteArchiveTips(ctx context.Context) {
	if m == nil || m.exchangeClient == nil {
		return
	}
	m.mu.RLock()
	symbol := m.symbol
	intervals := make([]string, 0, len(m.analysts))
	for interval := range m.analysts {
		intervals = append(intervals, interval)
	}
	m.mu.RUnlock()

	nowMs := time.Now().UnixMilli()
	for _, interval := range intervals {
		if intervalSkipsKlineGapFill(interval) {
			continue
		}
		if err := m.catchUpSQLiteArchiveTip(ctx, symbol, interval, nowMs); err != nil {
			log.Printf("[SQLiteArchive] catch-up %s %s: %v", symbol, interval, err)
		}
	}
}

func (m *MasterGeneral) catchUpSQLiteArchiveTip(ctx context.Context, symbol, interval string, nowMs int64) error {
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
