package exchange

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2/futures"

	"trading_bot/data"
)

// Binance USDⓈ-M Futures klines: GET https://fapi.binance.com/fapi/v1/klines

const maxKlinesLimit = 1000

// BinanceFuturesGenesisMs is the earliest open time for USDⓈ-M futures klines (2019-09-08 UTC).
const BinanceFuturesGenesisMs int64 = 1567900800000

// BinanceSpotGenesisMs is the earliest open time for BTCUSDT spot klines (2017-08-17 UTC).
const BinanceSpotGenesisMs int64 = 1502928000000

// Candle represents a parsed OHLCV candle ready for technical analysis.
type Candle struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

// GetKlines fetches historical candles via GET /fapi/v1/klines (public, no signature).
// b.client is *futures.Client — never the spot binance.Client.
func (b *BinanceExchange) GetKlines(symbol, interval string, limit int) ([]Candle, error) {
	if b.client == nil {
		return nil, fmt.Errorf("futures client is not configured")
	}
	symbol = NormalizeFuturesSymbol(symbol)
	if limit <= 0 || limit > maxKlinesLimit {
		return nil, fmt.Errorf("limit must be between 1 and %d, got %d", maxKlinesLimit, limit)
	}

	klines, err := b.client.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		Limit(limit).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("fetch klines for %s: %w", symbol, err)
	}

	candles := make([]Candle, 0, len(klines))
	for i, k := range klines {
		if k == nil {
			return nil, fmt.Errorf("nil kline at index %d", i)
		}

		candle, err := candleFromFuturesKline(k)
		if err != nil {
			return nil, fmt.Errorf("parse kline at index %d: %w", i, err)
		}
		candles = append(candles, candle)
	}

	return candles, nil
}

// GetKlinesBefore fetches historical candles ending before endTimeMs (exclusive).
func (b *BinanceExchange) GetKlinesBefore(symbol, interval string, limit int, endTimeMs int64) ([]Candle, error) {
	if b.client == nil {
		return nil, fmt.Errorf("futures client is not configured")
	}
	symbol = NormalizeFuturesSymbol(symbol)
	if limit <= 0 || limit > maxKlinesLimit {
		return nil, fmt.Errorf("limit must be between 1 and %d, got %d", maxKlinesLimit, limit)
	}

	svc := b.client.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		Limit(limit)
	if endTimeMs > 0 {
		svc = svc.EndTime(endTimeMs)
	}

	klines, err := svc.Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("fetch klines for %s: %w", symbol, err)
	}

	candles := make([]Candle, 0, len(klines))
	for i, k := range klines {
		if k == nil {
			return nil, fmt.Errorf("nil kline at index %d", i)
		}
		candle, err := candleFromFuturesKline(k)
		if err != nil {
			return nil, fmt.Errorf("parse kline at index %d: %w", i, err)
		}
		candles = append(candles, candle)
	}
	return candles, nil
}

const historicalKlinesPageLimit = 1000
const historicalKlinesRequestDelay = 150 * time.Millisecond

type klineTimeRange struct {
	start int64
	end   int64
}

// FetchHistoricalKlines returns klines for [startTimeMs, endTimeMs] (milliseconds).
// Unified data layer: SQLite first, interval-aware gap detection, per-gap REST fetch,
// forward-fill synthetic fallback when the API is unavailable.
func (b *BinanceExchange) FetchHistoricalKlines(symbol, interval string, startTimeMs, endTimeMs int64) ([]Candle, error) {
	startTimeMs, endTimeMs, ok := normalizeContinuousContractRange(startTimeMs, endTimeMs)
	if !ok {
		return nil, nil
	}

	symbol = NormalizeFuturesSymbol(symbol)
	stepMs, stepErr := data.IntervalDurationMs(interval)
	if stepErr != nil {
		return nil, stepErr
	}

	bounds := queryContinuousContractCacheBounds(symbol, interval, startTimeMs, endTimeMs)
	if bounds.HasData {
		log.Printf("[Klines] continuous cache bounds %s %s: count=%d min=%d max=%d request=[%d..%d]",
			symbol, interval, bounds.Count, bounds.MinTime, bounds.MaxTime, startTimeMs, endTimeMs)
	}

	log.Printf("[Klines] loading SQLite (continuous): %s %s [%d .. %d]", symbol, interval, startTimeMs, endTimeMs)
	dbCandles, err := LoadContinuousContractFromDB(symbol, interval, startTimeMs, endTimeMs)
	if err != nil {
		log.Printf("[Klines] continuous cache read failed for %s %s: %v", symbol, interval, err)
		dbCandles = nil
	}
	dbKlines := candlesToData(dbCandles)
	log.Printf("[Klines] SQLite returned %d stitched bars for requested range", len(dbKlines))

	gaps := detectKlineGaps(dbKlines, startTimeMs, endTimeMs, bounds, stepMs)
	if len(gaps) == 0 {
		log.Printf("[Klines] cache hit: %s %s (%d bars)", symbol, interval, len(dbKlines))
		return candlesFromData(dbKlines), nil
	}

	log.Printf("[Klines] %s %s — %d gap(s) to fill", symbol, interval, len(gaps))
	merged := candlesFromData(dbKlines)
	var fetched []Candle

	for i, gap := range gaps {
		log.Printf("[Klines] gap %d/%d: %s %s [%d .. %d]", i+1, len(gaps), symbol, interval, gap.start, gap.end)

		segments := splitRangeAtGenesis(gap.start, gap.end)
		var chunk []Candle
		apiOK := false

		for _, seg := range segments {
			storageSym := storageSymbolForSegment(symbol, seg)
			market := "futures"
			if seg.spotStorage {
				market = "spot"
			}
			log.Printf("[Klines] gap segment %s %s %s [%d .. %d]", market, storageSym, interval, seg.start, seg.end)

			var segChunk []Candle
			segOK := false
			if seg.spotStorage || b.client != nil {
				var fetchErr error
				segChunk, fetchErr = b.fetchGapSegment(symbol, interval, seg)
				if fetchErr != nil {
					if seg.spotStorage {
						log.Printf("[Warning] spot API failed (api.binance.com) for %s %s gap [%d..%d]: %v",
							symbol, interval, seg.start, seg.end, fetchErr)
					} else {
						log.Printf("[Warning] %s API failed for gap segment [%d..%d]: %v", market, seg.start, seg.end, fetchErr)
					}
				} else {
					segOK = true
				}
			} else {
				log.Printf("[Warning] futures client not configured for gap segment [%d..%d]", seg.start, seg.end)
			}

			if !segOK {
				seed, hasSeed := seedCandleForGap(merged, append(fetched, chunk...), seg.start)
				segChunk = synthesizeForwardFillGap(klineTimeRange{start: seg.start, end: seg.end}, stepMs, seed, hasSeed)
			}

			if len(segChunk) == 0 {
				log.Printf("[Warning] API returned exactly 0 bars for %s %s gap [%d..%d]. Ignoring.", market, interval, seg.start, seg.end)
				continue
			}

			if segOK {
				if saveErr := data.SaveKlines(storageSym, interval, candlesToData(segChunk)); saveErr != nil {
					log.Printf("[Klines] cache save failed for %s %s: %v", storageSym, interval, saveErr)
				} else {
					log.Printf("[Klines] cached %d %s bars for %s %s", len(segChunk), market, storageSym, interval)
				}
			} else {
				log.Printf("[Klines] synthesized %d forward-fill bars for %s segment [%d .. %d]", len(segChunk), market, seg.start, seg.end)
			}

			chunk = append(chunk, segChunk...)
			apiOK = apiOK || segOK
		}

		if len(chunk) == 0 {
			continue
		}

		chunk = dedupeCandlesByOpenTime(chunk)
		fetched = append(fetched, chunk...)
		merged = mergeDataAndExchangeCandles(candlesToData(merged), chunk)
	}

	if len(fetched) > 0 {
		reloaded, reloadErr := LoadContinuousContractFromDB(symbol, interval, startTimeMs, endTimeMs)
		if reloadErr != nil {
			log.Printf("[Klines] continuous cache reload failed for %s %s: %v", symbol, interval, reloadErr)
		} else {
			dbKlines = candlesToData(reloaded)
			log.Printf("[Klines] reloaded %d stitched bars from SQLite after gap fill", len(dbKlines))
			merged = mergeDataAndExchangeCandles(dbKlines, fetched)
		}
	}

	merged = filterCandlesInRange(merged, startTimeMs, endTimeMs)
	log.Printf("[Klines] unified result: %d bars (%d from SQLite, %d gap-filled)", len(merged), len(dbKlines), len(fetched))
	return merged, nil
}

// detectKlineGaps returns time ranges (ms) missing from the SQLite slice for [startTimeMs, endTimeMs].
func detectKlineGaps(dbKlines []data.Candle, startTimeMs, endTimeMs int64, bounds data.KlineCacheBounds, stepMs int64) []klineTimeRange {
	if stepMs <= 0 {
		stepMs = 60_000
	}

	if len(dbKlines) == 0 {
		if !bounds.HasData || bounds.Count == 0 {
			return []klineTimeRange{{start: startTimeMs, end: endTimeMs}}
		}
		var gaps []klineTimeRange
		if startTimeMs < bounds.MinTime {
			gaps = append(gaps, klineTimeRange{start: startTimeMs, end: bounds.MinTime - 1})
		}
		if endTimeMs > bounds.MaxTime {
			gaps = append(gaps, klineTimeRange{start: bounds.MaxTime + 1, end: endTimeMs})
		}
		if len(gaps) == 0 && bounds.HasData &&
			startTimeMs >= bounds.MinTime && endTimeMs <= bounds.MaxTime {
			log.Printf("[Klines] request inside DB bounds but LoadKlines=0 for [%d..%d]", startTimeMs, endTimeMs)
			return []klineTimeRange{{start: startTimeMs, end: endTimeMs}}
		}
		return gaps
	}

	var gaps []klineTimeRange

	if dbKlines[0].OpenTime > startTimeMs {
		gaps = append(gaps, klineTimeRange{
			start: startTimeMs,
			end:   dbKlines[0].OpenTime - 1,
		})
	}

	for i := 1; i < len(dbKlines); i++ {
		prev := dbKlines[i-1]
		cur := dbKlines[i]
		expectedNext := prev.OpenTime + stepMs
		if cur.OpenTime > expectedNext {
			gaps = append(gaps, klineTimeRange{
				start: expectedNext,
				end:   cur.OpenTime - 1,
			})
		}
	}

	last := dbKlines[len(dbKlines)-1]
	nextOpen := last.OpenTime + stepMs
	if nextOpen <= endTimeMs {
		gaps = append(gaps, klineTimeRange{
			start: nextOpen,
			end:   endTimeMs,
		})
	}

	return gaps
}

func seedCandleForGap(merged, fetched []Candle, gapStart int64) (Candle, bool) {
	var seed Candle
	found := false
	for _, c := range merged {
		if c.OpenTime < gapStart {
			seed = c
			found = true
		}
	}
	if found {
		return seed, true
	}
	for _, c := range fetched {
		if c.OpenTime < gapStart {
			seed = c
			found = true
		}
	}
	if found {
		return seed, true
	}
	// Head gap: borrow the first candle after the hole for a flat seed price.
	candidates := append([]Candle(nil), merged...)
	candidates = append(candidates, fetched...)
	for _, c := range candidates {
		if c.OpenTime >= gapStart {
			price := c.Open
			if price <= 0 {
				price = c.Close
			}
			if price <= 0 {
				continue
			}
			return Candle{Open: price, High: price, Low: price, Close: price}, true
		}
	}
	return Candle{}, false
}

func synthesizeForwardFillGap(gap klineTimeRange, stepMs int64, seed Candle, hasSeed bool) []Candle {
	if stepMs <= 0 || gap.end < gap.start {
		return nil
	}
	price := seed.Close
	if price <= 0 {
		price = seed.Open
	}
	if !hasSeed || price <= 0 {
		log.Printf("[Warning] cannot synthesize gap [%d..%d]: no seed price", gap.start, gap.end)
		return nil
	}

	startOpen := alignOpenTimeMs(gap.start, stepMs)
	if seed.OpenTime > 0 {
		next := seed.OpenTime + stepMs
		if next > startOpen {
			startOpen = next
		}
	}

	out := make([]Candle, 0, 16)
	for open := startOpen; open <= gap.end; open += stepMs {
		out = append(out, Candle{
			OpenTime:  open,
			Open:      price,
			High:      price,
			Low:       price,
			Close:     price,
			Volume:    0,
			CloseTime: open + stepMs - 1,
		})
	}
	return out
}

func alignOpenTimeMs(t, stepMs int64) int64 {
	if stepMs <= 0 {
		return t
	}
	return (t / stepMs) * stepMs
}

func mergeDataAndExchangeCandles(dbKlines []data.Candle, fetched []Candle) []Candle {
	if len(dbKlines) == 0 {
		return dedupeCandlesByOpenTime(fetched)
	}
	if len(fetched) == 0 {
		return candlesFromData(dbKlines)
	}

	merged := make([]Candle, 0, len(dbKlines)+len(fetched))
	merged = append(merged, candlesFromData(dbKlines)...)
	merged = append(merged, fetched...)
	return dedupeCandlesByOpenTime(merged)
}

func filterCandlesInRange(candles []Candle, startTimeMs, endTimeMs int64) []Candle {
	if len(candles) == 0 {
		return candles
	}
	out := make([]Candle, 0, len(candles))
	for _, c := range candles {
		if c.OpenTime >= startTimeMs && c.OpenTime <= endTimeMs {
			out = append(out, c)
		}
	}
	return out
}

func (b *BinanceExchange) fetchHistoricalKlinesFromAPI(symbol, interval string, startTimeMs, endTimeMs int64) ([]Candle, error) {
	cursor := startTimeMs
	all := make([]Candle, 0, historicalKlinesPageLimit)

	for cursor < endTimeMs {
		klines, err := b.client.NewKlinesService().
			Symbol(symbol).
			Interval(interval).
			StartTime(cursor).
			EndTime(endTimeMs).
			Limit(historicalKlinesPageLimit).
			Do(context.Background())
		if err != nil {
			return nil, fmt.Errorf("fetch klines page at %d: %w", cursor, err)
		}
		if len(klines) == 0 {
			break
		}

		for i, k := range klines {
			if k == nil {
				return nil, fmt.Errorf("nil kline at page offset %d", i)
			}
			candle, err := candleFromFuturesKline(k)
			if err != nil {
				return nil, fmt.Errorf("parse kline: %w", err)
			}
			all = append(all, candle)
		}

		last := klines[len(klines)-1]
		if last.CloseTime >= endTimeMs || len(klines) < historicalKlinesPageLimit {
			break
		}
		cursor = last.CloseTime + 1
		time.Sleep(historicalKlinesRequestDelay)
	}

	return dedupeCandlesByOpenTime(all), nil
}

func candlesToData(in []Candle) []data.Candle {
	out := make([]data.Candle, len(in))
	for i, c := range in {
		out[i] = data.Candle{
			OpenTime:  c.OpenTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
			CloseTime: c.CloseTime,
		}
	}
	return out
}

func candlesFromData(in []data.Candle) []Candle {
	out := make([]Candle, len(in))
	for i, c := range in {
		out[i] = Candle{
			OpenTime:  c.OpenTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
			CloseTime: c.CloseTime,
		}
	}
	return out
}

func dedupeCandlesByOpenTime(candles []Candle) []Candle {
	if len(candles) == 0 {
		return candles
	}
	out := make([]Candle, 0, len(candles))
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

func candleFromFuturesKline(k *futures.Kline) (Candle, error) {
	open, err := strconv.ParseFloat(k.Open, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse open %q: %w", k.Open, err)
	}
	high, err := strconv.ParseFloat(k.High, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse high %q: %w", k.High, err)
	}
	low, err := strconv.ParseFloat(k.Low, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse low %q: %w", k.Low, err)
	}
	closePrice, err := strconv.ParseFloat(k.Close, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse close %q: %w", k.Close, err)
	}
	volume, err := strconv.ParseFloat(k.Volume, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse volume %q: %w", k.Volume, err)
	}

	return Candle{
		OpenTime:  k.OpenTime,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     closePrice,
		Volume:    volume,
		CloseTime: k.CloseTime,
	}, nil
}
