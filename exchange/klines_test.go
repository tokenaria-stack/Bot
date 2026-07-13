package exchange

import (
	"testing"
)

func TestAlignKlineRangeMs(t *testing.T) {
	t.Parallel()
	step := int64(15 * 60 * 1000)
	start, end := alignKlineRangeMs(1_700_000_001_234, 1_700_000_901_234, step)
	if start%step != 0 || end%step != 0 {
		t.Fatalf("unaligned: start=%d end=%d step=%d", start, end, step)
	}
}

func TestDedupeCandlesByOpenTime_PrefersLast(t *testing.T) {
	t.Parallel()
	in := []Candle{
		{OpenTime: 100, Close: 1},
		{OpenTime: 100, Close: 2},
		{OpenTime: 200, Close: 3},
	}
	out := dedupeCandlesByOpenTime(in)
	if len(out) != 2 || out[0].Close != 2 || out[1].Close != 3 {
		t.Fatalf("dedupe = %+v", out)
	}
}

func TestCandlesToDataRoundTrip(t *testing.T) {
	t.Parallel()
	in := []Candle{{OpenTime: 1, Open: 2, High: 3, Low: 1.5, Close: 2.5, Volume: 9, CloseTime: 10}}
	mid := CandlesToData(in)
	out := candlesFromData(mid)
	if len(out) != 1 || out[0] != in[0] {
		t.Fatalf("roundtrip = %+v", out)
	}
}
