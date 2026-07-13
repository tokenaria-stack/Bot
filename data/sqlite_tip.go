package data

import "fmt"

// SQLite tip lag threshold: archive is "behind" if MAX(open_time) is more than
// this many closed intervals older than CapKlineEndToLastClosed(now).
const SQLiteTipLagBars = 2

// SQLiteCatchUpMaxBars caps a single REST catch-up window (Binance limit).
const SQLiteCatchUpMaxBars = 1000

// SQLiteTipNeedsCatchUp reports whether historical_klines tip for (symbol, interval)
// lags wall-clock last-closed by more than SQLiteTipLagBars.
//
// tipOpenMs is MAX(open_time) when the series has data, or 0 when the archive is empty.
// This check is SQLite-only — it never consults Analyst RAM.
func SQLiteTipNeedsCatchUp(symbol, interval string, currentMs int64) (needs bool, tipOpenMs int64, err error) {
	if symbol == "" || interval == "" {
		return false, 0, fmt.Errorf("SQLiteTipNeedsCatchUp: empty symbol/interval")
	}
	intervalMs, err := IntervalDurationMs(interval)
	if err != nil {
		return false, 0, err
	}
	endMs, err := CapKlineEndToLastClosed(currentMs, interval)
	if err != nil {
		return false, 0, err
	}
	tipOpenMs, hasTip, err := QueryKlineTipMaxOpenTime(symbol, interval)
	if err != nil {
		return false, 0, err
	}
	if !hasTip {
		return true, 0, nil
	}
	lagThreshold := int64(SQLiteTipLagBars) * intervalMs
	if endMs-tipOpenMs > lagThreshold {
		return true, tipOpenMs, nil
	}
	return false, tipOpenMs, nil
}

// SQLiteCatchUpWindow returns [startMs, endMs] for one limited REST catch-up chunk.
// tipOpenMs==0 means empty archive → seed the last SQLiteCatchUpMaxBars closed bars.
// Large holes are filled progressively (caller may loop with a pause).
func SQLiteCatchUpWindow(tipOpenMs, currentMs int64, interval string) (startMs, endMs int64, ok bool, err error) {
	intervalMs, err := IntervalDurationMs(interval)
	if err != nil {
		return 0, 0, false, err
	}
	endMs, err = CapKlineEndToLastClosed(currentMs, interval)
	if err != nil {
		return 0, 0, false, err
	}
	if tipOpenMs <= 0 {
		startMs = endMs - int64(SQLiteCatchUpMaxBars-1)*intervalMs
		if startMs < 0 {
			startMs = 0
		}
		return startMs, endMs, startMs <= endMs, nil
	}
	startMs = tipOpenMs + intervalMs
	if startMs > endMs {
		return 0, 0, false, nil
	}
	maxSpan := int64(SQLiteCatchUpMaxBars-1) * intervalMs
	if endMs-startMs > maxSpan {
		endMs = startMs + maxSpan
	}
	return startMs, endMs, true, nil
}
