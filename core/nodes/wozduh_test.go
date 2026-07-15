package nodes_test

import (
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/strategy"
)

const wozduhParityEps = 1e-6

func volCrossCode(marker string) float64 {
	switch marker {
	case "lime":
		return 1.0
	case "red":
		return -1.0
	default:
		return 0.0
	}
}

func TestWozduh_GoldenParity(t *testing.T) {
	t.Parallel()

	falcon := strategy.NewFalconEngine()
	bus := core.NewBus(256)
	node := nodes.NewWozduhNode()
	node.Init(bus)

	const bars = 150
	for i := 0; i < bars; i++ {
		// Synthetic trending OHLCV with mild noise (deterministic).
		base := 100.0 + float64(i)*0.35 + math.Sin(float64(i)/7)*1.2
		open := base
		high := base + 0.8 + math.Mod(float64(i), 5)*0.15
		low := base - 0.7 - math.Mod(float64(i*3), 4)*0.1
		close := base + math.Cos(float64(i)/5)*0.55
		volume := 1000.0 + float64(i)*3.5 + math.Mod(float64(i*11), 50)

		sig := falcon.Evaluate(high, low, close, volume)

		bus.Cur.Set(core.SlotPriceOpen, open)
		bus.Cur.Set(core.SlotPriceHigh, high)
		bus.Cur.Set(core.SlotPriceLow, low)
		bus.Cur.Set(core.SlotPriceClose, close)
		bus.Cur.Set(core.SlotVolume, volume)
		node.Update()

		checks := []struct {
			name string
			got  float64
			want float64
		}{
			{"RsiPrice", bus.Cur.Get(core.SlotWozduhRsiPrice), sig.RsiPrice},
			{"EmaRsi", bus.Cur.Get(core.SlotWozduhEmaRsi), sig.EmaRsi},
			{"RsiRsi", bus.Cur.Get(core.SlotWozduhRsiRsi), sig.RsiRsi},
			{"RsiHl2", bus.Cur.Get(core.SlotWozduhRsiHl2), sig.RsiHl2},
			{"MacdRsi", bus.Cur.Get(core.SlotWozduhMacdRsi), sig.MacdRsi},
			{"wt11", bus.Cur.Get(core.SlotWozduhFast), sig.RsiVolFast},
			{"wt22", bus.Cur.Get(core.SlotWozduhSlow), sig.RsiVolSlow},
			{"RsiAd", bus.Cur.Get(core.SlotWozduhRsiAd), sig.RsiAd},
			{"RsiHl2Vol", bus.Cur.Get(core.SlotWozduhRsiHl2Vol), sig.RsiHl2Vol},
			{"VolChanMid", bus.Cur.Get(core.SlotWozduhVolChanMid), sig.VolChanMid},
			{"VolChanUp", bus.Cur.Get(core.SlotWozduhVolChanUp), sig.VolChanUp},
			{"VolChanDn", bus.Cur.Get(core.SlotWozduhVolChanDn), sig.VolChanDn},
			{"PriceChanMid", bus.Cur.Get(core.SlotWozduhPriceChanMid), sig.PriceChanMid},
			{"PriceChanUp", bus.Cur.Get(core.SlotWozduhPriceChanUp), sig.PriceChanUp},
			{"PriceChanDn", bus.Cur.Get(core.SlotWozduhPriceChanDn), sig.PriceChanDn},
			{"VolCross", bus.Cur.Get(core.SlotWozduhVolCross), volCrossCode(sig.VolCrossMarker)},
		}
		for _, c := range checks {
			if math.Abs(c.got-c.want) > wozduhParityEps {
				t.Fatalf("bar %d %s: got %v want %v (Δ=%v)", i, c.name, c.got, c.want, c.got-c.want)
			}
		}
	}
}
