package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestRSXMarkerState_MatchesBatchScan(t *testing.T) {
	t.Parallel()

	klines := syntheticRSXKlines(120)
	falcon := NewFalconEngine()
	rsxValues := make([]float64, len(klines))
	for i, k := range klines {
		rsxValues[i] = falcon.Evaluate(k.High, k.Low, k.Close, k.Volume).JurikRSX
	}
	batch := BuildRSXChart(klines, rsxValues, RSXLookbackDefault)

	state := newRSXMarkerState(RSXLookbackDefault)
	for i, k := range klines {
		state.appendBar(k.High, k.Low, k.Close, rsxValues[i])
	}
	for i := range klines {
		got := state.markerAt(i)
		want := batch[i].Marker
		if got != want {
			t.Fatalf("bar %d marker = %q, batch = %q", i, got, want)
		}
	}
	if state.latest != batch[len(batch)-1].Marker {
		t.Fatalf("latest = %q, want %q", state.latest, batch[len(batch)-1].Marker)
	}
}

func syntheticRSXKlines(n int) []exchange.Kline {
	out := make([]exchange.Kline, n)
	price := 100.0
	for i := range out {
		wobble := float64(i%17-8) * 0.4
		price += wobble * 0.05
		high := price + 1.2
		low := price - 1.1
		out[i] = exchange.Kline{
			OpenTime: int64(i) * 60_000,
			Open:     price,
			High:     high,
			Low:      low,
			Close:    price + wobble*0.02,
			Volume:   1000 + float64(i%50)*10,
		}
	}
	return out
}
