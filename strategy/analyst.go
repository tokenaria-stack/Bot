package strategy

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/qdrant/go-client/qdrant"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/indicators"
	"trading_bot/vector_db"
)

const (
	patternWindowSize      = 20
	similarPatternsLimit   = 3
	billWilliamsFractalWin = 2
	fractalATRFilterWindow = 5
	fractalATRMultiplier   = 1.0
)

// ChaosConfig holds configurable Trading Chaos indicator parameters.
type ChaosConfig struct {
	AOFastPeriod int
	AOSlowPeriod int
}

// Marker is the thread-safe market data layer for the analytics pipeline.
type Marker struct {
	mu                       sync.RWMutex
	klines                   []exchange.Kline
	db                       *vector_db.DBClient
	timeframe                string
	collection               string
	config                   ChaosConfig
	lastSavedFractalOpenTime int64
	falcon                   *FalconEngine
	falconSignals            FalconSignals
	volEngine                *VolatilityEngine
	divEngine                *indicators.SmartDivergenceEngine
	zigzag                   *indicators.ZigZag
	geometry                 *geometryTracker
	orangeRsi                *indicators.RSI
	ad                       *indicators.AD
	stoch                    *indicators.Stochastic
	ao                       *indicators.AO
	latestAO                 float64
	fibEngine                *indicators.FibonacciEngine
	fibZones                 []indicators.FibZone
	fibWaveStart             float64
	fibWaveEnd               float64
	fibWaveReady             bool
	prevFalconRed            float64
	prevFalconGreen          float64
	prevFalconBlue           float64
	redLineCrossGreenUp      bool
	redLineCrossGreenDown    bool
	jurikValue               float64
	jurikIsRising            bool
	prevJurik                float64
	jurikPrevBar             float64
	prevAO                   float64
	prevAOReady              bool
	adHistory                []float64
	wozduxVolumeSpikeUp      bool
	wozduxVolumeSpikeDown    bool
	geometryBounceUp         bool
	geometryBounceDown       bool
	geometryTriangle         bool
	accumulationRising       bool
	distributionFalling      bool
	aoCrossZeroUp            bool
	aoCrossZeroDown          bool
	volatilityState          VolatilityState
	divSignal                indicators.DivSignal
	zigZagState              ZigZagState
	geometryState            GeometryState
	prevZigNode              indicators.ZigZagNode
	prevZigHas               bool
	rsxSettings              *RSXSettings
	// DataBus — единый реестр синхронизированных серий (владелец — только Marker).
	JurikLines           []float64
	WozduhRed            []float64
	WozduhGreen          []float64
	Annotations          []ChartAnnotation
	layer2Snap           layer2StreamingSnapshot
	mtfStates            map[string]*HTFState
	cachedRSXMarkerBar   int
	cachedRSXMarkerLabel string
	closeLines           []float64
	rsxPriceLines        []float64
	bulkReplayMode       bool
	dag                  *core.DAGRunner
	// lastCommittedOpenTime is the OpenTime of the most recently Save-committed bar
	// (streaming engines + DAG). Guards UpdateKlineTick's cross-bar handoff against
	// double-committing a bar already closed via isClosed==true (Jeweler Protocol: no double IIR pass).
	lastCommittedOpenTime int64
}

// NewMarker loads the initial candle history into a protected store.
func NewMarker(history []exchange.Kline, db *vector_db.DBClient, timeframe, collection string, config ChaosConfig) *Marker {
	copied := make([]exchange.Kline, len(history))
	copy(copied, history)

	a := &Marker{
		klines:     copied,
		db:         db,
		timeframe:  timeframe,
		collection: collection,
		config:     config,
	}
	a.warmupStreaming(copied)
	return a
}

// SetRSXSettings pins per-marker RSX config (backtest isolation). Call ReapplyRSXSettings after.
func (a *Marker) SetRSXSettings(settings RSXSettings) {
	a.mu.Lock()
	defer a.mu.Unlock()
	normalized := NormalizeRSXSettings(settings)
	a.rsxSettings = &normalized
}

func (a *Marker) effectiveRSXSettings() RSXSettings {
	if a.rsxSettings != nil {
		return *a.rsxSettings
	}
	return GetRSXSettings()
}

// ReapplyRSXSettings rebuilds RSX marker state and replays streaming engines after setting changes.
func (a *Marker) ReapplyRSXSettings() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.replayStreamingLocked()
}

// ApplyBacktestRSXConfig pins per-run RSX settings and replays engines for isolated backtests.
func (a *Marker) ApplyBacktestRSXConfig(settings RSXSettings) {
	a.mu.Lock()
	defer a.mu.Unlock()
	normalized := NormalizeRSXSettings(settings)
	a.rsxSettings = &normalized
	a.falcon.SetRSXLength(normalized.Length)
	a.falcon.SetRSXSignalLength(normalized.SignalLength)
	a.falcon.SetRSXSource(normalized.Source)
	if a.divEngine != nil {
		a.divEngine.UpdateRSXConfig(rsxScanConfigFromSettings(normalized))
	}
	a.replayStreamingLocked()
}

// UpdateRSXScanConfig applies RSX settings on the fly: replays Jurik when length/signal/source
// change, otherwise rebuilds divergence annotations only.
func (a *Marker) UpdateRSXScanConfig(settings RSXSettings) {
	a.mu.Lock()
	defer a.mu.Unlock()

	prev := a.effectiveRSXSettings()
	if a.rsxSettings != nil {
		normalized := NormalizeRSXSettings(settings)
		a.rsxSettings = &normalized
	}
	next := a.effectiveRSXSettings()

	a.falcon.SetRSXLength(next.Length)
	a.falcon.SetRSXSignalLength(next.SignalLength)
	a.falcon.SetRSXSource(next.Source)

	if a.divEngine != nil {
		a.divEngine.UpdateRSXConfig(rsxScanConfigFromSettings(next))
	}

	if a.dag != nil {
		_ = a.dag.OnConfigChange("rsx", nodes.RSXNodeConfig{
			Length:       next.Length,
			SignalLength: next.SignalLength,
			Source:       next.Source,
		})
	}

	needsReplay := prev.Length != next.Length ||
		prev.SignalLength != next.SignalLength ||
		normalizeRSXSource(prev.Source) != normalizeRSXSource(next.Source)
	if needsReplay {
		a.replayStreamingLocked()
		return
	}
	a.rebuildRSXAnnotationsLocked()
}

// JurikRSXColor returns the TradingView-style RSX line color for the latest bar.
func (a *Marker) JurikRSXColor() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return RSXColor(a.jurikValue, a.jurikPrevBar)
}

// LoadHistoricalKlines merges real exchange bars with the live RAM buffer (overlay wins on
// duplicate OpenTime), then replays streaming engines once.
// Callers must pass exchange-sourced bars only (FetchClosedRange / WS) — never synthetic fills.
//
// Mutex contract: analyst.mu is a plain sync.Mutex (blocking wait, no timeout).
// UpdateKlineTick and LoadHistoricalKlines are the only writers; both call Lock once.
// replayStreamingLocked / evaluateTickLocked must never acquire mu — no re-entrant deadlock.
// While this runs, the WS data-feed goroutine blocks on Lock; OutCh (cap 1000) applies
// backpressure to the socket reader until hydrate completes.
func (a *Marker) LoadHistoricalKlines(klines []exchange.Kline) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Ingress SSOT: RAM live bars are WS-confirmed (Final) — REST backfill (Settled)
	// can add missing bars but never overwrite what the live feed already finalized.
	merged := exchange.MergeKlineSeries(klines, a.klines, exchange.AuthoritySettled, exchange.AuthorityFinal)
	if len(merged) > LiveKlineRAMCap {
		merged = merged[len(merged)-LiveKlineRAMCap:]
	}
	a.klines = merged
	a.replayStreamingLocked()
	a.alignAllDataBusToKlinesLocked()
}

// UpdateKline appends a new candle or overwrites the latest one for the same open time.
func (a *Marker) UpdateKline(k exchange.Kline) {
	a.UpdateKlineTick(k, false)
}

// UpdateKlineTick ingests a live or historical bar; isClosed is Binance k.x (bar finalized).
func (a *Marker) UpdateKlineTick(k exchange.Kline, isClosed bool) {
	k = exchange.NormalizeKline(k)
	if k.CloseTime <= 0 && k.OpenTime > 0 && a.timeframe != "" {
		if dur, err := data.IntervalDurationMs(a.timeframe); err == nil {
			k.CloseTime = k.OpenTime + dur - 1
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.klines) == 0 {
		a.klines = append(a.klines, k)
		a.evalTick(k, 0, isClosed)
		if isClosed {
			a.lastCommittedOpenTime = k.OpenTime
		}
		return
	}

	lastIdx := len(a.klines) - 1
	last := a.klines[lastIdx]

	if k.OpenTime == last.OpenTime {
		a.klines[lastIdx] = k
		a.evalTick(k, lastIdx, isClosed)
		if isClosed {
			a.lastCommittedOpenTime = k.OpenTime
		}
		return
	}

	if k.OpenTime > last.OpenTime {
		// Commit the previous bar into streaming snapshots before opening a new one —
		// unless it was already committed via isClosed==true (avoid double IIR pass).
		if last.OpenTime != a.lastCommittedOpenTime {
			a.evalTick(last, lastIdx, true)
			a.lastCommittedOpenTime = last.OpenTime
		}
		a.klines = append(a.klines, k)
		a.evalTick(k, len(a.klines)-1, isClosed)
		a.alignAllDataBusToKlinesLocked()
		a.trimKlinesToCapLocked()
		a.clampDataBusToKlinesLocked()
		return
	}

	// Out-of-order tick for an earlier period — drop.
	_ = isClosed
}

func (a *Marker) evalTick(k exchange.Kline, barIndex int, isClosed bool) {
	if a.bulkReplayMode {
		a.evaluateTickBulkChartLocked(k, barIndex, isClosed)
		return
	}
	a.evaluateTickLocked(k, barIndex, isClosed)
}

// evaluateTickBulkChartLocked is the chart-only cold replay path.
// Shot 9F: ChartOnly → DAG only; Live → Falcon + RSX markers (legacy export).
func (a *Marker) evaluateTickBulkChartLocked(k exchange.Kline, barIndex int, isClosed bool) {
	a.evaluateFalconSignalsLocked(k, barIndex, isClosed)
	if !EngineAllowsStrategies() {
		return
	}
	a.recordDataBusBarLocked(barIndex, a.falconSignals)
	a.cachedRSXMarkerBar = barIndex
	a.cachedRSXMarkerLabel = ""
	if a.divEngine != nil {
		hit := indicators.RSXHitAtDisplayBar(a, barIndex, a.divEngine.RSXConfig())
		if hit.Label != "" {
			a.cachedRSXMarkerLabel = hit.Label
		}
	}
}

// SetCurrentMTFState stores walk-forward HTF navigator state for scoring (keyed by interval).
// states is adopted by pointer-swap; callers must treat published HTFState values as read-only.
func (a *Marker) SetCurrentMTFState(states map[string]*HTFState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mtfStates = states
}

// MTFState returns walk-forward HTF state for one interval (nil when unavailable).
// Returned state is read-only; valid until the next SetCurrentMTFState.
func (a *Marker) MTFState(tf string) *HTFState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.mtfStates[tf]
}

// MTFStates returns walk-forward HTF states keyed by interval (read-only view).
// Valid until the next SetCurrentMTFState on this marker.
func (a *Marker) MTFStates() map[string]*HTFState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.mtfStates) == 0 {
		return nil
	}
	return a.mtfStates
}

func (a *Marker) EffectiveRSXSettings() RSXSettings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.effectiveRSXSettings()
}

// SetBulkReplayMode enables linear replay without per-bar snapshot restore/save (chart cold path).
func (a *Marker) SetBulkReplayMode(enabled bool) {
	a.mu.Lock()
	a.bulkReplayMode = enabled
	a.mu.Unlock()
}

// BarCount returns the number of klines held by the marker.
func (a *Marker) BarCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.klines)
}

// GetKlines returns a defensive copy of the stored candles.
func (a *Marker) GetKlines() []exchange.Kline {
	a.mu.RLock()
	defer a.mu.RUnlock()

	copied := make([]exchange.Kline, len(a.klines))
	copy(copied, a.klines)
	return copied
}

// UpdateIndicators is retained for pipeline compatibility; AO is updated in the streaming path.
func (a *Marker) UpdateIndicators() {}

// GetLatestAO returns the streaming Awesome Oscillator value from the latest tick.
func (a *Marker) GetLatestAO() float64 {
	a.mu.RLock()
	defer a.mu.Unlock()
	return a.latestAO
}

// BackfillHistory scans stored candles and saves historical fractal patterns to Qdrant.
func (a *Marker) BackfillHistory(ctx context.Context) error {
	klines := a.GetKlines()
	if len(klines) < patternWindowSize {
		return nil
	}

	savedCount := 0

	filteredPeaks, err := filteredFractalPeaks(klines)
	if err != nil {
		return fmt.Errorf("detect fractal peaks: %w", err)
	}

	for _, peak := range filteredPeaks {
		if peak.Index < patternWindowSize-1 {
			continue
		}

		candle := klines[peak.Index]
		isUpFractal := peak.Type == indicators.PeakHigh
		price := peak.Value

		window := klines[peak.Index-(patternWindowSize-1) : peak.Index+1]
		vector, err := vector_db.VectorizeCandles(window)
		if err != nil {
			return fmt.Errorf("vectorize backfill window at index %d: %w", peak.Index, err)
		}

		pointID := uint64(candle.OpenTime)
		if err := a.db.SavePattern(ctx, a.collection, vector, pointID, price, isUpFractal); err != nil {
			return fmt.Errorf("save backfill pattern at index %d: %w", peak.Index, err)
		}

		savedCount++
	}

	slog.Info("Historical backfill completed",
		slog.String("tf", a.timeframe),
		"saved_patterns", savedCount,
	)
	return nil
}

// CheckAndSaveFractal detects a confirmed fractal on the 3rd candle from the end and saves its embedding.
func (a *Marker) CheckAndSaveFractal(ctx context.Context) error {
	klines := a.GetKlines()
	if len(klines) < patternWindowSize {
		return nil
	}

	index := len(klines) - 3
	fractalCandle := klines[index]
	if fractalCandle.OpenTime == a.lastSavedFractalOpenTime {
		return nil
	}

	filteredPeaks, err := filteredFractalPeaks(klines)
	if err != nil {
		return fmt.Errorf("detect fractal peaks: %w", err)
	}

	var (
		isUpFractal bool
		price       float64
		fractalType string
		found       bool
	)

	for _, peak := range filteredPeaks {
		if peak.Index != index {
			continue
		}

		found = true
		price = peak.Value
		if peak.Type == indicators.PeakHigh {
			isUpFractal = true
			fractalType = "UP"
		} else {
			isUpFractal = false
			fractalType = "DOWN"
		}
		break
	}

	if !found {
		return nil
	}

	window := klines[len(klines)-patternWindowSize:]
	vector, err := vector_db.VectorizeCandles(window)
	if err != nil {
		return fmt.Errorf("vectorize candles: %w", err)
	}

	pointID := uint64(fractalCandle.OpenTime)
	if err := a.db.SavePattern(ctx, a.collection, vector, pointID, price, isUpFractal); err != nil {
		return fmt.Errorf("save pattern: %w", err)
	}

	a.mu.Lock()
	a.lastSavedFractalOpenTime = fractalCandle.OpenTime
	a.mu.Unlock()

	slog.Info("Fractal pattern vectorized and saved to memory!",
		slog.String("tf", a.timeframe),
		"type", fractalType,
		"price", price,
	)
	return nil
}

// PredictNextMovement searches Qdrant for historical patterns similar to the latest 20 candles.
func (a *Marker) PredictNextMovement(ctx context.Context) error {
	klines := a.GetKlines()
	if len(klines) < patternWindowSize {
		return fmt.Errorf("not enough klines for prediction: got %d, need %d", len(klines), patternWindowSize)
	}

	window := klines[len(klines)-patternWindowSize:]
	vector, err := vector_db.VectorizeCandles(window)
	if err != nil {
		return fmt.Errorf("vectorize prediction window: %w", err)
	}

	results, err := a.db.SearchSimilarPatterns(ctx, a.collection, vector, similarPatternsLimit)
	if err != nil {
		return fmt.Errorf("search similar patterns: %w", err)
	}

	if len(results) == 0 {
		slog.Info("no similar fractal patterns found in qdrant", slog.String("tf", a.timeframe))
		return nil
	}

	for _, point := range results {
		payload := point.GetPayload()
		slog.Info("similar historical pattern",
			slog.String("tf", a.timeframe),
			"score", point.GetScore(),
			"is_up_fractal", payloadBool(payload, "is_up_fractal"),
			"price", payloadDouble(payload, "price"),
		)
	}

	return nil
}

func payloadBool(payload map[string]*qdrant.Value, key string) bool {
	if payload == nil {
		return false
	}

	value, ok := payload[key]
	if !ok || value == nil {
		return false
	}

	return value.GetBoolValue()
}

func payloadDouble(payload map[string]*qdrant.Value, key string) float64 {
	if payload == nil {
		return 0
	}

	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}

	return value.GetDoubleValue()
}

func filteredFractalPeaks(klines []exchange.Kline) ([]indicators.Peak, error) {
	if len(klines) < billWilliamsFractalWin*2+1 {
		return nil, nil
	}

	highs, lows, closes := indicators.OHLCFromKlines(klines)

	highPeaks, err := indicators.FindExtremes(highs, billWilliamsFractalWin)
	if err != nil {
		return nil, err
	}

	lowPeaks, err := indicators.FindExtremes(lows, billWilliamsFractalWin)
	if err != nil {
		return nil, err
	}

	atr := indicators.ATRValues(highs, lows, closes, indicators.DefaultATRPeriod)
	if len(atr) == 0 {
		return nil, fmt.Errorf("not enough data for ATR")
	}

	filtered := append(
		indicators.FilterPeaksByATR(highs, highPeaks, atr, fractalATRFilterWindow, fractalATRMultiplier),
		indicators.FilterPeaksByATR(lows, lowPeaks, atr, fractalATRFilterWindow, fractalATRMultiplier)...,
	)

	return filtered, nil
}

// LastClose returns the latest candle close price (0 when no bars).
func (a *Marker) LastClose() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.klines) == 0 {
		return 0
	}
	return a.klines[len(a.klines)-1].Close
}

// LastATR returns the streaming volatility ATR from the latest tick.
func (a *Marker) LastATR() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.volatilityState.ATR
}

// VolatilityStateSnapshot returns a copy of the current volatility regime state.
func (a *Marker) VolatilityStateSnapshot() VolatilityState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.volatilityState
}

// ClosedVolatilityRegime returns the regime fixed at the last closed bar (layer2 snapshot).
func (a *Marker) ClosedVolatilityRegime() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return string(a.layer2Snap.volatilityState.Regime)
}

// Timeframe returns the marker's candle interval.
func (a *Marker) Timeframe() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.timeframe
}

// MarkerTailPollSnapshot holds tail-poll indicator values under one lock acquisition.
type MarkerTailPollSnapshot struct {
	Falcon    FalconSignals
	RSXColor  string
	RSXMarker string
}

// TailPollSnapshot copies the latest tail-poll fields for HTTP poll=1 responses.
func (a *Marker) TailPollSnapshot() MarkerTailPollSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return MarkerTailPollSnapshot{
		Falcon:    a.falconSignals,
		RSXColor:  RSXColor(a.jurikValue, a.jurikPrevBar),
		RSXMarker: a.rsxTradingMarkerAtCurrentBarLocked(),
	}
}

// FalconSnapshot returns a copy of the latest Falcon dashboard values.
func (a *Marker) FalconSnapshot() FalconSignals {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.falconSignals
}

// RSXSignalLine returns the Jurik RSX signal line value.
func (a *Marker) RSXSignalLine() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.falconSignals.JurikRSXSignal
}

// FibZonesSnapshot returns a defensive copy of active Fibonacci zones.
func (a *Marker) FibZonesSnapshot() []indicators.FibZone {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]indicators.FibZone(nil), a.fibZones...)
}

// RecentRSXMarker returns the RSX trading marker (L/LL/S/SS) on the current bar only.
func (a *Marker) RecentRSXMarker() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rsxTradingMarkerAtCurrentBarLocked()
}

// HasMinBars reports whether the marker has enough bars for scoring.
func (a *Marker) HasMinBars(min int) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.klines) >= min
}

// VectorSnapshot projects Marker state into vector_db.ReportSnapshot for Qdrant embeddings.
func (a *Marker) VectorSnapshot() vector_db.ReportSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	fibActive := false
	for _, zone := range a.fibZones {
		if zone.IsActive {
			fibActive = true
			break
		}
	}
	return vector_db.ReportSnapshot{
		JurikValue:      a.jurikValue,
		DivergenceScore: a.divSignal.Score,
		Regime:          string(a.volatilityState.Regime),
		FalconRedLine:   a.falconSignals.RedLine,
		FalconBlueLine:  a.falconSignals.BlueLine,
		FibActive:       fibActive,
	}
}

// lastConfirmedFractals returns the most recent filtered Bill Williams fractals,
// or the last candle High/Low when peaks cannot be computed.
func lastConfirmedFractals(klines []exchange.Kline) (up, down float64) {
	last := klines[len(klines)-1]

	peaks, err := filteredFractalPeaks(klines)
	if err != nil || len(peaks) == 0 {
		return last.High, last.Low
	}

	for i := len(peaks) - 1; i >= 0; i-- {
		switch peaks[i].Type {
		case indicators.PeakHigh:
			if up == 0 {
				up = peaks[i].Value
			}
		case indicators.PeakLow:
			if down == 0 {
				down = peaks[i].Value
			}
		}
		if up > 0 && down > 0 {
			break
		}
	}
	if up == 0 {
		up = last.High
	}
	if down == 0 {
		down = last.Low
	}
	return up, down
}
