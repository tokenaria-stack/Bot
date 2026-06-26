package strategy

import (
	"path/filepath"
	"testing"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

func TestCollectWalkForwardMTFPeriods(t *testing.T) {
	t.Parallel()

	navs := map[string]NavigatorUISettings{
		"price": {
			Enabled: true,
			Periods: []string{"4h", "1d", "15m"},
		},
	}
	got := CollectWalkForwardMTFPeriods(navs, "15m")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 HTF periods (4h, 1d)", len(got))
	}
}

func TestWalkForwardMTFTracker_StepFunctionNoLookAhead(t *testing.T) {
	data.ResetDBForTest(filepath.Join(t.TempDir(), "wf-mtf.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}

	symbol := "BTCUSDT"
	interval := "4h"
	step := int64(4 * 60 * 60 * 1000)
	base := int64(1_700_000_000_000)

	rows := []data.Candle{
		{OpenTime: base, CloseTime: base + step - 1, Open: 100, High: 110, Low: 90, Close: 105, Volume: 1},
		{OpenTime: base + step, CloseTime: base + 2*step - 1, Open: 105, High: 115, Low: 95, Close: 110, Volume: 1},
		{OpenTime: base + 2*step, CloseTime: base + 3*step - 1, Open: 110, High: 120, Low: 100, Close: 115, Volume: 1},
	}
	if err := data.SaveKlines(symbol, interval, rows); err != nil {
		t.Fatal(err)
	}

	provider := exchange.NewHTFProvider()
	ui := NavigatorUISettings{
		Enabled: true,
		Source:  navigatorSourcePrice,
		UseLong: true,
		LongLen: 10,
	}
	tracker := NewWalkForwardMTFTracker(provider, symbol, "15m", ui, []string{interval})
	tracker.SetChartStartMs(base)
	tracker.Prefetch()

	afterBar2Sec := (base + 2*step - 1) / 1000
	tracker.Update(afterBar2Sec, nil)
	st := tracker.GetState(interval)
	if st == nil {
		t.Fatal("expected HTF state after first boundary")
	}
	if st.CandleCount != 2 {
		t.Fatalf("CandleCount = %d, want 2 strictly closed 4h bars", st.CandleCount)
	}

	// Ten ticks at the same simulation second must not advance HTF state.
	for i := 0; i < 10; i++ {
		tracker.Update(afterBar2Sec, nil)
	}
	stRepeat := tracker.GetState(interval)
	if stRepeat.CandleCount != 2 {
		t.Fatalf("repeated intra-boundary updates advanced CandleCount to %d", stRepeat.CandleCount)
	}
	if stRepeat.LastUpdateSec != st.LastUpdateSec {
		t.Fatalf("LastUpdateSec changed on duplicate updates: %d -> %d", st.LastUpdateSec, stRepeat.LastUpdateSec)
	}

	afterBar3Sec := (base + 3*step - 1) / 1000
	tracker.Update(afterBar3Sec, nil)
	st3 := tracker.GetState(interval)
	if st3.CandleCount != 3 {
		t.Fatalf("CandleCount = %d, want 3 after third 4h close", st3.CandleCount)
	}
}

func TestHtfPrefetchStartMs(t *testing.T) {
	t.Parallel()

	nowMs := time.Now().UnixMilli()
	intervalMs := int64(4 * 60 * 60 * 1000)
	chartStart := nowMs - intervalMs*5

	got := htfPrefetchStartMs(chartStart, "4h")
	wantEarliest := nowMs - int64(MinHTFPrefetchBars)*intervalMs
	if got > wantEarliest+intervalMs || got < wantEarliest-intervalMs {
		t.Fatalf("htfPrefetchStartMs = %d, want near %d", got, wantEarliest)
	}

	pastChart := nowMs - int64(MinHTFPrefetchBars+50)*intervalMs
	if early := htfPrefetchStartMs(pastChart, "4h"); early != pastChart {
		t.Fatalf("deep chart start preserved: got %d want %d", early, pastChart)
	}
}

func TestMarker_SetCurrentMTFState(t *testing.T) {
	t.Parallel()

	m := NewMarker(nil, nil, "15m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	m.SetCurrentMTFState(map[string]*HTFState{
		"4h": {
			Interval:   "4h",
			TrendLines: []NavigatorLineDTO{{Interval: "4h", X1: 1, Y1: 100}},
		},
	})

	st := m.MTFState("4h")
	if st == nil || len(st.TrendLines) != 1 {
		t.Fatalf("MTFState = %+v, want one trendline", st)
	}
	all := m.MTFStates()
	if len(all) != 1 {
		t.Fatalf("MTFStates len = %d, want 1", len(all))
	}
}
