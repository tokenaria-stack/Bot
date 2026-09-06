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
	canon, err := klinesToCanonical(bars)
	if err != nil {
		return err
	}
	return forecast.GenerateLabelSet(path, tapePath, spec, canon, expect)
}

// DumpLabelSetWithFiner writes a v2 LabelSet using materialized primary and finer klines.
func DumpLabelSetWithFiner(path, tapePath string, spec forecast.TargetSpec, primary, finer []exchange.Kline, finerMarket forecast.MarketKey, expect *forecast.LabelExpect) error {
	p, err := klinesToCanonical(primary)
	if err != nil {
		return err
	}
	var f []forecast.CanonicalClosedBar
	if len(finer) > 0 {
		f, err = klinesToCanonical(finer)
		if err != nil {
			return err
		}
	}
	return forecast.GenerateLabelSetWithFiner(path, tapePath, spec, p, finerMarket, f, expect)
}

func klinesToCanonical(bars []exchange.Kline) ([]forecast.CanonicalClosedBar, error) {
	if len(bars) == 0 {
		return nil, fmt.Errorf("market: refuse empty label-set source")
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
	return canon, nil
}
