package strategy

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/qdrant/go-client/qdrant"

	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/indicators"
	"trading_bot/vector_db"
)

const (
	patternWindowSize       = 20
	similarPatternsLimit    = 3
	billWilliamsFractalWin  = 2
	fractalATRFilterWindow  = 5
	fractalATRMultiplier    = 1.0
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
	rsxMarkers               rsxMarkerState
	layer2Snap               layer2StreamingSnapshot
	mtfStates                map[string]*HTFState
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
		rsxMarkers: newRSXMarkerStateFromSettings(GetRSXSettings()),
	}
	a.warmupStreaming(copied)
	return a
}

// ReapplyRSXSettings rebuilds RSX marker state and replays streaming engines after setting changes.
func (a *Marker) ReapplyRSXSettings() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.replayStreamingLocked()
}

// JurikRSXColor returns the TradingView-style RSX line color for the latest bar.
func (a *Marker) JurikRSXColor() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return RSXColor(a.jurikValue, a.jurikPrevBar)
}

// LoadHistoricalKlines merges SQLite backfill with the live RAM buffer (overlay wins on
// duplicate OpenTime), then replays streaming engines once.
//
// Mutex contract: analyst.mu is a plain sync.Mutex (blocking wait, no timeout).
// UpdateKlineTick and LoadHistoricalKlines are the only writers; both call Lock once.
// replayStreamingLocked / evaluateTickLocked must never acquire mu — no re-entrant deadlock.
// While this runs, the WS data-feed goroutine blocks on Lock; OutCh (cap 1000) applies
// backpressure to the socket reader until hydrate completes.
func (a *Marker) LoadHistoricalKlines(klines []exchange.Kline) {
	a.mu.Lock()
	defer a.mu.Unlock()
	merged := mergeKlinesByOpenTime(klines, a.klines)
	a.klines = merged
	a.replayStreamingLocked()
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
		a.evaluateTickLocked(k, 0, isClosed)
		return
	}

	lastIdx := len(a.klines) - 1
	last := a.klines[lastIdx]

	if k.OpenTime == last.OpenTime {
		a.klines[lastIdx] = k
		a.evaluateTickLocked(k, lastIdx, isClosed)
		return
	}

	if k.OpenTime > last.OpenTime {
		// Commit the previous bar into streaming snapshots before opening a new one.
		a.evaluateTickLocked(last, lastIdx, true)
		a.klines = append(a.klines, k)
		a.evaluateTickLocked(k, len(a.klines)-1, isClosed)
		return
	}

	// Out-of-order tick for an earlier period — drop.
	_ = isClosed
}

// SetCurrentMTFState stores walk-forward HTF navigator state for scoring (keyed by interval).
func (a *Marker) SetCurrentMTFState(states map[string]*HTFState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(states) == 0 {
		a.mtfStates = nil
		return
	}
	a.mtfStates = make(map[string]*HTFState, len(states))
	for tf, st := range states {
		if st == nil {
			continue
		}
		cp := *st
		cp.TrendLines = append([]NavigatorLineDTO(nil), st.TrendLines...)
		cp.Markers = append([]NavigatorMarkerDTO(nil), st.Markers...)
		a.mtfStates[tf] = &cp
	}
}

// MTFState returns walk-forward HTF state for one interval (nil when unavailable).
func (a *Marker) MTFState(tf string) *HTFState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	st := a.mtfStates[tf]
	if st == nil {
		return nil
	}
	cp := *st
	cp.TrendLines = append([]NavigatorLineDTO(nil), st.TrendLines...)
	cp.Markers = append([]NavigatorMarkerDTO(nil), st.Markers...)
	return &cp
}

// MTFStates returns a defensive copy of all walk-forward HTF states.
func (a *Marker) MTFStates() map[string]*HTFState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.mtfStates) == 0 {
		return nil
	}
	out := make(map[string]*HTFState, len(a.mtfStates))
	for tf, st := range a.mtfStates {
		if st == nil {
			continue
		}
		cp := *st
		cp.TrendLines = append([]NavigatorLineDTO(nil), st.TrendLines...)
		cp.Markers = append([]NavigatorMarkerDTO(nil), st.Markers...)
		out[tf] = &cp
	}
	return out
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

// RecentRSXMarker returns the latest RSX trading marker string (L/LL/S/SS).
func (a *Marker) RecentRSXMarker() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rsxMarkers.recentTradingMarker(RSXSignalMemoryBars)
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
