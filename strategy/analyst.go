package strategy

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/qdrant/go-client/qdrant"

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
		rsxMarkers: newRSXMarkerState(GetRSXSettings().DivLookback),
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

// UpdateKline appends a new candle or overwrites the latest one for the same open time.
func (a *Marker) UpdateKline(k exchange.Kline) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.klines) == 0 || k.OpenTime > a.klines[len(a.klines)-1].OpenTime {
		a.klines = append(a.klines, k)
		a.evaluateTickLocked(k, len(a.klines)-1)
		return
	}

	lastIdx := len(a.klines) - 1
	if k.OpenTime == a.klines[lastIdx].OpenTime {
		a.klines[lastIdx] = k
		a.replayStreamingLocked()
	}
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

// Report contains a standardized market summary for MasterGeneral.
type Report struct {
	Timeframe    string
	Close        float64
	RSI          float64
	MACD         float64
	ATR          float64
	IsOverbought bool
	IsOversold   bool
	LatestAO     float64
	UpFractal    float64
	DownFractal  float64
	Falcon       FalconSignals
	Volatility   VolatilityState
	Divergence   indicators.DivSignal
	ZigZag       ZigZagState
	Geometry     GeometryState
	FibZones     []indicators.FibZone
	RedLineCrossGreenUp   bool
	RedLineCrossGreenDown bool
	JurikValue            float64
	JurikIsRising         bool
	WozduxVolumeSpikeUp   bool // anomalous volume at oversold bottom (green dot)
	WozduxVolumeSpikeDown bool // anomalous volume at overbought top (red dot)
	GeometryBounceUp      bool // bounce off support trendline
	GeometryBounceDown    bool // bounce off resistance trendline
	GeometryTriangle      bool // price compressed in triangle (energy building)
	AccumulationRising    bool // AD line climbing (whale accumulation)
	DistributionFalling   bool // AD line falling (whale distribution)
	AOCrossZeroUp         bool // AO histogram crossed zero upward
	AOCrossZeroDown       bool // AO histogram crossed zero downward
	RSXMarker             string // L, LL, S, SS, P from RSX chart pivot/divergence scan
	RSXSignal             float64 // SMA(RSX, 9) signal line
}

// GenerateMarketReport collects current indicator values into a unified summary.
func (a *Marker) GenerateMarketReport() (*Report, error) {
	klines := a.GetKlines()
	if len(klines) < 50 {
		return nil, fmt.Errorf("not enough klines to generate full report: %d", len(klines))
	}

	_, high, low, closePrices, _ := indicators.ExtractPrices(klines)
	lastClose := closePrices[len(closePrices)-1]

	rsiArr := indicators.RSIValues(closePrices, 14)
	var lastRSI float64
	if len(rsiArr) > 0 {
		lastRSI = rsiArr[len(rsiArr)-1]
	}

	macdArr, _, _ := indicators.MACDValues(closePrices, 12, 26, 9)
	var lastMACD float64
	if len(macdArr) > 0 {
		lastMACD = macdArr[len(macdArr)-1]
	}

	atrArr := indicators.ATRValues(high, low, closePrices, 14)
	var lastATR float64
	if len(atrArr) > 0 {
		lastATR = atrArr[len(atrArr)-1]
	}

	upFractal, downFractal := lastConfirmedFractals(klines)

	a.mu.RLock()
	falcon := a.falconSignals
	volatility := a.volatilityState
	divergence := a.divSignal
	zigZag := a.zigZagState
	geometry := a.geometryState
	latestAO := a.latestAO
	fibZones := append([]indicators.FibZone(nil), a.fibZones...)
	redCross := a.redLineCrossGreenUp
	redCrossDown := a.redLineCrossGreenDown
	jurikValue := a.jurikValue
	jurikRising := a.jurikIsRising
	volSpikeUp := a.wozduxVolumeSpikeUp
	volSpikeDown := a.wozduxVolumeSpikeDown
	geomBounceUp := a.geometryBounceUp
	geomBounceDown := a.geometryBounceDown
	geomTriangle := a.geometryTriangle
	adRising := a.accumulationRising
	adFalling := a.distributionFalling
	aoCrossUp := a.aoCrossZeroUp
	aoCrossDown := a.aoCrossZeroDown
	a.mu.RUnlock()

	rsxMarker := LatestRSXChartMarker(klines, GetRSXSettings().DivLookback)

	return &Report{
		Timeframe:    a.timeframe,
		Close:        lastClose,
		RSI:          lastRSI,
		MACD:         lastMACD,
		ATR:          lastATR,
		IsOverbought: lastRSI >= 70,
		IsOversold:   lastRSI <= 30,
		LatestAO:     latestAO,
		UpFractal:    upFractal,
		DownFractal:  downFractal,
		Falcon:       falcon,
		Volatility:   volatility,
		Divergence:   divergence,
		ZigZag:       zigZag,
		Geometry:     geometry,
		FibZones:     fibZones,
		RedLineCrossGreenUp:   redCross,
		RedLineCrossGreenDown: redCrossDown,
		JurikValue:            jurikValue,
		JurikIsRising:         jurikRising,
		WozduxVolumeSpikeUp:   volSpikeUp,
		WozduxVolumeSpikeDown: volSpikeDown,
		GeometryBounceUp:      geomBounceUp,
		GeometryBounceDown:    geomBounceDown,
		GeometryTriangle:      geomTriangle,
		AccumulationRising:    adRising,
		DistributionFalling:   adFalling,
		AOCrossZeroUp:         aoCrossUp,
		AOCrossZeroDown:       aoCrossDown,
		RSXMarker:             rsxMarker,
		RSXSignal:             falcon.JurikRSXSignal,
	}, nil
}

// GenerateStreamingReport builds a Report from incremental indicator state (O(1) per call).
// Use for backtests instead of GenerateMarketReport to avoid rescanning full history.
func (a *Marker) GenerateStreamingReport() (Report, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if len(a.klines) < 50 {
		return Report{}, fmt.Errorf("not enough klines to generate full report: %d", len(a.klines))
	}

	last := a.klines[len(a.klines)-1]
	lastRSI := a.orangeRsi.Value()
	lastMACD := a.falconSignals.BlackLine - 50.0

	upFractal := last.High
	downFractal := last.Low
	if a.zigZagState.LastNode.Confirmed {
		if a.zigZagState.LastNode.IsHigh {
			upFractal = a.zigZagState.LastNode.Price
		} else {
			downFractal = a.zigZagState.LastNode.Price
		}
	}

	fibZones := append([]indicators.FibZone(nil), a.fibZones...)

	return Report{
		Timeframe:             a.timeframe,
		Close:                 last.Close,
		RSI:                   lastRSI,
		MACD:                  lastMACD,
		ATR:                   a.volatilityState.ATR,
		IsOverbought:          lastRSI >= 70,
		IsOversold:            lastRSI <= 30,
		LatestAO:              a.latestAO,
		UpFractal:             upFractal,
		DownFractal:           downFractal,
		Falcon:                a.falconSignals,
		Volatility:            a.volatilityState,
		Divergence:            a.divSignal,
		ZigZag:                a.zigZagState,
		Geometry:              a.geometryState,
		FibZones:              fibZones,
		RedLineCrossGreenUp:   a.redLineCrossGreenUp,
		RedLineCrossGreenDown: a.redLineCrossGreenDown,
		JurikValue:            a.jurikValue,
		JurikIsRising:         a.jurikIsRising,
		WozduxVolumeSpikeUp:   a.wozduxVolumeSpikeUp,
		WozduxVolumeSpikeDown: a.wozduxVolumeSpikeDown,
		GeometryBounceUp:      a.geometryBounceUp,
		GeometryBounceDown:    a.geometryBounceDown,
		GeometryTriangle:      a.geometryTriangle,
		AccumulationRising:    a.accumulationRising,
		DistributionFalling:   a.distributionFalling,
		AOCrossZeroUp:         a.aoCrossZeroUp,
		AOCrossZeroDown:       a.aoCrossZeroDown,
		RSXMarker:             a.rsxMarkers.recentTradingMarker(RSXSignalMemoryBars),
		RSXSignal:             a.falconSignals.JurikRSXSignal,
	}, nil
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
