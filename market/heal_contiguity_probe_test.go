package market

import (
	"fmt"
	"testing"

	"trading_bot/data"
	"trading_bot/exchange"
)

// TestProbe_TimelineHealFlush_PendingJumpCreatesHole — controlled recreation of the
// offline heal hole (logs only path). Simulates:
//
//	Frame Cap tip = T (14:03)
//	pending = T+2m only (14:05) — 14:04 never arrived (clearPending + Cap grace)
//	ungated applyTick flush → Frame …T, T+2m
//
// Answers GPT Q1–Q4 without live Binance.
func TestProbe_TimelineHealFlush_PendingJumpCreatesHole(t *testing.T) {
	step := int64(60_000)
	// Contiguous closed prefix ending at "14:03".
	n := 30
	closed := make([]exchange.Kline, n)
	base := int64(1_784_786_580_000) // 1784786580 sec → ChartTimeSec = 1784786580
	base = (base / step) * step
	for i := 0; i < n; i++ {
		ot := base + int64(i)*step
		p := 100.0 + float64(i)*0.1
		closed[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + step - 1,
			Open: p, High: p + 1, Low: p - 1, Close: p + 0.2, Volume: 10,
		})
	}
	capTip := closed[n-1]
	missingOT := capTip.OpenTime + step      // 14:04 — never in Frame or pending
	formingOT := capTip.OpenTime + 2*step    // 14:05 — only pending tip

	frame := NewFrame(append([]exchange.Kline{}, closed...), "1m", testChaos())
	rt := NewRuntime(map[string]*Frame{"1m": frame}, nil, nil, true, false, "BTCUSDT", "1m")

	// Disconnect clears pending (production OnBinanceDisconnect).
	rt.OnBinanceDisconnect()
	if len(rt.snapshotPendingTicks()) != 0 {
		t.Fatal("expected empty pending after disconnect clear")
	}

	// Reconnect window: only forming tip enqueued (Binance does not replay missed 14:04).
	forming := exchange.NormalizeKline(exchange.Kline{
		OpenTime: formingOT, CloseTime: formingOT + step - 1,
		Open: 110, High: 112, Low: 109, Close: 111, Volume: 40,
	})
	rt.enqueuePendingTick(exchange.WsTick{
		Timeframe: "1m",
		IsClosed:  false,
		Kline:     forming,
	})
	// Second tick same open (coalesce) — still no missingOT.
	forming.Close = 111.5
	rt.enqueuePendingTick(exchange.WsTick{
		Timeframe: "1m",
		IsClosed:  false,
		Kline:     forming,
	})

	pendingSnap := rt.snapshotPendingTicks()
	rt.logHealContiguityProbe("pre_flush", pendingSnap)

	preOpens := frameTipOpens(frame, 5)
	pendOpens := pendingTipOpens(pendingSnap, "1m")
	preVerdict := classifyHealFlushProbe(preOpens, pendOpens, "1m", "pre_flush")

	fmt.Printf("\n=== HEAL PROBE (controlled) ===\n")
	fmt.Printf("Cap tip openSec=%d\n", exchange.ChartTimeSec(capTip.OpenTime))
	fmt.Printf("missing (never owned) openSec=%d\n", exchange.ChartTimeSec(missingOT))
	fmt.Printf("pending opens=%v\n", pendOpens)
	fmt.Printf("pre_flush frameLast=%v verdict=%s\n", preOpens, preVerdict)

	if preVerdict != "PENDING_JUMP_MISSING_MIDDLE" {
		t.Fatalf("Q1 expected PENDING_JUMP_MISSING_MIDDLE, got %s (pending=%v frameTip=%v)",
			preVerdict, pendOpens, preOpens)
	}
	for _, o := range pendOpens {
		if o == exchange.ChartTimeSec(missingOT) {
			t.Fatal("Q1 FAIL: missing 14:04 unexpectedly present in pending")
		}
	}

	// Production B3 path: Exact fill then flush.
	rt.healClosedFetcher = func(string, string, int64, int64) ([]exchange.Candle, error) {
		return []exchange.Candle{{
			OpenTime: missingOT, CloseTime: missingOT + step - 1,
			Open: 105, High: 106, Low: 104, Close: 105.5, Volume: 20,
		}}, nil
	}
	if !rt.finalizeTimelineHealFlush() {
		t.Fatal("B3 finalize should succeed with Exact fill")
	}
	postOpens := frameTipOpens(frame, 5)
	postVerdict := classifyHealFlushProbe(postOpens, []int64{exchange.ChartTimeSec(formingOT)}, "1m", "post_flush")
	fmt.Printf("post_B3_flush frameLast=%v verdict=%s\n", postOpens, postVerdict)
	if postVerdict != "CONTIGUOUS" {
		t.Fatalf("after B3 fill+flush want CONTIGUOUS got %s", postVerdict)
	}
	raw := frame.GetKlines()
	if raw[len(raw)-2].OpenTime != missingOT || raw[len(raw)-1].OpenTime != formingOT {
		t.Fatalf("want missing then forming in Frame tip")
	}

	fmt.Printf("VERDICT: B3 Exact fill restores 14:04 before flush; publishable only when contiguous.\n")
	fmt.Printf("================================\n\n")
}

// TestProbe_Classify_ContiguousPending — control: pending = NextBarOpen(tip) → no jump class.
func TestProbe_Classify_ContiguousPending(t *testing.T) {
	tipMs := int64(1784786580) * 1000
	next, err := data.NextBarOpen(tipMs, "1m")
	if err != nil {
		t.Fatal(err)
	}
	frameSecs := []int64{exchange.ChartTimeSec(tipMs - 60_000), exchange.ChartTimeSec(tipMs)}
	pendSecs := []int64{exchange.ChartTimeSec(next)}
	got := classifyHealFlushProbe(frameSecs, pendSecs, "1m", "pre_flush")
	if got != "CONTIGUOUS" {
		t.Fatalf("contiguous pending: got %s want CONTIGUOUS", got)
	}
}
