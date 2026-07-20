package market

import (
	"context"
	"fmt"
	"log"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

const (
	klineGapFillInterval     = 5 * time.Minute
	timelineReconcileMaxTry  = 8
	timelineReconcileBackoff = time.Second
)

// intervalSkipsKlineGapFill reports intervals where REST gap-fill is unreliable or unused.
func intervalSkipsKlineGapFill(interval string) bool {
	switch interval {
	case "1M", "1w":
		return true
	default:
		return false
	}
}

// KlineTailNeedsGapFill reports whether the live tip is missing at least one closed bar.
// P0: Δ > 1×interval (was > 2×) — a single skipped 1m bar must not look "healthy".
func KlineTailNeedsGapFill(lastOpenMs, endMs, intervalMs int64) bool {
	if lastOpenMs <= 0 || endMs <= 0 || intervalMs <= 0 {
		return false
	}
	return endMs-lastOpenMs > intervalMs
}

// KlineSeriesNeedsGapFill reports tail staleness or an internal hole between consecutive bars.
// P0: any OpenTime jump > 1×interval is a hole (one missing bar ⇒ Δ = 2×interval).
func KlineSeriesNeedsGapFill(klines []exchange.Kline, endMs, intervalMs int64) bool {
	if len(klines) == 0 {
		return true
	}
	if KlineTailNeedsGapFill(klines[len(klines)-1].OpenTime, endMs, intervalMs) {
		return true
	}
	for i := 1; i < len(klines); i++ {
		if klines[i].OpenTime-klines[i-1].OpenTime > intervalMs {
			return true
		}
	}
	return false
}

func klineReconcileWindowMs(interval string, maxBars int) (startMs, endMs int64) {
	endMs = time.Now().UnixMilli()
	if capped, err := data.CapKlineEndToLastClosed(endMs, interval); err == nil {
		endMs = capped
	}
	intervalMs, err := data.IntervalDurationMs(interval)
	if err != nil || intervalMs <= 0 {
		return 0, endMs
	}
	if maxBars <= 0 {
		maxBars = LiveKlineRAMCap
	}
	startMs = endMs - intervalMs*int64(maxBars)
	if startMs < 0 {
		startMs = 0
	}
	return startMs, endMs
}

// StartKlineGapFillLoop runs an immediate reconcile and periodic RAM gap-fill (Master SSOT).
// When the publish gate is closed, the loop retries ReconcileTimeline (not a bare
// ReconcileAllKlineGaps) so pending ticks can flush once the Frame is continuous.
func (m *Runtime) StartKlineGapFillLoop(ctx context.Context) {
	if m == nil {
		return
	}
	go func() {
		m.ReconcileAllKlineGaps()
		ticker := time.NewTicker(klineGapFillInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !m.IsTimelinePublishable() {
					m.ReconcileTimeline(ctx)
					continue
				}
				m.ReconcileAllKlineGaps()
			}
		}
	}()
}

// ReconcileTimeline is the single mid-session heal path (reconnect / ingest gap / backup loop).
// P0: always force REST tip fetch (never skip on false-healthy); publishable only after
// successful fetch cycle AND contiguous@1bar framesTimelineHealthy.
func (m *Runtime) ReconcileTimeline(ctx context.Context) {
	if m == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !m.timelineReconciling.CompareAndSwap(false, true) {
		return
	}
	defer m.timelineReconciling.Store(false)

	m.timelinePublishable.Store(false)
	m.notifyTimelineHealing()
	log.Printf("[Master] Timeline Reconcile started")

	backoff := timelineReconcileBackoff
	healthy := false
	for attempt := 1; attempt <= timelineReconcileMaxTry; attempt++ {
		if err := ctx.Err(); err != nil {
			log.Printf("[Master] Timeline Reconcile cancelled: %v", err)
			return
		}
		fetchErr := m.reconcileAllKlineGaps(true)
		if fetchErr != nil {
			log.Printf("[Master] Timeline Reconcile attempt %d/%d: forced REST failed: %v (backoff %s)",
				attempt, timelineReconcileMaxTry, fetchErr, backoff)
		} else if m.framesTimelineHealthy() {
			healthy = true
			break
		} else {
			log.Printf("[Master] Timeline Reconcile attempt %d/%d: REST ok but Frame still discontinuous (backoff %s)",
				attempt, timelineReconcileMaxTry, backoff)
		}
		select {
		case <-ctx.Done():
			log.Printf("[Master] Timeline Reconcile cancelled during backoff")
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}

	if !healthy {
		// Stay unpublishable; pending keeps absorbing live ticks. 5m loop / next reconnect retries.
		log.Printf("[Master] Timeline Reconcile incomplete — remaining unpublishable")
		return
	}

	pending := m.drainPendingTicks()
	dropped := 0
	m.pendingMu.Lock()
	dropped = m.pendingDropped
	m.pendingDropped = 0
	m.pendingMu.Unlock()

	for _, tick := range pending {
		m.applyTick(tick)
	}
	m.timelinePublishable.Store(true)
	log.Printf("[Master] Timeline publishable (flushed %d pending, dropped %d during heal)",
		len(pending), dropped)
	m.notifyTimelinePublishable()
}

// framesTimelineHealthy reports whether every chart Frame that participates in
// gap-fill has a continuous closed-bar series through the last closed tip (1-bar resolution).
func (m *Runtime) framesTimelineHealthy() bool {
	if m == nil {
		return false
	}
	nowMs := time.Now().UnixMilli()
	m.mu.RLock()
	intervals := make([]string, 0, len(m.frames))
	frames := make([]*Frame, 0, len(m.frames))
	for interval, frame := range m.frames {
		intervals = append(intervals, interval)
		frames = append(frames, frame)
	}
	m.mu.RUnlock()

	for i, interval := range intervals {
		if intervalSkipsKlineGapFill(interval) {
			continue
		}
		frame := frames[i]
		if frame == nil {
			return false
		}
		intervalMs, err := data.IntervalDurationMs(interval)
		if err != nil || intervalMs <= 0 {
			continue
		}
		endMs := nowMs
		if capped, err := data.CapKlineEndToLastClosed(nowMs, interval); err == nil {
			endMs = capped
		}
		if KlineSeriesNeedsGapFill(frame.GetKlines(), endMs, intervalMs) {
			return false
		}
	}
	return true
}

// ReconcileAllKlineGaps checks every Frame for holes and backfills via FetchClosedRange → RAM.
// Periodic path: skip REST when series already contiguous (force=false).
func (m *Runtime) ReconcileAllKlineGaps() {
	_ = m.reconcileAllKlineGaps(false)
}

// reconcileAllKlineGaps walks chart Frames. force=true (Timeline Reconcile): always REST
// tip-window — reconnect must not skip on a false-healthy 1-bar hole.
func (m *Runtime) reconcileAllKlineGaps(force bool) error {
	if m == nil || m.exchangeClient == nil {
		if force {
			return fmt.Errorf("exchange client not bound")
		}
		return nil
	}
	m.mu.RLock()
	symbol := m.symbol
	intervals := make([]string, 0, len(m.frames))
	for interval := range m.frames {
		intervals = append(intervals, interval)
	}
	m.mu.RUnlock()

	var firstErr error
	for _, interval := range intervals {
		m.mu.RLock()
		frame := m.frames[interval]
		m.mu.RUnlock()
		if frame == nil {
			if force && !intervalSkipsKlineGapFill(interval) {
				if firstErr == nil {
					firstErr = fmt.Errorf("nil Frame for %s", interval)
				}
			}
			continue
		}
		if err := m.reconcileKlineGap(symbol, interval, frame, force); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *Runtime) reconcileKlineGap(symbol, interval string, frame *Frame, force bool) error {
	if m == nil || m.exchangeClient == nil || frame == nil {
		return fmt.Errorf("reconcileKlineGap: nil receiver/client/frame")
	}
	if intervalSkipsKlineGapFill(interval) {
		return nil
	}
	startMs, endMs := klineReconcileWindowMs(interval, LiveKlineRAMCap)
	intervalMs, err := data.IntervalDurationMs(interval)
	if err != nil || intervalMs <= 0 {
		return fmt.Errorf("interval duration %s: %w", interval, err)
	}
	tail := frame.GetKlines()
	// Periodic path only: healthy Frame skips REST. Forced Timeline Reconcile never skips.
	if !force && len(tail) > 0 && !KlineSeriesNeedsGapFill(tail, endMs, intervalMs) {
		return nil
	}

	candles, err := m.exchangeClient.FetchClosedRangePages(symbol, interval, startMs, endMs)
	if err != nil {
		log.Printf("[Master] gap-fill %s %s [%d..%d]: %v", symbol, interval, startMs, endMs, err)
		return err
	}
	if len(candles) == 0 {
		if force {
			log.Printf("[Master] gap-fill %s %s: empty REST window (force)", symbol, interval)
		}
		return nil
	}
	klines := exchange.KlinesFromCandles(candles)
	frame.LoadHistoricalKlines(klines)

	// Archive the same real bars via sole writer (does not mutate RAM).
	m.enqueueArchiveCandles(symbol, interval, candles)

	m.mu.RLock()
	workTF := m.timeframe
	m.mu.RUnlock()
	if interval == workTF {
		m.SeedClosedBarTelemetry()
		m.rebuildMTFTrackerIfReady()
	}
	return nil
}

func (m *Runtime) enqueueArchiveCandles(symbol, interval string, candles []exchange.Candle) {
	if m == nil || len(candles) == 0 {
		return
	}
	m.mu.RLock()
	q := m.persistQ
	m.mu.RUnlock()
	if q == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := q.AppendClosedBars(ctx, symbol, interval, exchange.CandlesToData(candles)); err != nil {
		log.Printf("[Master] archive enqueue %s %s: %v", symbol, interval, err)
	}
}
