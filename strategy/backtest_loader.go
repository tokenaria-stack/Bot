package strategy

import (
	"fmt"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

// LoadBacktestCandlesOpts configures historical candle loading for CLI / batch runs.
type LoadBacktestCandlesOpts struct {
	Symbol   string
	Interval string
	StartMs  int64
	EndMs    int64
	Rest     *exchange.BinanceExchange // optional REST gap-fill when SQLite is sparse
}

// LoadBacktestCandles reads OHLCV from SQLite (continuous contract stitch).
// When Rest is set and bars are below BacktestMinBars(), it fetches via the same
// unified layer as the dashboard (SQLite write-through + reload).
func LoadBacktestCandles(opts LoadBacktestCandlesOpts) ([]exchange.Candle, int64, error) {
	if opts.Symbol == "" {
		opts.Symbol = "BTCUSDT"
	}
	if opts.Interval == "" {
		opts.Interval = "15m"
	}
	if opts.EndMs <= 0 {
		opts.EndMs = time.Now().UnixMilli()
	}
	if opts.StartMs <= 0 || opts.StartMs >= opts.EndMs {
		opts.StartMs = opts.EndMs - int64(180)*24*time.Hour.Milliseconds()
	}

	if err := data.InitDB(); err != nil {
		return nil, opts.StartMs, fmt.Errorf("init history db: %w", err)
	}

	effectiveStart := opts.StartMs
	candles, err := loadCandlesRange(opts.Symbol, opts.Interval, effectiveStart, opts.EndMs)
	if err != nil {
		return nil, effectiveStart, err
	}

	minBars := BacktestMinBars()
	if len(candles) < minBars && opts.Rest != nil {
		fetched, fetchErr := opts.Rest.FetchHistoricalKlines(opts.Symbol, opts.Interval, effectiveStart, opts.EndMs)
		if fetchErr != nil {
			return nil, effectiveStart, fmt.Errorf("sqlite %d bars, rest fetch: %w", len(candles), fetchErr)
		}
		candles = fetched
	}

	for padAttempt := 0; len(candles) < minBars && padAttempt < 4; padAttempt++ {
		paddedStart, ok := PadBacktestStartMs(opts.Interval, effectiveStart, opts.EndMs, len(candles))
		if !ok {
			break
		}
		effectiveStart = paddedStart
		if opts.Rest != nil {
			candles, err = opts.Rest.FetchHistoricalKlines(opts.Symbol, opts.Interval, effectiveStart, opts.EndMs)
		} else {
			candles, err = loadCandlesRange(opts.Symbol, opts.Interval, effectiveStart, opts.EndMs)
		}
		if err != nil {
			return nil, effectiveStart, err
		}
	}

	if len(candles) < minBars {
		return candles, effectiveStart, fmt.Errorf("not enough candles (%d, need %d)", len(candles), minBars)
	}
	return candles, effectiveStart, nil
}

func loadCandlesRange(symbol, interval string, startMs, endMs int64) ([]exchange.Candle, error) {
	raw, err := exchange.LoadContinuousContractFromDB(symbol, interval, startMs, endMs)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
