package market

import (
	"fmt"
	"sync/atomic"
	"testing"

	"trading_bot/exchange"
)

func TestHealClosedFillWindow(t *testing.T) {
	step := int64(60_000)
	tip := int64(1_784_786_580_000)
	pend := tip + 2*step
	from, to, ok := healClosedFillWindow(tip, pend, "1m")
	if !ok || from != tip+step || to != tip+step {
		t.Fatalf("1-bar gap: from=%d to=%d ok=%v want from=to=%d", from, to, ok, tip+step)
	}
	from, to, ok = healClosedFillWindow(tip, tip+step, "1m")
	if ok {
		t.Fatalf("adjacent pending: unexpected fill window from=%d to=%d", from, to)
	}
	from, to, ok = healClosedFillWindow(tip, tip, "1m")
	if ok {
		t.Fatalf("same open: unexpected fill ok from=%d to=%d", from, to)
	}
	nFrom, nTo, ok := healClosedFillWindow(tip, tip+5*step, "1m")
	if !ok || nFrom != tip+step || nTo != tip+4*step {
		t.Fatalf("N-gap: from=%d to=%d ok=%v", nFrom, nTo, ok)
	}
}

// TestTimelineHeal_B3_OneBarGapFilledBeforePublish — ADR-017: missing closed bar
// restored via Exact fill before pending flush; publishable only when contiguous.
func TestTimelineHeal_B3_OneBarGapFilledBeforePublish(t *testing.T) {
	step := int64(60_000)
	n := 30
	closed := make([]exchange.Kline, n)
	base := int64(1_784_786_400_000)
	base = (base / step) * step
	for i := 0; i < n; i++ {
		ot := base + int64(i)*step
		p := 100.0 + float64(i)*0.1
		closed[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + step - 1,
			Open: p, High: p + 1, Low: p - 1, Close: p + 0.2, Volume: 10,
		})
	}
	capTip := closed[n-1]
	missingOT := capTip.OpenTime + step
	formingOT := capTip.OpenTime + 2*step

	missingBar := exchange.NormalizeKline(exchange.Kline{
		OpenTime: missingOT, CloseTime: missingOT + step - 1,
		Open: 105, High: 106, Low: 104, Close: 105.5, Volume: 20,
	})
	forming := exchange.NormalizeKline(exchange.Kline{
		OpenTime: formingOT, CloseTime: formingOT + step - 1,
		Open: 110, High: 112, Low: 109, Close: 111, Volume: 40,
	})

	frame := NewFrame(append([]exchange.Kline{}, closed...), "1m", testChaos())
	rt := NewRuntime(map[string]*Frame{"1m": frame}, nil, nil, true, false, "BTCUSDT", "1m")
	rt.OnBinanceDisconnect()

	rt.enqueuePendingTick(exchange.WsTick{Timeframe: "1m", IsClosed: false, Kline: forming})

	var published atomic.Bool
	rt.SetOnTimelinePublishable(func() { published.Store(true) })

	rt.healClosedFetcher = func(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
		if fromMs != missingOT || toMs != missingOT {
			t.Fatalf("Exact fill window from=%d to=%d want %d", fromMs, toMs, missingOT)
		}
		return []exchange.Candle{{
			OpenTime: missingBar.OpenTime, CloseTime: missingBar.CloseTime,
			Open: missingBar.Open, High: missingBar.High, Low: missingBar.Low,
			Close: missingBar.Close, Volume: missingBar.Volume,
		}}, nil
	}

	if !rt.finalizeTimelineHealFlush() {
		t.Fatal("expected finalizeTimelineHealFlush success")
	}
	if !published.Load() || !rt.IsTimelinePublishable() {
		t.Fatal("expected publishable after contiguous heal")
	}

	raw := frame.GetKlines()
	tip := raw[len(raw)-1].OpenTime
	prev := raw[len(raw)-2].OpenTime
	mid := raw[len(raw)-3].OpenTime
	if mid != capTip.OpenTime || prev != missingOT || tip != formingOT {
		t.Fatalf("want Cap/missing/forming = %d/%d/%d got %d/%d/%d",
			capTip.OpenTime, missingOT, formingOT, mid, prev, tip)
	}
	if !rt.framesSeriesContiguous() {
		t.Fatal("Frame must be contiguous")
	}
	fmt.Printf("B3.0 one-bar heal OK: %d → %d → %d\n",
		exchange.ChartTimeSec(mid), exchange.ChartTimeSec(prev), exchange.ChartTimeSec(tip))
}

func TestTimelineHeal_B3_NBarGapFilled(t *testing.T) {
	step := int64(60_000)
	n := 20
	closed := make([]exchange.Kline, n)
	base := int64(1_784_786_000_000)
	base = (base / step) * step
	for i := 0; i < n; i++ {
		ot := base + int64(i)*step
		closed[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + step - 1,
			Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 5,
		})
	}
	capTip := closed[n-1]
	gapN := 3
	formingOT := capTip.OpenTime + int64(gapN+1)*step

	fillBars := make([]exchange.Candle, gapN)
	for i := 0; i < gapN; i++ {
		ot := capTip.OpenTime + int64(i+1)*step
		fillBars[i] = exchange.Candle{
			OpenTime: ot, CloseTime: ot + step - 1,
			Open: 100, High: 101, Low: 99, Close: 100.2, Volume: 5,
		}
	}
	forming := exchange.NormalizeKline(exchange.Kline{
		OpenTime: formingOT, CloseTime: formingOT + step - 1,
		Open: 110, High: 111, Low: 109, Close: 110.5, Volume: 8,
	})

	frame := NewFrame(append([]exchange.Kline{}, closed...), "1m", testChaos())
	rt := NewRuntime(map[string]*Frame{"1m": frame}, nil, nil, true, false, "BTCUSDT", "1m")
	rt.OnBinanceDisconnect()
	rt.enqueuePendingTick(exchange.WsTick{Timeframe: "1m", IsClosed: false, Kline: forming})
	rt.healClosedFetcher = func(string, string, int64, int64) ([]exchange.Candle, error) {
		return fillBars, nil
	}
	if !rt.finalizeTimelineHealFlush() {
		t.Fatal("N-bar heal finalize failed")
	}
	if !rt.framesSeriesContiguous() {
		t.Fatal("N-bar Frame not contiguous")
	}
	raw := frame.GetKlines()
	if raw[len(raw)-1].OpenTime != formingOT {
		t.Fatalf("tip want %d got %d", formingOT, raw[len(raw)-1].OpenTime)
	}
}

func TestTimelineHeal_B3_NoPendingUnchangedContiguous(t *testing.T) {
	step := int64(60_000)
	base := (int64(1_784_786_200_000) / step) * step
	klines := make([]exchange.Kline, 40)
	for i := range klines {
		ot := base + int64(i)*step
		klines[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + step - 1,
			Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 5,
		})
	}
	frame := NewFrame(klines, "1m", testChaos())
	rt := NewRuntime(map[string]*Frame{"1m": frame}, nil, nil, true, false, "BTCUSDT", "1m")
	rt.OnBinanceDisconnect()
	rt.healClosedFetcher = func(string, string, int64, int64) ([]exchange.Candle, error) {
		t.Fatal("Exact fetch must not run without pending jump")
		return nil, nil
	}
	if !rt.finalizeTimelineHealFlush() {
		t.Fatal("no-pending heal should publish")
	}
	if !rt.IsTimelinePublishable() {
		t.Fatal("want publishable")
	}
}

func TestTimelineHeal_B3_EmptyFillBlocksPublish(t *testing.T) {
	step := int64(60_000)
	base := int64(1_784_787_000_000)
	klines := make([]exchange.Kline, 20)
	for i := range klines {
		ot := base + int64(i)*step
		klines[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + step - 1,
			Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 1,
		})
	}
	capTip := klines[len(klines)-1]
	formingOT := capTip.OpenTime + 2*step
	forming := exchange.NormalizeKline(exchange.Kline{
		OpenTime: formingOT, CloseTime: formingOT + step - 1,
		Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 1,
	})

	frame := NewFrame(klines, "1m", testChaos())
	rt := NewRuntime(map[string]*Frame{"1m": frame}, nil, nil, true, false, "BTCUSDT", "1m")
	rt.OnBinanceDisconnect()
	rt.enqueuePendingTick(exchange.WsTick{Timeframe: "1m", IsClosed: false, Kline: forming})
	rt.healClosedFetcher = func(string, string, int64, int64) ([]exchange.Candle, error) {
		return nil, nil // empty — exchange hole / Cap race
	}
	if rt.finalizeTimelineHealFlush() {
		t.Fatal("empty Exact fill must not publish")
	}
	if rt.IsTimelinePublishable() {
		t.Fatal("must remain unpublishable")
	}
	if len(rt.snapshotPendingTicks()) == 0 {
		t.Fatal("pending must be preserved when fill fails")
	}
}
