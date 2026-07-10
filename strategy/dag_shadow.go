package strategy

import (
	"log/slog"
	"math"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/exchange"
)

const dagShadowEpsilon = 1e-4

const dagHistoryCap = 1024

func newDAGRunner(historyCap int, rsx RSXSettings) *core.DAGRunner {
	normalized := NormalizeRSXSettings(rsx)
	bus := core.NewBus(historyCap)
	runner := core.NewDAGRunner(bus)
	runner.AddNode(nodes.NewRSXNode(normalized.Length, normalized.SignalLength, normalized.Source))
	runner.AddNode(nodes.NewWozduhNode())
	runner.AddNode(nodes.NewZigZagNode(nodes.ZigZagConfig{
		LeftBars:  2,
		RightBars: 2,
	}))
	runner.AddNode(nodes.NewDivergenceNode(nodes.DivergenceNodeConfig{
		TargetSlot: core.SlotJurikRSX,
	}))
	runner.AddNode(nodes.NewMicroPatternNode(core.SlotJurikRSX))
	runner.AddNode(nodes.NewScoreNode(nodes.ScoreConfig{
		Weights: map[core.Slot]float64{
			core.SlotDivScore:      1.0,
			core.SlotMicroDivScore: 0.5,
		},
	}))
	return runner
}

// ReplayDAGKlines runs the DAG over closed klines and returns the populated history ring.
func ReplayDAGKlines(klines []exchange.Kline, rsx RSXSettings) *core.HistoryBus {
	if len(klines) == 0 {
		return nil
	}
	cap := core.ValidateHistoryCap(len(klines))
	runner := newDAGRunner(cap, rsx)
	for i, k := range klines {
		runner.TickUpdate(k.Open, k.High, k.Low, k.Close, k.Volume, i, true)
	}
	bus := runner.Bus()
	if bus == nil {
		return nil
	}
	return bus.Hist
}

func (a *Marker) initDAGShadowLocked() {
	a.dag = newDAGRunner(dagHistoryCap, a.effectiveRSXSettings())
}

func (a *Marker) runDAGShadowLocked(k exchange.Kline, barIndex int, isClosed bool) {
	if a.dag == nil {
		return
	}
	a.dag.TickUpdate(k.Open, k.High, k.Low, k.Close, k.Volume, barIndex, isClosed)
	if isClosed {
		a.validateDAGShadowLocked()
	}
}

func (a *Marker) validateDAGShadowLocked() {
	if a.dag == nil {
		return
	}
	bus := a.dag.Bus()
	if bus == nil || bus.Cur == nil {
		return
	}
	cur := bus.Cur
	checks := []struct {
		slot string
		got  float64
		want float64
	}{
		{"jurik_rsx", cur.Get(core.SlotJurikRSX), a.falconSignals.JurikRSX},
		{"jurik_signal", cur.Get(core.SlotJurikSignal), a.falconSignals.JurikRSXSignal},
		{"woz_fast", cur.Get(core.SlotWozduhFast), a.falconSignals.RsiVolFast},
		{"woz_slow", cur.Get(core.SlotWozduhSlow), a.falconSignals.RsiVolSlow},
	}
	for _, c := range checks {
		if !shadowValuesMatch(c.got, c.want) {
			slog.Warn("dag shadow drift",
				"slot", c.slot,
				"dag", c.got,
				"falcon", c.want,
				"delta", math.Abs(c.got-c.want),
			)
		}
	}

	slog.Debug("dag shadow total score",
		"total", cur.Get(core.SlotTotalScore),
		"macro", cur.Get(core.SlotDivScore),
		"micro", cur.Get(core.SlotMicroDivScore),
	)
}

func shadowValuesMatch(got, want float64) bool {
	if math.IsNaN(got) && math.IsNaN(want) {
		return true
	}
	return math.Abs(got-want) <= dagShadowEpsilon
}

// DAGTickFrame returns the current DAG bus frame for dual-write projection (read-only).
func (a *Marker) DAGTickFrame() *core.TickFrame {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.dag == nil {
		return nil
	}
	bus := a.dag.Bus()
	if bus == nil {
		return nil
	}
	return bus.Cur
}
