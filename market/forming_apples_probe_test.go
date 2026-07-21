package market

import (
	"fmt"
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/exchange"
)

// Probe A (#67): apples-to-apples forming-tip identity.
// After each intra-bar tick, compare:
//
//	Live Cur  (incremental Restore→Update, isClosed=false)
//	vs
//	Cold Replay(prefix + current forming OHLC as closed)
//
// liveRSX≠histRSX while forming≠poison. Same OHLC on both paths ≠poison only if they diverge.
// Does not modify ADR-009 / GetWindow / FE.

const (
	formingApplesWarmBars = 500
	formingApplesEps      = 1e-9
	formingApplesVisual   = 1e-2
)

var formingApplesSlots = []core.Slot{
	core.SlotJurikRSX,
	core.SlotJurikSignal,
	core.SlotWozduhRsiPrice,
	core.SlotWozduhFast,
	core.SlotWozduhSlow,
}

type formingTickOHLC struct {
	label string
	k     exchange.Kline
}

func formingApplesRSX() RSXSettings {
	return NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
}

// coldReplayCurTips runs a fresh closed-only DAG over prefix||tip and returns Cur slots.
func coldReplayCurTips(prefix []exchange.Kline, tip exchange.Kline, rsx RSXSettings, slots []core.Slot) map[core.Slot]float64 {
	n := len(prefix) + 1
	run := newDAGRunner(n, rsx)
	for i, k := range prefix {
		run.TickUpdate(k.Open, k.High, k.Low, k.Close, k.Volume, i, true)
	}
	run.TickUpdate(tip.Open, tip.High, tip.Low, tip.Close, tip.Volume, len(prefix), true)
	cur := run.Bus().Cur
	out := make(map[core.Slot]float64, len(slots))
	for _, s := range slots {
		out[s] = cur.Get(s)
	}
	return out
}

func liveCurTips(run *core.DAGRunner, slots []core.Slot) map[core.Slot]float64 {
	cur := run.Bus().Cur
	out := make(map[core.Slot]float64, len(slots))
	for _, s := range slots {
		out[s] = cur.Get(s)
	}
	return out
}

func frameCurTips(frame *Frame, slots []core.Slot) map[core.Slot]float64 {
	out := make(map[core.Slot]float64, len(slots))
	for _, s := range slots {
		out[s] = math.NaN()
	}
	dag := frame.DAGTickFrame()
	if dag == nil {
		return out
	}
	for _, s := range slots {
		out[s] = dag.Get(s)
	}
	return out
}

func firstSlotDiverge(live, cold map[core.Slot]float64, slots []core.Slot, eps float64) (core.Slot, float64, float64, float64, bool) {
	for _, s := range slots {
		lv, cv := live[s], cold[s]
		if math.IsNaN(lv) || math.IsNaN(cv) {
			if math.IsNaN(lv) != math.IsNaN(cv) {
				return s, lv, cv, math.Inf(1), true
			}
			continue
		}
		d := math.Abs(lv - cv)
		if d > eps {
			return s, lv, cv, d, true
		}
	}
	return 0, 0, 0, 0, false
}

func slotLabel(s core.Slot) string {
	switch s {
	case core.SlotJurikRSX:
		return "JurikRSX"
	case core.SlotJurikSignal:
		return "JurikSignal"
	case core.SlotWozduhRsiPrice:
		return "WozduhRsiPrice"
	case core.SlotWozduhFast:
		return "WozduhFast"
	case core.SlotWozduhSlow:
		return "WozduhSlow"
	default:
		return fmt.Sprintf("Slot(%d)", s)
	}
}

// buildFormingTickSequence mutates one tip bar like a busy 1m: open → expand range → wander close → settle.
func buildFormingTickSequence(final exchange.Kline) []formingTickOHLC {
	ot, ct := final.OpenTime, final.CloseTime
	o := final.Open
	seq := []formingTickOHLC{
		{"t01_open", exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ct, Open: o, High: o, Low: o, Close: o, Volume: final.Volume * 0.05,
		})},
		{"t02_up", exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ct, Open: o, High: o + 0.4, Low: o - 0.1, Close: o + 0.25, Volume: final.Volume * 0.15,
		})},
		{"t03_spike", exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ct, Open: o, High: final.High + 2.0, Low: final.Low - 2.0, Close: o + 0.1, Volume: final.Volume * 0.35,
		})},
		{"t04_pull", exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ct, Open: o, High: final.High + 2.0, Low: final.Low - 2.0, Close: o - 0.3, Volume: final.Volume * 0.55,
		})},
		{"t05_mid", exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ct, Open: o, High: math.Max(final.High, o+0.5), Low: math.Min(final.Low, o-0.5), Close: (final.High + final.Low) / 2, Volume: final.Volume * 0.75,
		})},
		{"t06_near", exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ct, Open: o, High: final.High, Low: final.Low, Close: final.Close + 0.05, Volume: final.Volume * 0.9,
		})},
		{"t07_final_ohlc", final},
	}
	return seq
}

// TestFormingApples_DAGRunner_PerTick — Probe A on the DAG protocol (Restore→Update).
func TestFormingApples_DAGRunner_PerTick(t *testing.T) {
	rsxCfg := formingApplesRSX()
	full := synthOHLCV(formingApplesWarmBars)
	prefix := full[:len(full)-1]
	final := full[len(full)-1]
	ticks := buildFormingTickSequence(final)

	live := newDAGRunner(len(full), rsxCfg)
	for i, k := range prefix {
		live.TickUpdate(k.Open, k.High, k.Low, k.Close, k.Volume, i, true)
	}

	fmt.Printf("\n=== #67 PROBE A: DAG Live Cur vs Cold Replay(prefix+forming OHLC) ===\n")
	fmt.Printf("warm=%d ticks=%d eps=%.0e\n", len(prefix), len(ticks), formingApplesEps)

	var firstTick string
	var firstSlot core.Slot
	var firstLive, firstCold, firstDelta float64
	maxAbsRSX := 0.0
	allOK := true

	for i, tick := range ticks {
		live.TickUpdate(tick.k.Open, tick.k.High, tick.k.Low, tick.k.Close, tick.k.Volume, len(prefix), false)
		liveTips := liveCurTips(live, formingApplesSlots)
		coldTips := coldReplayCurTips(prefix, tick.k, rsxCfg, formingApplesSlots)

		dRSX := math.Abs(liveTips[core.SlotJurikRSX] - coldTips[core.SlotJurikRSX])
		if dRSX > maxAbsRSX {
			maxAbsRSX = dRSX
		}

		slot, lv, cv, d, bad := firstSlotDiverge(liveTips, coldTips, formingApplesSlots, formingApplesEps)
		status := "OK"
		if bad {
			status = "DIVERGE"
			allOK = false
			if firstTick == "" {
				firstTick = tick.label
				firstSlot, firstLive, firstCold, firstDelta = slot, lv, cv, d
			}
		}
		fmt.Printf("[%02d] %-14s %-7s  liveRSX=%.10f coldRSX=%.10f |ΔRSX|=%.3e  C=%.4f H=%.4f L=%.4f\n",
			i+1, tick.label, status,
			liveTips[core.SlotJurikRSX], coldTips[core.SlotJurikRSX], dRSX,
			tick.k.Close, tick.k.High, tick.k.Low)
		if bad {
			fmt.Printf("       first-bad slot=%s live=%.12f cold=%.12f |Δ|=%.12e\n", slotLabel(slot), lv, cv, d)
		}
	}

	// After-close must still match cold final.
	live.TickUpdate(final.Open, final.High, final.Low, final.Close, final.Volume, len(prefix), true)
	liveClose := liveCurTips(live, formingApplesSlots)
	coldClose := coldReplayCurTips(prefix, final, rsxCfg, formingApplesSlots)
	_, _, _, _, closeBad := firstSlotDiverge(liveClose, coldClose, formingApplesSlots, formingApplesEps)
	fmt.Printf("[close] liveRSX=%.10f coldRSX=%.10f |ΔRSX|=%.3e closeMatch=%v\n",
		liveClose[core.SlotJurikRSX], coldClose[core.SlotJurikRSX],
		math.Abs(liveClose[core.SlotJurikRSX]-coldClose[core.SlotJurikRSX]), !closeBad)

	if !allOK {
		fmt.Printf("VERDICT: FIRST DIVERGENCE at %s slot=%s |Δ|=%.6e (live=%.10f cold=%.10f)\n",
			firstTick, slotLabel(firstSlot), firstDelta, firstLive, firstCold)
		fmt.Printf("max|ΔRSX| during forming=%.6e\n", maxAbsRSX)
		fmt.Printf("================================================================\n\n")
		t.Fatalf("Probe A DAG: first diverge %s/%s Δ=%.12e", firstTick, slotLabel(firstSlot), firstDelta)
	}
	if closeBad {
		fmt.Printf("VERDICT: forming ticks OK but AFTER-CLOSE diverges — commit/Save bug.\n")
		fmt.Printf("================================================================\n\n")
		t.Fatal("Probe A DAG: after-close live ≠ cold")
	}
	fmt.Printf("VERDICT: CLEAN — Live Cur ≡ Cold Replay on same forming OHLC every tick (max|ΔRSX|=%.3e).\n", maxAbsRSX)
	fmt.Printf("NOTE: TipSSOT liveRSX≠histRSX with forming=true is apples≠oranges, not this leak.\n")
	fmt.Printf("================================================================\n\n")
}

// TestFormingApples_Frame_PerTick — Probe A on production Frame.UpdateKlineTick (ChartOnly DAG path).
func TestFormingApples_Frame_PerTick(t *testing.T) {
	prev := GetEngineMode()
	SetEngineMode(EngineModeChartOnly)
	t.Cleanup(func() { SetEngineMode(prev) })

	rsxCfg := formingApplesRSX()
	full := synthOHLCV(formingApplesWarmBars)
	prefix := full[:len(full)-1]
	final := full[len(full)-1]
	ticks := buildFormingTickSequence(final)

	frame := newFrameWithRSX(prefix, rsxCfg)

	fmt.Printf("\n=== #67 PROBE A: Frame.UpdateKlineTick vs Cold Replay(prefix+forming OHLC) ===\n")
	fmt.Printf("warm=%d ticks=%d EngineMode=%s\n", len(prefix), len(ticks), GetEngineMode())

	var firstTick string
	var firstSlot core.Slot
	var firstLive, firstCold, firstDelta float64
	maxAbsRSX := 0.0
	allOK := true

	for i, tick := range ticks {
		frame.UpdateKlineTick(tick.k, false)
		liveTips := frameCurTips(frame, formingApplesSlots)
		coldTips := coldReplayCurTips(prefix, tick.k, rsxCfg, formingApplesSlots)

		dRSX := math.Abs(liveTips[core.SlotJurikRSX] - coldTips[core.SlotJurikRSX])
		if dRSX > maxAbsRSX {
			maxAbsRSX = dRSX
		}

		slot, lv, cv, d, bad := firstSlotDiverge(liveTips, coldTips, formingApplesSlots, formingApplesEps)
		status := "OK"
		if bad {
			status = "DIVERGE"
			allOK = false
			if firstTick == "" {
				firstTick = tick.label
				firstSlot, firstLive, firstCold, firstDelta = slot, lv, cv, d
			}
		}
		fmt.Printf("[%02d] %-14s %-7s  liveRSX=%.10f coldRSX=%.10f |ΔRSX|=%.3e\n",
			i+1, tick.label, status,
			liveTips[core.SlotJurikRSX], coldTips[core.SlotJurikRSX], dRSX)
		if bad {
			fmt.Printf("       first-bad slot=%s live=%.12f cold=%.12f |Δ|=%.12e\n", slotLabel(slot), lv, cv, d)
		}
	}

	frame.UpdateKlineTick(final, true)
	liveClose := frameCurTips(frame, formingApplesSlots)
	coldClose := coldReplayCurTips(prefix, final, rsxCfg, formingApplesSlots)
	_, _, _, _, closeBad := firstSlotDiverge(liveClose, coldClose, formingApplesSlots, formingApplesEps)
	fmt.Printf("[close] liveRSX=%.10f coldRSX=%.10f |ΔRSX|=%.3e closeMatch=%v\n",
		liveClose[core.SlotJurikRSX], coldClose[core.SlotJurikRSX],
		math.Abs(liveClose[core.SlotJurikRSX]-coldClose[core.SlotJurikRSX]), !closeBad)

	// Document the TipSSOT apples≠oranges gap: forming Cur vs closed-prefix-only Replay tip.
	closedOnlyRSX, _ := replayTip(prefix, rsxCfg)
	formingCur := liveClose // after close equals cold; use last forming before close for doc
	_ = formingCur
	preClose := ticks[len(ticks)-1]
	framePre := newFrameWithRSX(prefix, rsxCfg)
	framePre.UpdateKlineTick(preClose.k, false)
	preLive := frameCurTips(framePre, formingApplesSlots)
	vsClosedHist := math.Abs(preLive[core.SlotJurikRSX] - closedOnlyRSX)
	fmt.Printf("DOC TipSSOT trap: formingCurRSX=%.10f vs Replay(closedPrefixOnly)=%.10f |Δ|=%.6f (NOT poison if Probe A CLEAN)\n",
		preLive[core.SlotJurikRSX], closedOnlyRSX, vsClosedHist)
	if vsClosedHist > formingApplesVisual {
		fmt.Printf("DOC: |Δ| > visual threshold — explains log liveRSX≠histRSX with forming=true.\n")
	}

	if !allOK {
		fmt.Printf("VERDICT: FIRST DIVERGENCE at %s slot=%s |Δ|=%.6e (live=%.10f cold=%.10f)\n",
			firstTick, slotLabel(firstSlot), firstDelta, firstLive, firstCold)
		fmt.Printf("================================================================\n\n")
		t.Fatalf("Probe A Frame: first diverge %s/%s Δ=%.12e", firstTick, slotLabel(firstSlot), firstDelta)
	}
	if closeBad {
		fmt.Printf("VERDICT: AFTER-CLOSE Frame ≠ cold — commit path bug.\n")
		fmt.Printf("================================================================\n\n")
		t.Fatal("Probe A Frame: after-close live ≠ cold")
	}
	fmt.Printf("VERDICT: CLEAN — Frame forming Cur ≡ Cold Replay on same OHLC every tick (max|ΔRSX|=%.3e).\n", maxAbsRSX)
	fmt.Printf("================================================================\n\n")
}
