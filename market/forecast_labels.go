package market

import (
	"fmt"

	"trading_bot/exchange"
	"trading_bot/forecast"
)

// DumpLabelSet writes one immutable LabelSet from a FeatureTape and a
// materialized closed-bar slice. Converts Kline → CanonicalClosedBar only;
// labeling math lives in forecast.
func DumpLabelSet(path, tapePath string, spec forecast.TargetSpec, bars []exchange.Kline, expect *forecast.LabelExpect) error {
	if len(bars) == 0 {
		return fmt.Errorf("market: refuse empty label-set source")
	}
	canon := make([]forecast.CanonicalClosedBar, len(bars))
	for i, k := range bars {
		canon[i] = forecast.CanonicalClosedBar{
			OpenTime: k.OpenTime,
			Open:     k.Open,
			High:     k.High,
			Low:      k.Low,
			Close:    k.Close,
			Volume:   k.Volume,
		}
	}
	return forecast.GenerateLabelSet(path, tapePath, spec, canon, expect)
}
