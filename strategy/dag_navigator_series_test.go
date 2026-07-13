package strategy

import (
	"math"
	"testing"

	"trading_bot/exchange"
)

func TestExtractDAGNavigatorSeries_Aligned(t *testing.T) {
	t.Parallel()
	klines := make([]exchange.Kline, 120)
	base := int64(1_700_000_000_000)
	for i := range klines {
		px := 100.0 + float64(i)*0.25
		ot := base + int64(i)*60_000
		klines[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + 59_999,
			Open: px, High: px + 1, Low: px - 1, Close: px + 0.2, Volume: 10,
		})
	}
	rsx, woz := ExtractDAGNavigatorSeries(klines, GetRSXSettings())
	if len(rsx) != len(klines) || len(woz) != len(klines) {
		t.Fatalf("len rsx=%d woz=%d want %d", len(rsx), len(woz), len(klines))
	}
	// Tail should be finite after warmup.
	tail := rsx[len(rsx)-1]
	if math.IsNaN(tail) {
		t.Fatal("expected finite RSX at tip")
	}
}
