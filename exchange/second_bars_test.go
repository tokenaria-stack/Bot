package exchange

import (
	"reflect"
	"testing"
	"trading_bot/data"
)

func TestSecondBarOHLCVSameSecond(t *testing.T) {
	t.Parallel()
	b := NewSecondBarBuilder()
	base := int64(1_700_000_000_000)
	_, f, closed, ok := b.OnAggTrade(AggTrade{TimeMs: base + 10, Price: 100, Qty: 1})
	if closed || !ok || f.Open != 100 || f.Volume != 1 {
		t.Fatalf("first %+v closed=%v ok=%v", f, closed, ok)
	}
	_, f, closed, ok = b.OnAggTrade(AggTrade{TimeMs: base + 200, Price: 110, Qty: 2})
	if closed || !ok {
		t.Fatal("same second must not close")
	}
	_, f, closed, ok = b.OnAggTrade(AggTrade{TimeMs: base + 800, Price: 90, Qty: 3})
	if closed || !ok {
		t.Fatal("third")
	}
	if f.Open != 100 || f.High != 110 || f.Low != 90 || f.Close != 90 || f.Volume != 6 {
		t.Fatalf("OHLCV %+v", f)
	}
	if f.OpenTime != base {
		t.Fatalf("bucket T: open=%d want %d", f.OpenTime, base)
	}
	ct, _ := data.BarCloseTimeMs(base, "1s")
	if f.CloseTime != ct {
		t.Fatalf("CloseTime=%d want %d", f.CloseTime, ct)
	}
}

func TestSecondBarClosesOnceOnNextSecond(t *testing.T) {
	t.Parallel()
	b := NewSecondBarBuilder()
	base := int64(1_700_000_001_000)
	b.OnAggTrade(AggTrade{TimeMs: base + 1, Price: 10, Qty: 1})
	closed, forming, didClose, ok := b.OnAggTrade(AggTrade{TimeMs: base + 1000, Price: 12, Qty: 1})
	if !ok || !didClose {
		t.Fatal("next second must close prior once")
	}
	if closed.OpenTime != base || closed.Close != 10 || closed.Volume != 1 {
		t.Fatalf("closed %+v", closed)
	}
	if forming.OpenTime != base+1000 || forming.Open != 12 {
		t.Fatalf("forming %+v", forming)
	}
	_, _, didClose, _ = b.OnAggTrade(AggTrade{TimeMs: base + 1100, Price: 13, Qty: 1})
	if didClose {
		t.Fatal("repeat same new second must not close again")
	}
}

func TestSecondBarUsesTradeTimeNotArrival(t *testing.T) {
	t.Parallel()
	b := NewSecondBarBuilder()
	// Event processed "now" but T is previous second.
	tms := int64(1_700_000_002_500)
	_, f, _, ok := b.OnAggTrade(AggTrade{TimeMs: tms, Price: 1, Qty: 1})
	if !ok || f.OpenTime != 1_700_000_002_000 {
		t.Fatalf("must bucket on T, got %+v", f)
	}
}

func TestSecondBarDropsLateAndDuplicateOpen(t *testing.T) {
	t.Parallel()
	b := NewSecondBarBuilder()
	base := int64(1_700_000_003_000)
	b.OnAggTrade(AggTrade{TimeMs: base + 10, Price: 1, Qty: 1})
	b.OnAggTrade(AggTrade{TimeMs: base + 1000, Price: 2, Qty: 1})
	_, _, didClose, ok := b.OnAggTrade(AggTrade{TimeMs: base + 50, Price: 9, Qty: 1})
	if ok || didClose {
		t.Fatal("late T must drop")
	}
	_, f, didClose, ok := b.OnAggTrade(AggTrade{TimeMs: base + 1500, Price: 3, Qty: 1})
	if !ok || didClose || f.OpenTime != base+1000 || f.High != 3 {
		t.Fatalf("same open replace %+v closed=%v", f, didClose)
	}
}

func TestSecondBarBuilderHoldsNoTradeList(t *testing.T) {
	t.Parallel()
	b := NewSecondBarBuilder()
	b.OnAggTrade(AggTrade{TimeMs: 1_700_000_004_000, Price: 1, Qty: 1})
	rv := reflect.ValueOf(*b)
	for i := 0; i < rv.NumField(); i++ {
		if rv.Field(i).Kind() == reflect.Slice {
			t.Fatal("builder must not retain a trade slice")
		}
	}
}

func TestLiveSecondNotNativePersist(t *testing.T) {
	t.Parallel()
	if IsNativeBinance("1s") || ShouldPersist("1s") {
		t.Fatal("1s must not persist or be native")
	}
	if !IsLiveSecond("1s") || !IsLiveChartTF("1s") {
		t.Fatal("1s must be live second")
	}
	if IsLiveSecond("5s") {
		t.Fatal("5s must not be the 1s live-second identity")
	}
	if !IsSparseSecondChild("5s") || !IsLiveChartTF("5s") {
		t.Fatal("5s must be an activated sparse-second child")
	}
	e5, ok := TimeframeByName("5s")
	if !ok || e5.LiveSource != LiveParentClosed || e5.Persist || e5.Parent != SecondTF {
		t.Fatalf("5s catalog %+v", e5)
	}
	if IsSparseSecondChild("10s") || IsLiveChartTF("10s") {
		t.Fatal("10s must stay inactive")
	}
	for _, s := range CombinedKlineStreamNames("BTCUSDT") {
		if s == "btcusdt@kline_1s" {
			t.Fatal("1s must not be a kline stream")
		}
	}
	found := false
	for _, s := range CombinedLiveStreamNames("BTCUSDT") {
		if s == "btcusdt@aggTrade" {
			found = true
		}
		if s == "btcusdt@kline_2m" {
			t.Fatal("derived kline leaked")
		}
	}
	if !found {
		t.Fatal("combined live streams must include aggTrade")
	}
}

func TestSecondBarSeedClosedFloorDropsSameSecond(t *testing.T) {
	t.Parallel()
	b := NewSecondBarBuilder()
	base := int64(1_700_000_100_000)
	b.SeedClosedFloor(base)
	_, _, didClose, ok := b.OnAggTrade(AggTrade{TimeMs: base + 10, Price: 1, Qty: 1})
	if ok || didClose {
		t.Fatal("trades in hydrated closed second must drop")
	}
	closed, forming, didClose, ok := b.OnAggTrade(AggTrade{TimeMs: base + 1000, Price: 2, Qty: 1})
	if !ok || didClose || closed.OpenTime != 0 || forming.OpenTime != base+1000 {
		t.Fatalf("next second must start forming, got closed=%+v forming=%+v didClose=%v ok=%v", closed, forming, didClose, ok)
	}
}
