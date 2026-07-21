package market

import (
	"fmt"
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/exchange"
)

// Phase 1 (#67 Warmup-first): prove WarmupTrap and same-depth tip identity.
// No production boot-depth changes in this file.

const (
	tipSSOTDeepBars     = 3000
	tipSSOTShallowBars  = 400 // today's FrameBootKlineLimit
	tipSSOTTailCompare  = 50
	tipSSOTVisualThresh = 1e-2 // notch-scale; below = "weak trap"
	tipSSOTIdentityEps  = 1e-9
)

func tipSSOTRSX() RSXSettings {
	return NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
}

func tipSlots3FromHist(hist *core.HistoryBus) (rsx, sig, woz float64) {
	if hist == nil || hist.Count() < 1 {
		return math.NaN(), math.NaN(), math.NaN()
	}
	return hist.Get(core.SlotJurikRSX, 1), hist.Get(core.SlotJurikSignal, 1), hist.Get(core.SlotWozduhRsiPrice, 1)
}

func replayTip3(klines []exchange.Kline, rsx RSXSettings) (rsxTip, sigTip, wozTip float64) {
	return tipSlots3FromHist(ReplayDAGKlines(klines, rsx))
}

func frameTip3(frame *Frame) (rsx, sig, woz float64) {
	if frame == nil {
		return math.NaN(), math.NaN(), math.NaN()
	}
	dag := frame.DAGTickFrame()
	if dag == nil {
		return math.NaN(), math.NaN(), math.NaN()
	}
	return dag.Get(core.SlotJurikRSX), dag.Get(core.SlotJurikSignal), dag.Get(core.SlotWozduhRsiPrice)
}

func newFrameWithRSX(klines []exchange.Kline, rsx RSXSettings) *Frame {
	f := NewFrame(klines, "1m", testChaos())
	f.SetRSXSettings(rsx)
	f.ReapplyRSXSettings()
	return f
}

// maxAbsLookbackDelta compares Hist lookbacks 1..k (1 = tip) between two buses.
func maxAbsLookbackDelta(a, b *core.HistoryBus, slot core.Slot, k int) float64 {
	if a == nil || b == nil {
		return math.Inf(1)
	}
	max := 0.0
	n := k
	if a.Count() < n {
		n = a.Count()
	}
	if b.Count() < n {
		n = b.Count()
	}
	for lb := 1; lb <= n; lb++ {
		d := math.Abs(a.Get(slot, lb) - b.Get(slot, lb))
		if d > max {
			max = d
		}
	}
	return max
}

// TestTipSSOT_WarmupTrap_Replay400vs3000 — Phase 1 diagnostic.
// Same tip bar, different left context (today's boot depth vs history depth).
func TestTipSSOT_WarmupTrap_Replay400vs3000(t *testing.T) {
	rsxCfg := tipSSOTRSX()
	full := synthOHLCV(tipSSOTDeepBars)
	shallow := full[len(full)-tipSSOTShallowBars:]

	rsxDeep, sigDeep, wozDeep := replayTip3(full, rsxCfg)
	rsxShallow, sigShallow, wozShallow := replayTip3(shallow, rsxCfg)

	dRSX := math.Abs(rsxDeep - rsxShallow)
	dSig := math.Abs(sigDeep - sigShallow)
	dWoz := math.Abs(wozDeep - wozShallow)

	histDeep := ReplayDAGKlines(full, rsxCfg)
	histShallow := ReplayDAGKlines(shallow, rsxCfg)
	maxRSX50 := maxAbsLookbackDelta(histDeep, histShallow, core.SlotJurikRSX, tipSSOTTailCompare)
	maxSig50 := maxAbsLookbackDelta(histDeep, histShallow, core.SlotJurikSignal, tipSSOTTailCompare)
	maxWoz50 := maxAbsLookbackDelta(histDeep, histShallow, core.SlotWozduhRsiPrice, tipSSOTTailCompare)

	fmt.Printf("\n=== #67 PHASE1 WARMUP TRAP: Replay(%d) vs Replay(%d) ===\n", tipSSOTDeepBars, tipSSOTShallowBars)
	fmt.Printf("tip OHLC Close=%.6f OpenTime=%d\n", full[len(full)-1].Close, full[len(full)-1].OpenTime)
	fmt.Printf("RSX     deep=%.10f  shallow=%.10f  |Δtip|=%.10f  max|Δ|@%d=%.10f\n",
		rsxDeep, rsxShallow, dRSX, tipSSOTTailCompare, maxRSX50)
	fmt.Printf("Signal  deep=%.10f  shallow=%.10f  |Δtip|=%.10f  max|Δ|@%d=%.10f\n",
		sigDeep, sigShallow, dSig, tipSSOTTailCompare, maxSig50)
	fmt.Printf("Woz     deep=%.10f  shallow=%.10f  |Δtip|=%.10f  max|Δ|@%d=%.10f\n",
		wozDeep, wozShallow, dWoz, tipSSOTTailCompare, maxWoz50)
	fmt.Printf("visual threshold=%.4f (notch-scale)\n", tipSSOTVisualThresh)

	if math.IsNaN(rsxDeep) || math.IsNaN(rsxShallow) || math.IsNaN(wozDeep) || math.IsNaN(wozShallow) {
		t.Fatal("NaN tip — Replay failed")
	}

	if dRSX > tipSSOTVisualThresh || maxRSX50 > tipSSOTVisualThresh {
		fmt.Printf("VERDICT: WARMUP TRAP CONFIRMED — proceed to Phase 2 DAGInitKlineLimit.\n")
		t.Logf("WARMUP TRAP CONFIRMED |ΔRSX tip|=%.6f max@%d=%.6f", dRSX, tipSSOTTailCompare, maxRSX50)
	} else if dRSX > tipSSOTIdentityEps {
		fmt.Printf("VERDICT: WARMUP TRAP WEAK (sub-visual) — still unify init depth; watch commit path.\n")
		t.Logf("WARMUP TRAP WEAK |ΔRSX tip|=%.6f", dRSX)
	} else {
		fmt.Printf("VERDICT: depth-only Δ ~0 — warmup alone does not explain tip; Phase 3 commit path.\n")
		t.Logf("WARMUP TRAP ABSENT |ΔRSX tip|=%.6e", dRSX)
	}
	fmt.Printf("===============================================================\n\n")
}

// TestTipSSOT_SameDepth_FrameVsReplay — when init depth matches, Frame tip ≡ Replay tip.
func TestTipSSOT_SameDepth_FrameVsReplay(t *testing.T) {
	rsxCfg := tipSSOTRSX()
	full := synthOHLCV(tipSSOTDeepBars)

	rsxRep, sigRep, wozRep := replayTip3(full, rsxCfg)
	frame := newFrameWithRSX(full, rsxCfg)
	rsxFr, sigFr, wozFr := frameTip3(frame)

	fmt.Printf("\n=== #67 PHASE1 SAME-DEPTH: Frame(%d) vs ReplayDAG(%d) ===\n", tipSSOTDeepBars, tipSSOTDeepBars)
	fmt.Printf("RSX     Frame=%.12f  Replay=%.12f  |Δ|=%.12e\n", rsxFr, rsxRep, math.Abs(rsxFr-rsxRep))
	fmt.Printf("Signal  Frame=%.12f  Replay=%.12f  |Δ|=%.12e\n", sigFr, sigRep, math.Abs(sigFr-sigRep))
	fmt.Printf("Woz     Frame=%.12f  Replay=%.12f  |Δ|=%.12e\n", wozFr, wozRep, math.Abs(wozFr-wozRep))
	fmt.Printf("eps=%.0e\n", tipSSOTIdentityEps)

	if math.IsNaN(rsxFr) || math.IsNaN(rsxRep) {
		t.Fatal("NaN tip")
	}
	if math.Abs(rsxFr-rsxRep) > tipSSOTIdentityEps ||
		math.Abs(sigFr-sigRep) > tipSSOTIdentityEps ||
		math.Abs(wozFr-wozRep) > tipSSOTIdentityEps {
		fmt.Printf("VERDICT: FAIL — same depth still diverges → commit/path bug (Phase 3).\n")
		fmt.Printf("============================================================\n\n")
		t.Fatalf("same-depth tip mismatch: rsx Δ=%.12e sig Δ=%.12e woz Δ=%.12e",
			math.Abs(rsxFr-rsxRep), math.Abs(sigFr-sigRep), math.Abs(wozFr-wozRep))
	}
	fmt.Printf("VERDICT: PASS — Frame closed replay ≡ ReplayDAG at depth %d.\n", tipSSOTDeepBars)
	fmt.Printf("============================================================\n\n")
}

// TestTipSSOT_LiveContinuation_FormingThenClose — incremental live path after deep warm.
func TestTipSSOT_LiveContinuation_FormingThenClose(t *testing.T) {
	rsxCfg := tipSSOTRSX()
	full := synthOHLCV(tipSSOTDeepBars)
	n := len(full)
	prefix := full[:n-1]
	tip := full[n-1]

	frame := newFrameWithRSX(prefix, rsxCfg)

	// Forming ticks on tip bar (intra-bar OHLC mutation), then close with final OHLC.
	mid := tip
	mid.High = tip.High + 1.5
	mid.Low = tip.Low - 1.5
	mid.Close = tip.Open + 0.2
	frame.UpdateKlineTick(mid, false)
	frame.UpdateKlineTick(tip, false)
	frame.UpdateKlineTick(tip, true)

	rsxLive, sigLive, wozLive := frameTip3(frame)
	rsxRep, sigRep, wozRep := replayTip3(full, rsxCfg)

	fmt.Printf("\n=== #67 PHASE1 LIVE CONTINUATION (warm %d, forming→close) ===\n", tipSSOTDeepBars)
	fmt.Printf("RSX     Live=%.12f  Replay=%.12f  |Δ|=%.12e\n", rsxLive, rsxRep, math.Abs(rsxLive-rsxRep))
	fmt.Printf("Signal  Live=%.12f  Replay=%.12f  |Δ|=%.12e\n", sigLive, sigRep, math.Abs(sigLive-sigRep))
	fmt.Printf("Woz     Live=%.12f  Replay=%.12f  |Δ|=%.12e\n", wozLive, wozRep, math.Abs(wozLive-wozRep))

	if math.IsNaN(rsxLive) || math.IsNaN(rsxRep) {
		t.Fatal("NaN tip")
	}
	if math.Abs(rsxLive-rsxRep) > tipSSOTIdentityEps ||
		math.Abs(sigLive-sigRep) > tipSSOTIdentityEps ||
		math.Abs(wozLive-wozRep) > tipSSOTIdentityEps {
		fmt.Printf("VERDICT: FAIL — after-close live ≠ Replay (Snapshot/commit).\n")
		fmt.Printf("================================================================\n\n")
		t.Fatalf("live continuation tip mismatch: rsx Δ=%.12e sig Δ=%.12e woz Δ=%.12e",
			math.Abs(rsxLive-rsxRep), math.Abs(sigLive-sigRep), math.Abs(wozLive-wozRep))
	}
	fmt.Printf("VERDICT: PASS — forming→close settles to Replay tip.\n")
	fmt.Printf("================================================================\n\n")
}

// TestTipSSOT_ShallowFrameVsDeepReplay — production asymmetry today (boot 400 vs history 3000).
// Documents the cliff users see at tip handoff; not a hard fail (Phase 2 removes it).
func TestTipSSOT_ShallowFrameVsDeepReplay(t *testing.T) {
	rsxCfg := tipSSOTRSX()
	full := synthOHLCV(tipSSOTDeepBars)
	shallow := full[len(full)-tipSSOTShallowBars:]

	rsxRep, _, wozRep := replayTip3(full, rsxCfg)
	frame := newFrameWithRSX(shallow, rsxCfg)
	rsxFr, _, wozFr := frameTip3(frame)

	dRSX := math.Abs(rsxFr - rsxRep)
	dWoz := math.Abs(wozFr - wozRep)

	fmt.Printf("\n=== #67 PHASE1 PROD ASYMMETRY: Frame(boot=%d) vs Replay(%d) ===\n",
		tipSSOTShallowBars, tipSSOTDeepBars)
	fmt.Printf("RSX  Frame=%.10f  Replay=%.10f  |Δ|=%.10f\n", rsxFr, rsxRep, dRSX)
	fmt.Printf("Woz  Frame=%.10f  Replay=%.10f  |Δ|=%.10f\n", wozFr, wozRep, dWoz)
	if dRSX > tipSSOTVisualThresh {
		fmt.Printf("VERDICT: PROD CLIFF VISIBLE — FrameBoot %d vs History %d explains notch-scale tip.\n",
			tipSSOTShallowBars, tipSSOTDeepBars)
	} else {
		fmt.Printf("VERDICT: prod |Δ| below visual thresh — still unify DAGInitKlineLimit.\n")
	}
	fmt.Printf("====================================================================\n\n")
	t.Logf("Frame(%d) vs Replay(%d) |ΔRSX|=%.6f |ΔWoz|=%.6f", tipSSOTShallowBars, tipSSOTDeepBars, dRSX, dWoz)
}
