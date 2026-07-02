package strategy

import "trading_bot/exchange"

// IndicatorWarmupBars is the unified cold-path warmup gate for streaming replay.
// Bars [0..IndicatorWarmupBars-1] prime indicators; chart output starts at this index.
const IndicatorWarmupBars = 50

// AnnotationsFromWarmup returns annotations with time >= the warmup bar open time.
func AnnotationsFromWarmup(annotations []ChartAnnotation, klines []exchange.Kline, warmupBars int) []ChartAnnotation {
	if warmupBars <= 0 || len(klines) == 0 {
		return annotations
	}
	if warmupBars >= len(klines) {
		return nil
	}
	minTime := klines[warmupBars].OpenTime / 1000
	out := make([]ChartAnnotation, 0, len(annotations))
	for _, ann := range annotations {
		if ann.Time >= minTime {
			out = append(out, ann)
		}
	}
	return out
}
