package exchange

import (
	"context"
	"fmt"

	binance "github.com/adshao/go-binance/v2"
)

// FetchSpotVolumeAuthorityPage pages spot REST klines volume fields (index 5 = base v).
func FetchSpotVolumeAuthorityPage(symbol, interval string, startOpenMs int64, limit int) ([]VolumeAuthority, error) {
	if limit <= 0 || limit > maxKlinesLimit {
		return nil, fmt.Errorf("limit must be 1..%d", maxKlinesLimit)
	}
	symbol = NormalizeFuturesSymbol(symbol)
	startOpenMs = alignOpenTimeMs(startOpenMs, interval)
	client := binance.NewClient("", "")
	klines, err := client.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		StartTime(startOpenMs).
		Limit(limit).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("spot volume authority %s %s: %w", symbol, interval, err)
	}
	out := make([]VolumeAuthority, 0, len(klines))
	for i, k := range klines {
		if k == nil {
			return nil, fmt.Errorf("nil spot kline at %d", i)
		}
		row, err := VolumeAuthorityFromSpotRESTKline(k)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}
