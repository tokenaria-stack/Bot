package exchange

import (
	"testing"

	"trading_bot/data"
)

func step15m() int64 {
	return 15 * 60 * 1000
}

func msDay(n int) int64 {
	return int64(n) * 24 * 60 * 60 * 1000
}

func emptyBounds() data.KlineCacheBounds {
	return data.KlineCacheBounds{}
}

func boundsFromCandles(candles []data.Candle) data.KlineCacheBounds {
	if len(candles) == 0 {
		return emptyBounds()
	}
	minT := candles[0].OpenTime
	maxT := candles[0].OpenTime
	for _, c := range candles[1:] {
		if c.OpenTime < minT {
			minT = c.OpenTime
		}
		if c.OpenTime > maxT {
			maxT = c.OpenTime
		}
	}
	return data.KlineCacheBounds{
		Count:   len(candles),
		MinTime: minT,
		MaxTime: maxT,
		HasData: true,
	}
}

func TestDetectKlineGaps_EmptyDB(t *testing.T) {
	t.Parallel()

	start := int64(1_700_000_000_000)
	end := start + msDay(15)
	gaps := detectKlineGaps(nil, start, end, emptyBounds(), step15m())
	if len(gaps) != 1 {
		t.Fatalf("len(gaps) = %d, want 1 full-range gap", len(gaps))
	}
	if gaps[0].start != start || gaps[0].end != end {
		t.Fatalf("unexpected gap: %+v", gaps[0])
	}
}

func TestDetectKlineGaps_HeadGapOnly(t *testing.T) {
	t.Parallel()

	start := int64(1_700_000_000_000)
	mid := start + msDay(3)
	end := start + msDay(15)
	step := step15m()

	db := []data.Candle{
		{OpenTime: mid, CloseTime: mid + step - 1},
		{OpenTime: mid + step, CloseTime: mid + 2*step - 1},
	}
	bounds := boundsFromCandles(db)

	gaps := detectKlineGaps(db, start, end, bounds, step)
	if len(gaps) != 2 {
		t.Fatalf("len(gaps) = %d, want head + tail gaps, got %+v", len(gaps), gaps)
	}
	if gaps[0].start != start || gaps[0].end != mid-1 {
		t.Fatalf("head gap = %+v, want [%d .. %d]", gaps[0], start, mid-1)
	}
	wantTailStart := mid + 2*step
	if gaps[1].start != wantTailStart {
		t.Fatalf("tail gap start = %d, want %d", gaps[1].start, wantTailStart)
	}
}

func TestDetectKlineGaps_NoGapsWhenComplete(t *testing.T) {
	t.Parallel()

	start := int64(1_700_000_000_000)
	step := step15m()
	end := start + step*10

	db := make([]data.Candle, 11)
	for i := range db {
		open := start + int64(i)*step
		db[i] = data.Candle{OpenTime: open, CloseTime: open + step - 1}
	}

	gaps := detectKlineGaps(db, start, end, boundsFromCandles(db), step)
	if len(gaps) != 0 {
		t.Fatalf("expected cache hit (no gaps), got %+v", gaps)
	}
}

func TestDetectKlineGaps_TailGap(t *testing.T) {
	t.Parallel()

	start := int64(1_700_000_000_000)
	step := step15m()
	end := start + msDay(2)

	lastOpen := end - msDay(1)
	db := make([]data.Candle, 0, 96)
	for open := start; open <= lastOpen; open += step {
		db = append(db, data.Candle{OpenTime: open, CloseTime: open + step - 1})
	}
	bounds := boundsFromCandles(db)

	gaps := detectKlineGaps(db, start, end, bounds, step)
	if len(gaps) != 1 {
		t.Fatalf("len(gaps) = %d, want 1 tail gap, got %+v", len(gaps), gaps)
	}
	wantStart := lastOpen + step
	if gaps[0].start != wantStart {
		t.Fatalf("tail gap start = %d, want %d", gaps[0].start, wantStart)
	}
	if gaps[0].end != end {
		t.Fatalf("tail gap end = %d, want %d", gaps[0].end, end)
	}
}

func TestDetectKlineGaps_InternalHole15m(t *testing.T) {
	t.Parallel()

	start := int64(1_700_000_000_000)
	step := step15m()
	end := start + step*5

	db := []data.Candle{
		{OpenTime: start, CloseTime: start + step - 1},
		{OpenTime: start + 3*step, CloseTime: start + 4*step - 1},
	}

	gaps := detectKlineGaps(db, start, end, boundsFromCandles(db), step)
	if len(gaps) != 2 {
		t.Fatalf("len(gaps) = %d, want internal + tail, got %+v", len(gaps), gaps)
	}
	if gaps[0].start != start+step || gaps[0].end != start+3*step-1 {
		t.Fatalf("internal gap = %+v, want [%d .. %d]", gaps[0], start+step, start+3*step-1)
	}
}

func TestDetectKlineGaps_HeadFromBoundsWhenLoadEmpty(t *testing.T) {
	t.Parallel()

	start := int64(1_700_000_000_000)
	mid := start + msDay(5)
	end := start + msDay(20)

	bounds := data.KlineCacheBounds{
		Count:   100,
		MinTime: mid,
		MaxTime: end - msDay(1),
		HasData: true,
	}

	gaps := detectKlineGaps(nil, start, end, bounds, step15m())
	if len(gaps) != 2 {
		t.Fatalf("len(gaps) = %d, want head+tail without full refetch, got %+v", len(gaps), gaps)
	}
	if gaps[0].start != start || gaps[0].end != mid-1 {
		t.Fatalf("head gap = %+v", gaps[0])
	}
}

func TestSynthesizeForwardFillGap(t *testing.T) {
	t.Parallel()

	step := step15m()
	start := int64(1_700_000_000_000)
	seed := Candle{OpenTime: start, Close: 42_000, Open: 42_000}
	gap := klineTimeRange{start: start + step, end: start + 3*step - 1}

	got := synthesizeForwardFillGap(gap, step, seed, true)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 synthetic bars", len(got))
	}
	if got[0].OpenTime != start+step || got[0].Close != 42_000 {
		t.Fatalf("first synthetic = %+v, want open=%d close=42000", got[0], start+step)
	}
	if got[1].OpenTime != start+2*step {
		t.Fatalf("second synthetic open = %d, want %d", got[1].OpenTime, start+2*step)
	}
}

func TestMergeDataAndExchangeCandles_Dedupes(t *testing.T) {
	t.Parallel()

	db := []data.Candle{
		{OpenTime: 100, CloseTime: 199, Close: 1},
		{OpenTime: 200, CloseTime: 299, Close: 2},
	}
	fetched := []Candle{
		{OpenTime: 200, CloseTime: 299, Close: 2.5},
		{OpenTime: 300, CloseTime: 399, Close: 3},
	}

	merged := mergeDataAndExchangeCandles(db, fetched)
	if len(merged) != 3 {
		t.Fatalf("len = %d, want 3", len(merged))
	}
	if merged[1].Close != 2.5 {
		t.Fatalf("duplicate open_time should prefer fetched bar, close=%v", merged[1].Close)
	}
}

func TestFilterCandlesInRange(t *testing.T) {
	t.Parallel()

	candles := []Candle{
		{OpenTime: 50},
		{OpenTime: 100},
		{OpenTime: 200},
		{OpenTime: 300},
	}
	out := filterCandlesInRange(candles, 100, 250)
	if len(out) != 2 || out[0].OpenTime != 100 || out[1].OpenTime != 200 {
		t.Fatalf("unexpected filter result: %+v", out)
	}
}
