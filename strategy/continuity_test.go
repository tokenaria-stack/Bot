package strategy

import (
	"fmt"
	"math"
	"testing"

	"trading_bot/core"
	"trading_bot/exchange"
)

// synthOHLCV builds a deterministic oscillatory path (not flat trend) so RSX/Wozduh
// settle into non-trivial tip values. Sufficient for continuity diagnostics; not market replay.
func synthOHLCV(n int) []exchange.Kline {
	out := make([]exchange.Kline, n)
	base := int64(1_700_000_000_000)
	price := 100.0
	for i := 0; i < n; i++ {
		// Mild random-walk + periodic swings → IIR state actually converges to something interesting.
		swing := math.Sin(float64(i)/17.0)*1.8 + math.Cos(float64(i)/41.0)*0.9
		drift := 0.02
		if i%23 == 0 {
			drift = -0.55
		}
		price += drift + swing*0.08
		high := price + 0.35 + math.Abs(swing)*0.05
		low := price - 0.35 - math.Abs(swing)*0.05
		open := price - swing*0.02
		ot := base + int64(i)*60_000
		out[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime:  ot,
			CloseTime: ot + 59_999,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     price,
			Volume:    800 + float64(i%50)*12 + math.Abs(swing)*40,
		})
	}
	return out
}

func tipSlotsFromHist(hist *core.HistoryBus) (rsx, woz float64) {
	if hist == nil || hist.Count() < 1 {
		return math.NaN(), math.NaN()
	}
	return hist.Get(core.SlotJurikRSX, 1), hist.Get(core.SlotWozduhRsiPrice, 1)
}

func replayTip(klines []exchange.Kline, rsx RSXSettings) (rsxTip, wozTip float64) {
	hist := ReplayDAGKlines(klines, rsx)
	return tipSlotsFromHist(hist)
}

// TestStateContinuity_HistoryVsLiveBootDepth documents the Two Brains tip cliff:
// History REST uses a long cold Replay; Live Marker boots on AnalystBootKlineLimit=400 only.
// Same OHLCV tip bar, different internal DAG state → plot handoff spike on the UI.
//
// Pass criterion for P0 (State Handoff): |ΔRSX| > 0.5 on the shared last closed bar.
func TestStateContinuity_HistoryVsLiveBootDepth(t *testing.T) {
	const (
		histBars = 1000
		bootBars = AnalystBootKlineLimit // 400
	)
	if histBars <= bootBars {
		t.Fatalf("histBars (%d) must exceed bootBars (%d)", histBars, bootBars)
	}

	rsxCfg := NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	klines := synthOHLCV(histBars)
	tail := klines[len(klines)-bootBars:]

	// ── Path A: History (long cold Replay) ──────────────────────────────────
	rsxHist, wozHist := replayTip(klines, rsxCfg)

	// ── Path B: Live Boot (short cold Replay — AnalystBootKlineLimit) ───────
	rsxBoot, wozBoot := replayTip(tail, rsxCfg)

	dRSX := rsxHist - rsxBoot
	dWoz := wozHist - wozBoot
	absRSX := math.Abs(dRSX)
	absWoz := math.Abs(dWoz)

	fmt.Printf("\n=== STATE CONTINUITY DIAGNOSTIC (Two Brains) ===\n")
	fmt.Printf("OHLCV tip Close=%.6f OpenTime=%d (shared last closed bar)\n",
		klines[len(klines)-1].Close, klines[len(klines)-1].OpenTime)
	fmt.Printf("History depth=%d | Live Boot depth=%d (AnalystBootKlineLimit)\n", histBars, bootBars)
	fmt.Printf("---\n")
	fmt.Printf("RSX  History=%.8f  LiveBoot=%.8f  Δ=%+.8f  |Δ|=%.8f\n", rsxHist, rsxBoot, dRSX, absRSX)
	fmt.Printf("Woz  History=%.8f  LiveBoot=%.8f  Δ=%+.8f  |Δ|=%.8f  (SlotWozduhRsiPrice)\n", wozHist, wozBoot, dWoz, absWoz)
	fmt.Printf("---\n")
	const threshold = 0.5
	if absRSX > threshold {
		fmt.Printf("VERDICT: HYPOTHESIS CONFIRMED — |ΔRSX|=%.4f > %.1f → P0 State Continuity required.\n", absRSX, threshold)
	} else {
		fmt.Printf("VERDICT: DEPTH-ONLY MISMATCH SMALL — |ΔRSX|=%.4f ≤ %.1f.\n", absRSX, threshold)
		fmt.Printf("         Warmup depth alone does not explain TV-scale tip cliffs; check handoff semantics / data plane next.\n")
	}
	fmt.Printf("=================================================\n\n")

	if math.IsNaN(rsxHist) || math.IsNaN(rsxBoot) {
		t.Fatal("RSX tip NaN — runners failed to settle")
	}
	if math.IsNaN(wozHist) || math.IsNaN(wozBoot) {
		t.Fatal("Wozduh tip NaN — runners failed to settle")
	}

	// Soft signal (not a hard fail): document for tech lead. Hard assert only on NaN / plumbing.
	t.Logf("ΔRSX=%.6f ΔWoz=%.6f (threshold %.1f for P0)", absRSX, absWoz, threshold)
}

// TestStateContinuity_DeterminismControl — same input twice must match ε (engine sanity).
func TestStateContinuity_DeterminismControl(t *testing.T) {
	t.Parallel()
	rsxCfg := NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	klines := synthOHLCV(600)

	a1, w1 := replayTip(klines, rsxCfg)
	a2, w2 := replayTip(klines, rsxCfg)

	const eps = 1e-12
	if math.Abs(a1-a2) > eps || math.Abs(w1-w2) > eps {
		t.Fatalf("non-deterministic Replay: rsx %.12f vs %.12f | woz %.12f vs %.12f", a1, a2, w1, w2)
	}
	fmt.Printf("Determinism OK: RSX=%.8f Woz=%.8f (two identical Replays)\n", a1, w1)
}

// TestStateContinuity_DepthSweep — GPT 5-minute experiment: does tip keep drifting past 400?
func TestStateContinuity_DepthSweep(t *testing.T) {
	rsxCfg := NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	full := synthOHLCV(3000)
	depths := []int{400, 800, 1500, 3000}

	fmt.Printf("\n=== DEPTH SWEEP (same tip bar, growing left context) ===\n")
	var prevRSX float64
	for i, d := range depths {
		tail := full[len(full)-d:]
		rsx, woz := replayTip(tail, rsxCfg)
		deltaPrev := 0.0
		if i > 0 {
			deltaPrev = rsx - prevRSX
		}
		fmt.Printf("depth=%4d  RSX=%.8f  WozRsiPrice=%.8f  ΔvsPrev=%+.8f\n", d, rsx, woz, deltaPrev)
		prevRSX = rsx
	}
	fmt.Printf("=======================================================\n\n")
}

// TestStateContinuity_FormingOpenTickSemantics — Version #2 probe:
// after N-1 closed bars, evaluate tip bar as open (isClosed=false) vs closed (true).
// Same OHLC must yield identical Cur tips (Save only freezes for *next* bar).
func TestStateContinuity_FormingOpenTickSemantics(t *testing.T) {
	rsxCfg := NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	klines := synthOHLCV(500)
	n := len(klines)
	prefix := klines[:n-1]
	tip := klines[n-1]

	runClosed := newDAGRunner(n, rsxCfg)
	for i, k := range prefix {
		runClosed.TickUpdate(k.Open, k.High, k.Low, k.Close, k.Volume, i, true)
	}
	runClosed.TickUpdate(tip.Open, tip.High, tip.Low, tip.Close, tip.Volume, n-1, true)
	rsxClosed := runClosed.Bus().Cur.Get(core.SlotJurikRSX)
	wozClosed := runClosed.Bus().Cur.Get(core.SlotWozduhRsiPrice)

	runOpen := newDAGRunner(n, rsxCfg)
	for i, k := range prefix {
		runOpen.TickUpdate(k.Open, k.High, k.Low, k.Close, k.Volume, i, true)
	}
	runOpen.TickUpdate(tip.Open, tip.High, tip.Low, tip.Close, tip.Volume, n-1, false)
	rsxOpen := runOpen.Bus().Cur.Get(core.SlotJurikRSX)
	wozOpen := runOpen.Bus().Cur.Get(core.SlotWozduhRsiPrice)

	dRSX := math.Abs(rsxClosed - rsxOpen)
	dWoz := math.Abs(wozClosed - wozOpen)

	fmt.Printf("\n=== FORMING vs CLOSED (same tip OHLC) ===\n")
	fmt.Printf("RSX  closed=%.8f  open=%.8f  |Δ|=%.8f\n", rsxClosed, rsxOpen, dRSX)
	fmt.Printf("Woz  closed=%.8f  open=%.8f  |Δ|=%.8f\n", wozClosed, wozOpen, dWoz)
	if dRSX < 1e-12 && dWoz < 1e-12 {
		fmt.Printf("VERDICT: open/closed Update identical on same OHLC (Save is next-bar only). Version #2 alone ≠ tip cliff.\n")
	} else {
		fmt.Printf("VERDICT: open/closed diverge on same OHLC — Restore/Save semantics bug.\n")
	}
	fmt.Printf("=========================================\n\n")

	if dRSX > 1e-9 || dWoz > 1e-9 {
		t.Fatalf("forming vs closed tip diverge: ΔRSX=%.12f ΔWoz=%.12f", dRSX, dWoz)
	}
}

// TestStateContinuity_IntraBarMutation — live path mutates tip OHLC across ticks then closes;
// history Replay only ever sees the *final* closed OHLC. Tip before close can spike vs history.
func TestStateContinuity_IntraBarMutation(t *testing.T) {
	rsxCfg := NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	klines := synthOHLCV(500)
	n := len(klines)
	prefix := klines[:n-1]
	final := klines[n-1]

	// History: closed-only final tip.
	histRSX, histWoz := replayTip(klines, rsxCfg)

	// Live: closed prefix, then mid-bar wild high/low, then settle to final OHLC + close.
	live := newDAGRunner(n, rsxCfg)
	for i, k := range prefix {
		live.TickUpdate(k.Open, k.High, k.Low, k.Close, k.Volume, i, true)
	}
	// Intra-bar poison (typical forming): exaggerated range before final close paints.
	live.TickUpdate(final.Open, final.High+5, final.Low-5, final.Open+0.1, final.Volume, n-1, false)
	midRSX := live.Bus().Cur.Get(core.SlotJurikRSX)
	live.TickUpdate(final.Open, final.High, final.Low, final.Close, final.Volume, n-1, false)
	preCloseRSX := live.Bus().Cur.Get(core.SlotJurikRSX)
	live.TickUpdate(final.Open, final.High, final.Low, final.Close, final.Volume, n-1, true)
	endRSX := live.Bus().Cur.Get(core.SlotJurikRSX)
	endWoz := live.Bus().Cur.Get(core.SlotWozduhRsiPrice)

	fmt.Printf("\n=== INTRA-BAR MUTATION (live forming vs history final) ===\n")
	fmt.Printf("History closed tip   RSX=%.8f  Woz=%.8f\n", histRSX, histWoz)
	fmt.Printf("Live mid-bar spike   RSX=%.8f  |ΔvsHist|=%.8f\n", midRSX, math.Abs(midRSX-histRSX))
	fmt.Printf("Live pre-close       RSX=%.8f  |ΔvsHist|=%.8f\n", preCloseRSX, math.Abs(preCloseRSX-histRSX))
	fmt.Printf("Live after close     RSX=%.8f  Woz=%.8f  |ΔRSX|=%.8f\n", endRSX, endWoz, math.Abs(endRSX-histRSX))
	fmt.Printf("NOTE: mid-bar |Δ| is expected UI noise; after-close must match History (Restore+same OHLC).\n")
	fmt.Printf("=========================================================\n\n")

	if math.Abs(endRSX-histRSX) > 1e-9 || math.Abs(endWoz-histWoz) > 1e-9 {
		t.Fatalf("after-close live tip ≠ history: rsx %.12f vs %.12f | woz %.12f vs %.12f",
			endRSX, histRSX, endWoz, histWoz)
	}
}
