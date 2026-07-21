package data

import "fmt"

// SQLite tip lag threshold: archive is "behind" if tip is more than this many
// closed bars older than CapKlineEndToLastClosed(now) (boundary steps, not duration).
const SQLiteTipLagBars = 2

// SQLiteCatchUpMaxBars caps a single REST catch-up window (Binance limit).
const SQLiteCatchUpMaxBars = 1000

// SQLiteTipNeedsCatchUp reports whether historical_klines tip for (symbol, interval)
// lags wall-clock last-closed by more than SQLiteTipLagBars boundary steps.
//
// tipOpenMs is MAX(open_time) when the series has data, or 0 when the archive is empty.
// This check is SQLite-only — it never consults Frame RAM.
func SQLiteTipNeedsCatchUp(symbol, interval string, currentMs int64) (needs bool, tipOpenMs int64, err error) {
	if symbol == "" || interval == "" {
		return false, 0, fmt.Errorf("SQLiteTipNeedsCatchUp: empty symbol/interval")
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
	steps, err := BarStepsBetween(tipOpenMs, endMs, interval)
	if err != nil {
		return false, tipOpenMs, err
	}
	if steps > SQLiteTipLagBars {
		return true, tipOpenMs, nil
	}
	return false, tipOpenMs, nil
}

// SQLiteCatchUpWindow returns [startMs, endMs] for one limited REST catch-up chunk.
// tipOpenMs==0 means empty archive → seed the last SQLiteCatchUpMaxBars closed bars.
// Large holes are filled progressively (caller may loop with a pause).
//
// Calendar TFs: start = NextBarOpen(tip); span capped via AdvanceBarOpen — never tip+30d.
func SQLiteCatchUpWindow(tipOpenMs, currentMs int64, interval string) (startMs, endMs int64, ok bool, err error) {
	endMs, err = CapKlineEndToLastClosed(currentMs, interval)
	if err != nil {
		return 0, 0, false, err
	}
	if tipOpenMs <= 0 {
		startMs, err = RetreatBarOpen(endMs, SQLiteCatchUpMaxBars-1, interval)
		if err != nil {
			return 0, 0, false, err
		}
		if startMs < 0 {
			startMs = 0
		}
		return startMs, endMs, startMs <= endMs, nil
	}
	startMs, err = NextBarOpen(tipOpenMs, interval)
	if err != nil {
		return 0, 0, false, err
	}
	if startMs > endMs {
		return 0, 0, false, nil
	}
	maxEnd, err := AdvanceBarOpen(startMs, SQLiteCatchUpMaxBars-1, interval)
	if err != nil {
		return 0, 0, false, err
	}
	if endMs > maxEnd {
		endMs = maxEnd
	}
	return startMs, endMs, true, nil
}
