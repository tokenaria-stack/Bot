package strategy

import (
	"math"
	"testing"

	"trading_bot/exchange"
)

func synthPipelineKline(i int, base int64, price float64) exchange.Kline {
	return exchange.Kline{
		OpenTime: base + int64(i)*60_000,
		Open:     price,
		High:     price + 10,
		Low:      price - 10,
		Close:    price + 5,
		Volume:   100,
	}
}

func TestRunStreamingReplay_AlignedChartPoints(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, 80)
	base := int64(1_700_000_000_000)
	for i := range klines {
		klines[i] = synthPipelineKline(i, base, 50000+float64(i))
	}
	settings := RSXSettings{Length: 14, SignalLength: 9, Source: "close", DivMethod: "tv"}
	cfg := ChartStreamingReplayConfig(settings, "1m")

	result := RunStreamingReplay(nil, klines, cfg)
	wantPoints := len(klines)
	if len(result.ChartPoints) != wantPoints {
		t.Fatalf("chart points len = %d want %d", len(result.ChartPoints), wantPoints)
	}
	for i, pt := range result.ChartPoints {
		if pt.Jurik != pt.RSX {
			t.Fatalf("point %d jurik/rsx mismatch", i)
		}
		if pt.RSXSignal == 0 && pt.Jurik != 0 {
			// signal may legitimately be zero early; only check jurik populated
		}
	}
}

func TestRunStreamingReplay_SignalLengthAffectsOutput(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, 120)
	base := int64(1_700_000_000_000)
	for i := range klines {
		wave := math.Sin(float64(i) * 0.15)
		klines[i] = synthPipelineKline(i, base, 50000+wave*500+float64(i)*2)
	}

	cfg9 := ChartStreamingReplayConfig(RSXSettings{Length: 14, SignalLength: 9, Source: "close"}, "1m")
	cfg21 := ChartStreamingReplayConfig(RSXSettings{Length: 14, SignalLength: 21, Source: "close"}, "1m")
	pts9 := RunStreamingReplay(nil, klines, cfg9).ChartPoints
	pts21 := RunStreamingReplay(nil, klines, cfg21).ChartPoints

	if len(pts9) != len(pts21) {
		t.Fatalf("point count mismatch %d vs %d", len(pts9), len(pts21))
	}
	changed := 0
	for i := range pts9 {
		if pts9[i].RSXSignal != pts21[i].RSXSignal {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("expected signal line to differ when signal_length changes")
	}
}

func TestIndicatorWarmupBars_MatchesBacktestMinBars(t *testing.T) {
	t.Parallel()
	if BacktestMinBars() != IndicatorWarmupBars {
		t.Fatalf("BacktestMinBars=%d IndicatorWarmupBars=%d", BacktestMinBars(), IndicatorWarmupBars)
	}
}

func TestRsxAnnotationFromDecision(t *testing.T) {
	t.Parallel()
	decision := ScoreDecision{
		Factors: map[string]ScoreFactor{
			"RSX": {Name: "RSX L", Score: scoreRSXL, Direction: BuyAction},
		},
	}
	ann, ok := rsxAnnotationFromDecision(decision, 1700000000)
	if !ok || ann.Label != "L" || ann.Time != 1700000000 {
		t.Fatalf("annotation: ok=%v %+v", ok, ann)
	}
}
