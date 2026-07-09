package nodes

import (
	"math"
	"testing"

	"trading_bot/core"
)

func TestScoreNodeWeightedSum(t *testing.T) {
	bus := core.NewBus(64)
	n := NewScoreNode(ScoreConfig{
		Weights: map[core.Slot]float64{
			core.SlotDivScore:      1.0,
			core.SlotMicroDivScore: 0.5,
		},
	})
	n.Init(bus)
	bus.Cur.Set(core.SlotDivScore, 15)
	bus.Cur.Set(core.SlotMicroDivScore, 20)
	n.Update()

	got := bus.Cur.Get(core.SlotTotalScore)
	want := 15.0 + 20.0*0.5
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("total score: got %v want %v", got, want)
	}
}

func TestScoreNodeNaNTreatedAsZero(t *testing.T) {
	bus := core.NewBus(64)
	n := NewScoreNode(ScoreConfig{
		Weights: map[core.Slot]float64{core.SlotDivScore: 1.0},
	})
	n.Init(bus)
	bus.Cur.Set(core.SlotDivScore, math.NaN())
	n.Update()
	if bus.Cur.Get(core.SlotTotalScore) != 0 {
		t.Fatalf("expected NaN slot to contribute 0")
	}
}
