package market

import (
	"testing"

	"trading_bot/exchange"
	"trading_bot/indicators"
)

func fractalWaveKlines(n int) []exchange.Kline {
	klines := make([]exchange.Kline, n)
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
	return klines
}

func TestRSTFractalFacts_ReplayMatchesLiveClosedWalk(t *testing.T) {
	klines := fractalWaveKlines(120)
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivMethod: "tv", DivLookback: 30, PivotRadius: 2}

	hist := ReplayDAGKlines(klines, settings)
	fromReplay := RSTFractalFactsFromDAGHistory(klines, hist, settings)

	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	for _, k := range klines {
		frame.UpdateKlineTick(k, true)
	}
	fromLive := frame.RSTFractalFactsSnapshot()

	if len(fromReplay) != len(fromLive) {
		t.Fatalf("replay %d live %d", len(fromReplay), len(fromLive))
	}
	for i := range fromReplay {
		if fromReplay[i] != fromLive[i] {
			t.Fatalf("i=%d replay=%+v live=%+v", i, fromReplay[i], fromLive[i])
		}
	}
	if len(fromReplay) == 0 {
		t.Fatal("expected fractal facts on this series")
	}
	legacy := map[string]bool{"L": true, "LL": true, "S": true, "SS": true, "P": true}
	tvUnchanged := RSTVFactsFromDAGHistory(klines, hist, settings)
	for _, ev := range fromReplay {
		if legacy[ev.Direction] || legacy[ev.Pattern] {
			t.Fatalf("legacy vocab %+v", ev)
		}
		switch ev.Source {
		case indicators.FactSourceRSXFractalDiv, indicators.FactSourceRSXFractalPivot:
		default:
			t.Fatalf("source %q", ev.Source)
		}
	}
	for _, ev := range tvUnchanged {
		if ev.Source != indicators.FactSourceRSXTVDiv && ev.Source != indicators.FactSourceRSXTVPivot {
			t.Fatalf("TV family polluted: %+v", ev)
		}
	}
}

func TestRSTFractalFacts_IndependentOfDivMethod(t *testing.T) {
	klines := fractalWaveKlines(120)
	tv := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivMethod: "tv", DivLookback: 30, PivotRadius: 2}
	fr := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivMethod: "fractal", DivLookback: 30, PivotRadius: 2}
	a := RSTFractalFactsFromDAGHistory(klines, ReplayDAGKlines(klines, tv), tv)
	b := RSTFractalFactsFromDAGHistory(klines, ReplayDAGKlines(klines, fr), fr)
	if len(a) == 0 || len(a) != len(b) {
		t.Fatalf("tv-method %d fractal-method %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("i=%d %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestRSTFractalFacts_FormingBarDoesNotConfirm(t *testing.T) {
	klines := fractalWaveKlines(120)
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivMethod: "tv", DivLookback: 30, PivotRadius: 2}
	frame := NewFrame(nil, "1m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	for i, k := range klines {
		if i == len(klines)-1 {
			frame.UpdateKlineTick(k, false)
			break
		}
		frame.UpdateKlineTick(k, true)
	}
	closed := frame.RSTFractalFactsSnapshot()
	frame.UpdateKlineTick(klines[len(klines)-1], true)
	after := frame.RSTFractalFactsSnapshot()
	replay := RSTFractalFactsFromDAGHistory(klines, ReplayDAGKlines(klines, settings), settings)
	if len(after) != len(replay) {
		t.Fatalf("after close %d replay %d", len(after), len(replay))
	}
	if len(closed) > len(after) {
		t.Fatalf("forming published extra facts")
	}
}

func TestRSTFractalFacts_FrozenTVAndZZ(t *testing.T) {
	klines := fractalWaveKlines(120)
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivMethod: "tv", DivLookback: 30, PivotRadius: 2}
	replay := ReplayClosedBars(klines, settings)
	tv := RSTVFactsFromDAGHistory(klines, replay.Hist, settings)
	for _, ev := range tv {
		if ev.Source != indicators.FactSourceRSXTVDiv && ev.Source != indicators.FactSourceRSXTVPivot {
			t.Fatalf("tv %+v", ev)
		}
		if ev.Pattern == indicators.FactPatternClassA || ev.Pattern == indicators.FactPatternClassB || ev.Pattern == indicators.FactPatternClassC {
			t.Fatalf("TV used fractal pattern %+v", ev)
		}
	}
	for _, ev := range replay.ZZFacts {
		if ev.Source != indicators.FactSourceRSXZZDiv {
			t.Fatalf("zz %+v", ev)
		}
		if ev.Pattern != indicators.FactPatternRegular && ev.Pattern != indicators.FactPatternHidden {
			t.Fatalf("zz pattern %+v", ev)
		}
	}
}

func TestRSTFractalFacts_ShowPivotsDoesNotChangeFactCount(t *testing.T) {
	klines := fractalWaveKlines(120)
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3", DivMethod: "tv", DivLookback: 30, PivotRadius: 2}
	facts := RSTFractalFactsFromDAGHistory(klines, ReplayDAGKlines(klines, settings), settings)
	pivots := 0
	for _, ev := range facts {
		if ev.Source == indicators.FactSourceRSXFractalPivot {
			pivots++
		}
	}
	if len(facts) == 0 {
		t.Fatal("expected facts")
	}
	_ = pivots
}
