package nodes

import (
	"math"
	"testing"

	"trading_bot/core"
)

func TestMicroPatternNodeOpenBarRestore(t *testing.T) {
	bus := core.NewBus(64)
	runner := core.NewDAGRunner(bus)
	rsx := NewRSXNode(14, 9, "close")
	micro := NewMicroPatternNode(core.SlotJurikRSX)
	runner.AddNode(rsx)
	runner.AddNode(micro)

	for i := 1; i <= 3; i++ {
		p := 100.0 + float64(i)
		runner.TickUpdate(p-0.1, p+0.2, p-0.2, p, 1000, i, true)
	}

	sealedCurrent := micro.current
	sealedPrev := micro.prev
	sealedPrevPrev := micro.prevPrev

	runner.TickUpdate(200, 250, 50, 180, 5000, 4, false)
	runner.TickUpdate(300, 350, 40, 280, 6000, 4, false)

	micro.RestoreState()
	if micro.current != sealedCurrent || micro.prev != sealedPrev || micro.prevPrev != sealedPrevPrev {
		t.Fatalf("micro buffer not restored: got %v %v %v want %v %v %v",
			micro.current, micro.prev, micro.prevPrev,
			sealedCurrent, sealedPrev, sealedPrevPrev)
	}
}

func TestMicroPatternDetectSaucer(t *testing.T) {
	if !detectSaucer(22, 25, 29) {
		t.Fatal("expected saucer pattern")
	}
	if detectSaucer(35, 28, 32) {
		t.Fatal("saucer should fail above RSI threshold")
	}
}

func TestHistValueAtBarGuard(t *testing.T) {
	bus := core.NewBus(8)
	v := core.HistValueAtBar(bus, core.SlotJurikRSX, 100, 0)
	if !math.IsNaN(v) {
		t.Fatalf("expected NaN for out-of-range lookback, got %v", v)
	}
}
