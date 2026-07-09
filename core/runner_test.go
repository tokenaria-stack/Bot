package core_test

import (
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/core/nodes"
)

func TestDAGIntraBarImmunity(t *testing.T) {
	const warmupBars = 50
	const eps = 1e-9

	runWarmup := func(runner *core.DAGRunner, startPrice float64) float64 {
		price := startPrice
		for i := 0; i < warmupBars; i++ {
			price += 0.05
			if i%7 == 0 {
				price -= 0.2
			}
			o := price - 0.1
			h := price + 0.2
			l := price - 0.2
			v := 1000.0 + float64(i)*10
			runner.TickUpdate(o, h, l, price, v, i, true)
		}
		return price
	}

	poisonBus := core.NewBus(128)
	poisonRunner := core.NewDAGRunner(poisonBus)
	poisonRSX := nodes.NewRSXNode(14, 9, "close")
	poisonWoz := nodes.NewWozduhNode()
	poisonRunner.AddNode(poisonRSX)
	poisonRunner.AddNode(poisonWoz)

	price := runWarmup(poisonRunner, 100.0)
	sealedJurik := poisonBus.Cur.Get(core.SlotJurikRSX)
	sealedWt11 := poisonBus.Cur.Get(core.SlotWozduhFast)
	histJurik := poisonBus.Hist.Get(core.SlotJurikRSX, 1)

	wildCloses := []float64{price + 50, price - 40, price + 100, price - 80, price + 200}
	for _, wc := range wildCloses {
		poisonRunner.TickUpdate(wc-0.1, wc+5, wc-5, wc, 99999, warmupBars, false)

		if got := poisonBus.Hist.Get(core.SlotJurikRSX, 1); math.Abs(got-histJurik) > eps {
			t.Fatalf("history ring poisoned on open bar: got %v want %v", got, histJurik)
		}
	}

	poisonRSX.RestoreState()
	poisonWoz.RestoreState()
	if math.Abs(poisonRSX.JurikValue()-sealedJurik) > eps {
		t.Fatalf("jurik internal poison: restored=%v sealed=%v", poisonRSX.JurikValue(), sealedJurik)
	}
	if math.Abs(poisonWoz.Wt11Value()-sealedWt11) > eps {
		t.Fatalf("wt11 EMA internal poison: restored=%v sealed=%v", poisonWoz.Wt11Value(), sealedWt11)
	}

	bar51Close := price + 0.05
	poisonRunner.TickUpdate(bar51Close-0.1, bar51Close+0.2, bar51Close-0.2, bar51Close, 2000, warmupBars, true)

	refBus := core.NewBus(128)
	refRunner := core.NewDAGRunner(refBus)
	refRunner.AddNode(nodes.NewRSXNode(14, 9, "close"))
	refRunner.AddNode(nodes.NewWozduhNode())
	_ = runWarmup(refRunner, 100.0)
	refRunner.TickUpdate(bar51Close-0.1, bar51Close+0.2, bar51Close-0.2, bar51Close, 2000, warmupBars, true)

	checks := []struct {
		name string
		got  float64
		want float64
	}{
		{"jurik_rsx", poisonBus.Cur.Get(core.SlotJurikRSX), refBus.Cur.Get(core.SlotJurikRSX)},
		{"jurik_signal", poisonBus.Cur.Get(core.SlotJurikSignal), refBus.Cur.Get(core.SlotJurikSignal)},
		{"woz_fast", poisonBus.Cur.Get(core.SlotWozduhFast), refBus.Cur.Get(core.SlotWozduhFast)},
		{"woz_slow", poisonBus.Cur.Get(core.SlotWozduhSlow), refBus.Cur.Get(core.SlotWozduhSlow)},
	}
	for _, c := range checks {
		if math.Abs(c.got-c.want) > 1e-6 {
			t.Fatalf("%s mismatch after wild ticks: poison=%v reference=%v", c.name, c.got, c.want)
		}
	}
}
