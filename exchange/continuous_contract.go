package exchange

import (
	"fmt"

	"trading_bot/data"
)

// SpotStorageSymbol returns the SQLite symbol for spot-era candles (e.g. BTCUSDT_SPOT).
func SpotStorageSymbol(symbol string) string {
	return NormalizeFuturesSymbol(symbol) + "_SPOT"
}

// contractSegment is one contiguous slice of a continuous-contract request.
type contractSegment struct {
	start       int64
	end         int64
	spotStorage bool // _SPOT SQLite key when true; futures otherwise
}

// splitRangeAtGenesis splits [startMs, endMs] around BinanceFuturesGenesisMs.
func splitRangeAtGenesis(startMs, endMs int64) []contractSegment {
	if endMs < startMs {
		return nil
	}
	if startMs >= BinanceFuturesGenesisMs {
		return []contractSegment{{start: startMs, end: endMs, spotStorage: false}}
	}
	if endMs < BinanceFuturesGenesisMs {
		return []contractSegment{{start: startMs, end: endMs, spotStorage: true}}
	}
	return []contractSegment{
		{start: startMs, end: BinanceFuturesGenesisMs - 1, spotStorage: true},
		{start: BinanceFuturesGenesisMs, end: endMs, spotStorage: false},
	}
}

func storageSymbolForSegment(symbol string, seg contractSegment) string {
	if seg.spotStorage {
		return SpotStorageSymbol(symbol)
	}
	return NormalizeFuturesSymbol(symbol)
}

// LoadContinuousContractFromDB reads spot (_SPOT) and futures rows from SQLite and stitches them.
// When limit > 0, returns at most the last limit stitched bars (ascending).
func LoadContinuousContractFromDB(symbol, interval string, startTimeMs, endTimeMs int64, limit int) ([]Candle, error) {
	symbol = NormalizeFuturesSymbol(symbol)
	var merged []data.Candle

	for _, seg := range splitRangeAtGenesis(startTimeMs, endTimeMs) {
		storageSym := storageSymbolForSegment(symbol, seg)
		rows, err := data.LoadKlines(storageSym, interval, seg.start, seg.end, 0)
		if err != nil {
			return nil, fmt.Errorf("load %s %s [%d..%d]: %w", storageSym, interval, seg.start, seg.end, err)
		}
		merged = append(merged, rows...)
	}

	out := candlesFromData(dedupeDataCandlesByOpenTime(merged))
	return TruncateCandlesTail(out, limit), nil
}

// TruncateCandlesTail keeps the last max bars; max <= 0 is a no-op.
func TruncateCandlesTail(candles []Candle, max int) []Candle {
	if max <= 0 || len(candles) <= max {
		return candles
	}
	return candles[len(candles)-max:]
}

// normalizeSpotRange clamps spot-era window to BinanceSpotGenesisMs.
func normalizeSpotRange(startMs, endMs int64) (int64, int64, bool) {
	if endMs <= 0 || endMs < BinanceSpotGenesisMs {
		return 0, 0, false
	}
	if startMs < BinanceSpotGenesisMs {
		startMs = BinanceSpotGenesisMs
	}
	if startMs >= endMs {
		return 0, 0, false
	}
	return startMs, endMs, true
}

// normalizeContinuousContractRange clamps pre-spot starts for continuous-contract windows.
func normalizeContinuousContractRange(startMs, endMs int64) (int64, int64, bool) {
	if endMs <= 0 {
		return 0, 0, false
	}
	if endMs < BinanceSpotGenesisMs {
		return 0, 0, false
	}
	if startMs < BinanceSpotGenesisMs {
		startMs = BinanceSpotGenesisMs
	}
	if startMs >= endMs {
		return 0, 0, false
	}
	return startMs, endMs, true
}

func dedupeDataCandlesByOpenTime(candles []data.Candle) []data.Candle {
	if len(candles) == 0 {
		return candles
	}
	out := make([]data.Candle, 0, len(candles))
	var lastOpen int64 = -1
	for _, c := range candles {
		if c.OpenTime == lastOpen {
			out[len(out)-1] = c
			continue
		}
		out = append(out, c)
		lastOpen = c.OpenTime
	}
	return out
}
