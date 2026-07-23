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

// KlineTailNeedsGapFill reports whether the live tip is missing more than one closed bar
// relative to Cap endMs (boundary steps > 1). Fixed and calendar TFs share one law.
func KlineTailNeedsGapFill(lastOpenMs, endMs int64, interval string) bool {
	if lastOpenMs <= 0 || endMs <= 0 || interval == "" {
		return false
	}
	steps, err := data.BarStepsBetween(lastOpenMs, endMs, interval)
	if err != nil {
		return false
	}
	return steps > 1
}

// KlineSeriesNeedsGapFill reports tail staleness or an internal hole between consecutive bars.
// Consecutive opens must equal NextBarOpen(prev) — never Δ > duration for calendar TFs.
func KlineSeriesNeedsGapFill(klines []exchange.Kline, endMs int64, interval string) bool {
	if len(klines) == 0 {
		return true
	}
	if KlineTailNeedsGapFill(klines[len(klines)-1].OpenTime, endMs, interval) {
		return true
	}
	for i := 1; i < len(klines); i++ {
		expected, err := data.NextBarOpen(klines[i-1].OpenTime, interval)
		if err != nil {
			return true
		}
		if klines[i].OpenTime != expected {
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
	if maxBars <= 0 {
		maxBars = LiveKlineRAMCap
	}
	start, err := data.RetreatBarOpen(endMs, maxBars, interval)
	if err != nil || start < 0 {
		return 0, endMs
	}
	return start, endMs
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
// P0: always force REST tip fetch (never skip on false-healthy).
// ADR-017: publishable only after Cap-contiguous REST + Exact closed-gap fill (if pending
// tip jumps) + pending flush + Frame series contiguity check.
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

	if !m.finalizeTimelineHealFlush() {
		log.Printf("[Master] Timeline Reconcile incomplete — heal flush / continuity failed (remaining unpublishable)")
		return
	}
}

// finalizeTimelineHealFlush implements ADR-017 Publishability Contract:
// fill missing CLOSED bars (Exact REST) → flush pending → verify Frame contiguous → publishable.
// Returns false without publishing when fill cannot close a pending tip jump.
func (m *Runtime) finalizeTimelineHealFlush() bool {
	pendingSnap := m.snapshotPendingTicks()
	m.logHealContiguityProbe("pre_flush", pendingSnap)

	if err := m.healFillClosedGapsBeforeFlush(pendingSnap); err != nil {
		log.Printf("[Master] heal closed-gap fill failed: %v", err)
		m.logHealContiguityProbe("fill_failed", pendingSnap)
		return false
	}
	m.logHealContiguityProbe("post_fill", pendingSnap)

	if m.pendingTipJumpRemains(pendingSnap) {
		log.Printf("[Master] heal: pending tip jump remains after closed fill — not flushing")
		m.logHealContiguityProbe("jump_remains", pendingSnap)
		return false
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
	m.logHealContiguityProbe("post_flush", pending)

	if !m.framesSeriesContiguous() {
		log.Printf("[Master] heal: Frame discontinuous after flush — not publishable")
		return false
	}
	if len(m.snapshotPendingTicks()) != 0 {
		log.Printf("[Master] heal: pending not empty after flush — not publishable")
		return false
	}

	m.timelinePublishable.Store(true)
	log.Printf("[Master] Timeline publishable (flushed %d pending, dropped %d during heal)",
		len(pending), dropped)
	m.notifyTimelinePublishable()
	return true
}

// healClosedFillWindow returns Exact REST [from,to] for missing CLOSED opens between
// Frame tip and pending tip. ok=false when no closed fill is needed.
// Proof of settlement: pending/forming tip already exists → prior opens are closed
// (may still be inside Cap settle grace — hence Exact fetch, not Cap-clamped).
func healClosedFillWindow(frameTipOpen, pendingTipOpen int64, interval string) (fromMs, toMs int64, ok bool) {
	if frameTipOpen <= 0 || pendingTipOpen <= 0 || interval == "" {
		return 0, 0, false
	}
	if pendingTipOpen <= frameTipOpen {
		return 0, 0, false
	}
	fromMs, err := data.NextBarOpen(frameTipOpen, interval)
	if err != nil {
		return 0, 0, false
	}
	toMs, err = data.PreviousBarOpen(pendingTipOpen, interval)
	if err != nil {
		return 0, 0, false
	}
	if fromMs > toMs {
		return 0, 0, false
	}
	return fromMs, toMs, true
}

func (m *Runtime) healFillClosedGapsBeforeFlush(pending []exchange.WsTick) error {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	symbol := m.symbol
	intervals := make([]string, 0, len(m.frames))
	for interval := range m.frames {
		intervals = append(intervals, interval)
	}
	m.mu.RUnlock()

	for _, interval := range intervals {
		m.mu.RLock()
		frame := m.frames[interval]
		m.mu.RUnlock()
		if frame == nil {
			continue
		}
		klines := frame.GetKlines()
		if len(klines) == 0 {
			continue
		}
		frameTip := klines[len(klines)-1].OpenTime
		pendTip := minPendingOpenMs(pending, interval)
		if pendTip <= 0 {
			continue
		}
		fromMs, toMs, need := healClosedFillWindow(frameTip, pendTip, interval)
		if !need {
			continue
		}
		candles, err := m.fetchHealClosedRange(symbol, interval, fromMs, toMs)
		if err != nil {
			return fmt.Errorf("%s [%d..%d]: %w", interval, fromMs, toMs, err)
		}
		if len(candles) == 0 {
			return fmt.Errorf("%s [%d..%d]: empty Exact REST (need closed fill)", interval, fromMs, toMs)
		}
		log.Printf("[Master] heal closed fill %s [%d..%d] bars=%d", interval, fromMs, toMs, len(candles))
		frame.LoadHistoricalKlines(exchange.KlinesFromCandles(candles))
		m.enqueueArchiveCandles(symbol, interval, candles)
	}
	return nil
}

func (m *Runtime) fetchHealClosedRange(symbol, interval string, fromMs, toMs int64) ([]exchange.Candle, error) {
	if m.healClosedFetcher != nil {
		return m.healClosedFetcher(symbol, interval, fromMs, toMs)
	}
	if m.exchangeClient == nil {
		return nil, fmt.Errorf("exchange client not bound")
	}
	return m.exchangeClient.FetchClosedRangePagesExact(symbol, interval, fromMs, toMs)
}

func minPendingOpenMs(pending []exchange.WsTick, interval string) int64 {
	var min int64
	for _, tick := range pending {
		if tick.Timeframe != "" && tick.Timeframe != interval {
			continue
		}
		ot := tick.Kline.OpenTime
		if ot <= 0 {
			continue
		}
		if min == 0 || ot < min {
			min = ot
		}
	}
	return min
}

// pendingTipJumpRemains is true when pending would still skip ≥1 closed open after fill.
func (m *Runtime) pendingTipJumpRemains(pending []exchange.WsTick) bool {
	if m == nil || len(pending) == 0 {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for interval, frame := range m.frames {
		if frame == nil {
			continue
		}
		klines := frame.GetKlines()
		if len(klines) == 0 {
			continue
		}
		tip := klines[len(klines)-1].OpenTime
		pend := minPendingOpenMs(pending, interval)
		if pend <= 0 {
			continue
		}
		if _, _, need := healClosedFillWindow(tip, pend, interval); need {
			return true
		}
	}
	return false
}

// framesSeriesContiguous reports consecutive NextBarOpen links for every Frame (no Cap bound).
func (m *Runtime) framesSeriesContiguous() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for interval, frame := range m.frames {
		if frame == nil {
			return false
		}
		klines := frame.GetKlines()
		if len(klines) == 0 {
			return false
		}
		for i := 1; i < len(klines); i++ {
			expected, err := data.NextBarOpen(klines[i-1].OpenTime, interval)
			if err != nil || klines[i].OpenTime != expected {
				return false
			}
		}
	}
	return true
}

// framesTimelineHealthy reports whether every chart Frame has a continuous closed-bar
// series through the last closed tip (NextBarOpen continuity).
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
		frame := frames[i]
		if frame == nil {
			return false
		}
		endMs := nowMs
		if capped, err := data.CapKlineEndToLastClosed(nowMs, interval); err == nil {
			endMs = capped
		}
		if KlineSeriesNeedsGapFill(frame.GetKlines(), endMs, interval) {
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
			if force && firstErr == nil {
				firstErr = fmt.Errorf("nil Frame for %s", interval)
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
	startMs, endMs := klineReconcileWindowMs(interval, LiveKlineRAMCap)
	tail := frame.GetKlines()
	// Periodic path only: healthy Frame skips REST. Forced Timeline Reconcile never skips.
	if !force && len(tail) > 0 && !KlineSeriesNeedsGapFill(tail, endMs, interval) {
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

// logHealContiguityProbe answers GPT's four questions (logs only, no behavior change).
// Verdict classes:
//   PENDING_JUMP_MISSING_MIDDLE — Cap/Frame tip then pending skips ≥1 closed open
//   FRAME_HOLE_AFTER_FLUSH      — Frame series discontinuous after applyTick flush
//   CONTIGUOUS                  — no tip jump / no frame hole detected
func (m *Runtime) logHealContiguityProbe(phase string, pending []exchange.WsTick) {
	if m == nil {
		return
	}
	nowMs := time.Now().UnixMilli()
	capOpen := int64(0)
	if c, err := data.CapKlineEndToLastClosed(nowMs, m.timeframe); err == nil {
		capOpen = c
	}

	m.mu.RLock()
	tf := m.timeframe
	frame := m.frames[tf]
	m.mu.RUnlock()

	frameOpens := frameTipOpens(frame, 10)
	pendingOpens := pendingTipOpens(pending, tf)
	verdict := classifyHealFlushProbe(frameOpens, pendingOpens, tf, phase)

	log.Printf("[HealProbe] phase=%s tf=%s nowMs=%d capOpenMs=%d capOpenSec=%d frameLastOpens=%v pendingOpens=%v verdict=%s",
		phase, tf, nowMs, capOpen, exchange.ChartTimeSec(capOpen), frameOpens, pendingOpens, verdict)
}

func frameTipOpens(frame *Frame, n int) []int64 {
	if frame == nil || n <= 0 {
		return nil
	}
	klines := frame.GetKlines()
	if len(klines) == 0 {
		return nil
	}
	if n > len(klines) {
		n = len(klines)
	}
	out := make([]int64, n)
	base := len(klines) - n
	for i := 0; i < n; i++ {
		out[i] = exchange.ChartTimeSec(klines[base+i].OpenTime)
	}
	return out
}

func pendingTipOpens(pending []exchange.WsTick, tf string) []int64 {
	if len(pending) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(pending))
	out := make([]int64, 0, len(pending))
	for _, tick := range pending {
		if tf != "" && tick.Timeframe != "" && tick.Timeframe != tf {
			continue
		}
		sec := exchange.ChartTimeSec(tick.Kline.OpenTime)
		if _, ok := seen[sec]; ok {
			continue
		}
		seen[sec] = struct{}{}
		out = append(out, sec)
	}
	return out
}

func classifyHealFlushProbe(frameOpens, pendingOpens []int64, interval, phase string) string {
	if len(frameOpens) == 0 {
		return "NO_FRAME"
	}
	tipSec := frameOpens[len(frameOpens)-1]
	tipMs := tipSec * 1000

	if phase == "pre_flush" && len(pendingOpens) > 0 {
		minPend := pendingOpens[0]
		for _, o := range pendingOpens[1:] {
			if o < minPend {
				minPend = o
			}
		}
		next, err := data.NextBarOpen(tipMs, interval)
		if err == nil {
			nextSec := exchange.ChartTimeSec(next)
			if minPend > nextSec {
				return "PENDING_JUMP_MISSING_MIDDLE"
			}
		}
		steps, err := data.BarStepsBetween(tipMs, minPend*1000, interval)
		if err == nil && steps > 1 {
			return "PENDING_JUMP_MISSING_MIDDLE"
		}
	}

	if phase == "post_flush" {
		// Internal hole among last tip opens (ChartTimeSec deltas ≠ 60 for 1m).
		for i := 1; i < len(frameOpens); i++ {
			prevMs := frameOpens[i-1] * 1000
			curMs := frameOpens[i] * 1000
			expected, err := data.NextBarOpen(prevMs, interval)
			if err != nil {
				continue
			}
			if curMs != expected {
				return "FRAME_HOLE_AFTER_FLUSH"
			}
		}
		if len(pendingOpens) > 0 {
			minPend := pendingOpens[0]
			for _, o := range pendingOpens[1:] {
				if o < minPend {
					minPend = o
				}
			}
			// After flush, tip should reach pending; if tip still behind pending min by >1 step — incomplete.
			if tipSec < minPend {
				steps, err := data.BarStepsBetween(tipMs, minPend*1000, interval)
				if err == nil && steps > 1 {
					return "FRAME_TIP_STILL_BEHIND_PENDING"
				}
			}
		}
	}
	return "CONTIGUOUS"
}

