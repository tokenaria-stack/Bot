package market

import (
	"context"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"trading_bot/core"
	"trading_bot/exchange"
)

func testChaos() ChaosConfig {
	return ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34}
}

// TestTimelinePublishGate_UnpublishableBuffersTicks — Phase E gate unit:
// while !publishable, routeTick must not call onKlineBar; forced reconcile without
// exchange client must not flip publishable.
func TestTimelinePublishGate_UnpublishableBuffersTicks(t *testing.T) {
	klines := synthOHLCV(80)
	frame := NewFrame(klines, "1m", testChaos())
	rt := NewRuntime(map[string]*Frame{"1m": frame}, nil, nil, true, false, "BTCUSDT", "1m")

	var calls atomic.Int32
	rt.SetOnKlineBar(func(string, exchange.Kline, bool) {
		calls.Add(1)
	})

	rt.OnBinanceDisconnect()
	if rt.IsTimelinePublishable() {
		t.Fatal("expected unpublishable after Binance disconnect")
	}

	last := klines[len(klines)-1]
	nextOpen := last.OpenTime + 60_000
	tick := exchange.WsTick{
		Timeframe: "1m",
		IsClosed:  false,
		Kline: exchange.NormalizeKline(exchange.Kline{
			OpenTime:  nextOpen,
			CloseTime: nextOpen + 59_999,
			Open:      last.Close,
			High:      last.Close + 0.1,
			Low:       last.Close - 0.1,
			Close:     last.Close,
			Volume:    1,
		}),
	}
	rt.routeTick(tick)
	if calls.Load() != 0 {
		t.Fatalf("onKlineBar fired %d times while unpublishable", calls.Load())
	}

	var published atomic.Bool
	rt.SetOnTimelinePublishable(func() { published.Store(true) })
	// Pre-cancelled ctx: assert unpublishable without waiting forced-REST backoff ladder.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rt.ReconcileTimeline(ctx)
	if published.Load() || rt.IsTimelinePublishable() {
		t.Fatal("ReconcileTimeline without exchange must remain unpublishable")
	}

	// Same-package: restore publishable and confirm live path broadcasts again.
	rt.timelinePublishable.Store(true)
	rt.routeTick(tick)
	if calls.Load() != 1 {
		t.Fatalf("onKlineBar after publishable: got %d want 1", calls.Load())
	}
}

// TestTimelinePublishGate_IngestTipGapUnpublishes — backend twin of FE 1.5× gapDetect.
func TestTimelinePublishGate_IngestTipGapUnpublishes(t *testing.T) {
	klines := synthOHLCV(40)
	frame := NewFrame(klines, "1m", testChaos())
	rt := NewRuntime(map[string]*Frame{"1m": frame}, nil, nil, true, false, "BTCUSDT", "1m")

	feedCtx, feedCancel := context.WithCancel(context.Background())
	defer feedCancel()
	rt.mu.Lock()
	rt.feedCtx = feedCtx
	rt.mu.Unlock()

	var healing atomic.Bool
	rt.SetOnTimelineHealing(func() { healing.Store(true) })

	last := klines[len(klines)-1]
	// Jump two intervals ahead → > 1.5× on 1m.
	gapOpen := last.OpenTime + 2*60_000
	tick := exchange.WsTick{
		Timeframe: "1m",
		IsClosed:  true,
		Kline: exchange.NormalizeKline(exchange.Kline{
			OpenTime:  gapOpen,
			CloseTime: gapOpen + 59_999,
			Open:      last.Close,
			High:      last.Close,
			Low:       last.Close,
			Close:     last.Close,
			Volume:    1,
		}),
	}
	rt.routeTick(tick)
	if rt.IsTimelinePublishable() {
		t.Fatal("ingest tip gap must unpublish")
	}
	feedCancel() // stop async ReconcileTimeline backoff (nil exchange)
	deadline := time.Now().Add(200 * time.Millisecond)
	for !healing.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !healing.Load() {
		t.Fatal("expected timeline healing notify on ingest gap")
	}
}

// TestTimelineHeal_TipMatchesReplayAfterLoadHistorical — Phase E continuity:
// gap in Frame → LoadHistoricalKlines (REST fill twin) → live tip ≡ cold ReplayDAG.
func TestTimelineHeal_TipMatchesReplayAfterLoadHistorical(t *testing.T) {
	t.Parallel()
	full := synthOHLCV(400)
	// Drop three closed bars near tip (keep last two) → one-bar-class hole.
	gappy := make([]exchange.Kline, 0, len(full)-3)
	gappy = append(gappy, full[:len(full)-5]...)
	gappy = append(gappy, full[len(full)-2:]...)

	endMs := full[len(full)-1].OpenTime
	if !KlineSeriesNeedsGapFill(gappy, endMs, 60_000) {
		t.Fatal("precondition: gappy series must need gap fill at 1-bar threshold")
	}

	rsxCfg := NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})
	frame := NewFrame(gappy, "1m", testChaos())
	frame.LoadHistoricalKlines(full)

	dag := frame.DAGTickFrame()
	if dag == nil {
		t.Fatal("DAGTickFrame nil after LoadHistoricalKlines")
	}
	liveRSX := dag.Get(core.SlotJurikRSX)
	liveWoz := dag.Get(core.SlotWozduhRsiPrice)
	repRSX, repWoz := replayTip(full, rsxCfg)

	const eps = 1e-9
	if math.Abs(liveRSX-repRSX) > eps || math.Abs(liveWoz-repWoz) > eps {
		t.Fatalf("after heal tip mismatch: live RSX=%.12f replay=%.12f | live Woz=%.12f replay=%.12f",
			liveRSX, repRSX, liveWoz, repWoz)
	}
}
