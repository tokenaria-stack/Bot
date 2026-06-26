package strategy

import (
	"sort"

	"trading_bot/exchange"
)

// mergeKlinesByOpenTime unions two series; overlay wins on duplicate OpenTime (live/WS over DB).
func mergeKlinesByOpenTime(primary, overlay []exchange.Kline) []exchange.Kline {
	if len(overlay) == 0 {
		out := make([]exchange.Kline, len(primary))
		for i, k := range primary {
			out[i] = exchange.NormalizeKline(k)
		}
		return out
	}
	if len(primary) == 0 {
		out := make([]exchange.Kline, len(overlay))
		for i, k := range overlay {
			out[i] = exchange.NormalizeKline(k)
		}
		return out
	}

	merged := make(map[int64]exchange.Kline, len(primary)+len(overlay))
	for _, k := range primary {
		merged[k.OpenTime] = exchange.NormalizeKline(k)
	}
	for _, k := range overlay {
		merged[k.OpenTime] = exchange.NormalizeKline(k)
	}
	out := make([]exchange.Kline, 0, len(merged))
	for _, k := range merged {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenTime < out[j].OpenTime })
	return out
}
