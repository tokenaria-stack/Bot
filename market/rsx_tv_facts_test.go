package market

import (
	"testing"

	"trading_bot/exchange"
	"trading_bot/indicators"
)

func TestRSTVFacts_ReplayMatchesLiveClosedWalk(t *testing.T) {
	klines := make([]exchange.Kline, 120)
	base := int64(1_700_000_000_000)
	for i := range klines {
		wave := float64(i%40) - 20
		px := 50000 + wave*8 + float64(i)
		klines[i] = exchange.Kline{
			OpenTime:  base + int64(i)*60_000,
			CloseTime: base + int64(i)*60_000 + 59_999,
			Open:      px,
			High:      px + 10,
			Low:       px - 10,
			Close:     px + 5,
			Volume:    100,
		}
	}
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivMethod: "tv", DivLookback: 30}

	hist := ReplayDAGKlines(klines, settings)
	fromReplay := RSTVFactsFromDAGHistory(klines, hist, settings)

	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	for _, k := range klines {
		frame.UpdateKlineTick(k, true)
	}
	fromLive := frame.RSTVFactsSnapshot()

	if len(fromReplay) != len(fromLive) {
		t.Fatalf("replay %d live %d", len(fromReplay), len(fromLive))
	}
	for i := range fromReplay {
		if fromReplay[i] != fromLive[i] {
			t.Fatalf("i=%d replay=%+v live=%+v", i, fromReplay[i], fromLive[i])
		}
	}
	if len(fromReplay) == 0 {
		t.Fatal("expected TV facts on this series")
	}
	for _, ev := range fromReplay {
		if ev.Source != indicators.FactSourceRSXTVDiv {
			t.Fatalf("source %q", ev.Source)
		}
		if ev.ConfirmedAt-ev.AnchorAt != 60_000 {
			t.Fatalf("bar delay ms=%d", ev.ConfirmedAt-ev.AnchorAt)
		}
	}
}

func TestUpdateKlineTick_ClosedWalkHistCountMatchesKlines(t *testing.T) {
	klines := make([]exchange.Kline, 40)
	base := int64(1_700_000_000_000)
	for i := range klines {
		px := 100.0 + float64(i)
		klines[i] = exchange.Kline{
			OpenTime:  base + int64(i)*60_000,
			CloseTime: base + int64(i)*60_000 + 59_999,
			Open:      px,
			High:      px + 1,
			Low:       px - 1,
			Close:     px,
			Volume:    1,
		}
	}
	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivMethod: "tv", DivLookback: 30})
	for _, k := range klines {
		frame.UpdateKlineTick(k, true)
	}
	frame.mu.Lock()
	nK := len(frame.klines)
	hist := frame.dagHistoryLocked()
	nH := 0
	if hist != nil {
		nH = hist.Count()
	}
	committed := frame.lastCommittedOpenTime
	frame.mu.Unlock()
	if nH != nK {
		t.Fatalf("hist %d klines %d", nH, nK)
	}
	if committed != klines[len(klines)-1].OpenTime {
		t.Fatalf("lastCommitted=%d want %d", committed, klines[len(klines)-1].OpenTime)
	}
}

func TestRSTVFacts_FractalMethodUnpublished(t *testing.T) {
	klines := make([]exchange.Kline, 40)
	base := int64(1_700_000_000_000)
	for i := range klines {
		px := 100.0 + float64(i)
		klines[i] = exchange.Kline{
			OpenTime:  base + int64(i)*60_000,
			CloseTime: base + int64(i)*60_000 + 59_999,
			Open:      px,
			High:      px + 1,
			Low:       px - 1,
			Close:     px,
			Volume:    1,
		}
	}
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivMethod: "fractal", DivLookback: 30, PivotRadius: 2}
	hist := ReplayDAGKlines(klines, settings)
	if got := RSTVFactsFromDAGHistory(klines, hist, settings); len(got) != 0 {
		t.Fatalf("fractal must not publish rsx_tv_div: %+v", got)
	}
	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	for _, k := range klines {
		frame.UpdateKlineTick(k, true)
	}
	if got := frame.RSTVFactsSnapshot(); len(got) != 0 {
		t.Fatalf("live fractal published TV facts: %+v", got)
	}
}

func TestRSTVFacts_FormingBarDoesNotConfirm(t *testing.T) {
	klines := make([]exchange.Kline, 80)
	base := int64(1_700_000_000_000)
	for i := range klines {
		wave := float64(i%40) - 20
		px := 50000 + wave*8 + float64(i)
		klines[i] = exchange.Kline{
			OpenTime:  base + int64(i)*60_000,
			CloseTime: base + int64(i)*60_000 + 59_999,
			Open:      px,
			High:      px + 10,
			Low:       px - 10,
			Close:     px + 5,
			Volume:    100,
		}
	}
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivMethod: "tv", DivLookback: 30}
	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	for i, k := range klines {
		frame.UpdateKlineTick(k, i < len(klines)-1)
	}
	closed := frame.RSTVFactsSnapshot()
	for _, ev := range closed {
		if ev.ConfirmedAt == klines[len(klines)-1].OpenTime {
			t.Fatal("forming tip must not confirm TV facts")
		}
	}
	frame.UpdateKlineTick(klines[len(klines)-1], true)
	after := frame.RSTVFactsSnapshot()
	replay := RSTVFactsFromDAGHistory(klines, ReplayDAGKlines(klines, settings), settings)
	if len(after) != len(replay) {
		t.Fatalf("after close live=%d replay=%d", len(after), len(replay))
	}
}
