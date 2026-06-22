package server

import (
	"regexp"
	"strconv"
	"strings"

	"trading_bot/domain"
)

var (
	secondsTFRe = regexp.MustCompile(`^(\d+)s$`)
	ticksTFRe   = regexp.MustCompile(`^(\d+)tick(s)?$`)
)

// IsOrderFlowTimeframe reports whether the TF is synthesized from aggTrade ticks.
func IsOrderFlowTimeframe(spec TimeframeSpec) bool {
	id := strings.ToLower(spec.ID)
	if strings.Contains(id, "tick") {
		return true
	}
	return secondsTFRe.MatchString(id)
}

// SynthesizeMicroCandles builds OHLC candles from raw aggTrades.
// Seconds TF (e.g. "15s"): time-window buckets.
// Tick TF (e.g. "100ticks"): fixed N trades per candle, time = last trade in bucket.
// Candle.Time is always Unix seconds for Lightweight Charts.
func SynthesizeMicroCandles(trades []domain.AggTrade, tf string) []ChartCandle {
	if len(trades) == 0 {
		return []ChartCandle{}
	}

	tf = strings.ToLower(strings.TrimSpace(tf))
	if m := ticksTFRe.FindStringSubmatch(tf); m != nil {
		n, _ := strconv.Atoi(m[1])
		if n <= 0 {
			return []ChartCandle{}
		}
		return synthesizeTickBars(trades, n)
	}
	if m := secondsTFRe.FindStringSubmatch(tf); m != nil {
		sec, _ := strconv.Atoi(m[1])
		if sec <= 0 {
			return []ChartCandle{}
		}
		return synthesizeSecondBars(trades, int64(sec)*1000)
	}
	return []ChartCandle{}
}

// LatestMicroCandle returns the most recent synthesized bar for a micro timeframe.
func LatestMicroCandle(trades []domain.AggTrade, tf string) (ChartCandle, bool) {
	candles := SynthesizeMicroCandles(trades, tf)
	if len(candles) == 0 {
		return ChartCandle{}, false
	}
	return candles[len(candles)-1], true
}

func tradeTimeSec(timeMs int64) int64 {
	if timeMs <= 0 {
		return 0
	}
	return timeMs / 1000
}

func bucketStartSec(timeMs, windowMs int64) int64 {
	if windowMs <= 0 || timeMs <= 0 {
		return 0
	}
	return (timeMs / windowMs) * windowMs / 1000
}

func synthesizeSecondBars(trades []domain.AggTrade, windowMs int64) []ChartCandle {
	if windowMs <= 0 {
		return nil
	}

	candles := make([]ChartCandle, 0, len(trades)/10+1)
	var cur *ChartCandle
	var bucket int64 = -1

	flush := func() {
		if cur != nil {
			candles = append(candles, *cur)
			cur = nil
		}
	}

	for _, tr := range trades {
		if tr.Price <= 0 || tr.Time <= 0 {
			continue
		}
		b := tr.Time / windowMs
		if cur == nil || b != bucket {
			flush()
			bucket = b
			cur = &ChartCandle{
				Time:  bucketStartSec(tr.Time, windowMs),
				Open:  tr.Price,
				High:  tr.Price,
				Low:   tr.Price,
				Close: tr.Price,
			}
			continue
		}
		cur.Close = tr.Price
		if tr.Price > cur.High {
			cur.High = tr.Price
		}
		if tr.Price < cur.Low {
			cur.Low = tr.Price
		}
	}
	flush()
	if len(candles) == 0 {
		return []ChartCandle{}
	}
	return candles
}

func synthesizeTickBars(trades []domain.AggTrade, ticksPerBar int) []ChartCandle {
	if ticksPerBar <= 0 {
		return nil
	}

	candles := make([]ChartCandle, 0, len(trades)/ticksPerBar+1)
	var cur *ChartCandle
	count := 0

	flush := func() {
		if cur != nil {
			candles = append(candles, *cur)
			cur = nil
			count = 0
		}
	}

	for _, tr := range trades {
		if tr.Price <= 0 || tr.Time <= 0 {
			continue
		}
		if cur == nil {
			cur = &ChartCandle{
				Time:  tradeTimeSec(tr.Time),
				Open:  tr.Price,
				High:  tr.Price,
				Low:   tr.Price,
				Close: tr.Price,
			}
			count = 1
		} else {
			cur.Close = tr.Price
			cur.Time = tradeTimeSec(tr.Time)
			if tr.Price > cur.High {
				cur.High = tr.Price
			}
			if tr.Price < cur.Low {
				cur.Low = tr.Price
			}
			count++
		}
		if count >= ticksPerBar {
			flush()
		}
	}
	flush()
	if len(candles) == 0 {
		return []ChartCandle{}
	}
	return candles
}
