package strategy

import (
	"fmt"
	"strings"

	"trading_bot/exchange"
	"trading_bot/indicators"
)

const (
	defaultZigZagSensitivity = 0.5
	wozduxVolumeSpikeDelta   = 15.0
	adTrendLookback          = 3
)

// ZigZagState holds the latest confirmed swing from the adaptive ZigZag.
type ZigZagState struct {
	Direction indicators.ZigZagDirection
	LastNode  indicators.ZigZagNode
}

func (a *Marker) resetStreamingEngines() {
	settings := a.effectiveRSXSettings()
	a.falcon = NewFalconEngine()
	a.volEngine = NewVolatilityEngine()
	a.divEngine = indicators.NewSmartDivergenceEngine(rsxScanConfigFromSettings(settings))
	a.zigzag = indicators.NewZigZag(indicators.DefaultATRPeriod)
	a.zigzag.SetSensitivity(defaultZigZagSensitivity)
	a.geometry = newGeometryTracker()
	a.orangeRsi = indicators.NewRSI(14)
	a.ad = indicators.NewAD()
	a.stoch = indicators.NewStochastic(14, 3, 3)
	a.ao = indicators.NewAO(a.config.AOFastPeriod, a.config.AOSlowPeriod)
	a.fibEngine = indicators.NewFibonacciEngine()
	a.latestAO = 0
	a.fibZones = nil
	a.fibWaveReady = false
	a.prevFalconRed = 0
	a.prevFalconGreen = 0
	a.prevFalconBlue = 0
	a.redLineCrossGreenUp = false
	a.redLineCrossGreenDown = false
	a.prevJurik = 0
	a.jurikPrevBar = 0
	a.jurikValue = 0
	a.jurikIsRising = false
	a.prevAO = 0
	a.prevAOReady = false
	a.adHistory = nil
	a.wozduxVolumeSpikeUp = false
	a.wozduxVolumeSpikeDown = false
	a.geometryBounceUp = false
	a.geometryBounceDown = false
	a.geometryTriangle = false
	a.accumulationRising = false
	a.distributionFalling = false
	a.aoCrossZeroUp = false
	a.aoCrossZeroDown = false
	a.prevZigHas = false
	a.falcon.SetRSXLength(settings.Length)
	a.falcon.SetRSXSignalLength(settings.SignalLength)
	// Falcon Jurik and RSX divergence scans share the same normalized source.
	a.falcon.SetRSXSource(settings.Source)
	a.Annotations = nil
	a.clearDataBusLocked()
	a.initDAGShadowLocked()
}

func (a *Marker) warmupStreaming(klines []exchange.Kline) {
	a.resetStreamingEngines()
	for i, k := range klines {
		a.evaluateTickLocked(k, i, true)
	}
}

func (a *Marker) replayStreamingLocked() {
	klines := a.klines
	a.resetStreamingEngines()
	for i, k := range klines {
		a.evaluateTickLocked(k, i, true)
	}
	a.ensureChartExportPointsAlignedLocked()
	a.clampDataBusToKlinesLocked()
}

// layer2StreamingSnapshot holds Marker-level Layer 2 state between closed bars.
type layer2StreamingSnapshot struct {
	adHistory             []float64
	prevAO                float64
	prevAOReady           bool
	latestAO              float64
	prevJurik             float64
	jurikPrevBar          float64
	jurikValue            float64
	jurikIsRising         bool
	prevFalconRed         float64
	prevFalconGreen       float64
	prevFalconBlue        float64
	redLineCrossGreenUp   bool
	redLineCrossGreenDown bool
	wozduxVolumeSpikeUp   bool
	wozduxVolumeSpikeDown bool
	accumulationRising    bool
	distributionFalling   bool
	aoCrossZeroUp         bool
	aoCrossZeroDown       bool
	volatilityState       VolatilityState
	divSignal             indicators.DivSignal
	annotations           []ChartAnnotation
	jurikLines            []float64
	wozduhRed             []float64
	wozduhGreen           []float64
	chartExportPoints     []BacktestChartPoint
}

func (a *Marker) restoreLayer2StreamingState() {
	live := a.captureDataBusLiveLocked()

	if a.volEngine != nil {
		a.volEngine.RestoreState()
	}
	if a.orangeRsi != nil {
		a.orangeRsi.RestoreState()
	}
	if a.ad != nil {
		a.ad.RestoreState()
	}
	if a.stoch != nil {
		a.stoch.RestoreState()
	}
	if a.ao != nil {
		a.ao.RestoreState()
	}
	if a.divEngine != nil {
		a.divEngine.RestoreState()
	}

	s := a.layer2Snap
	a.adHistory = append(a.adHistory[:0], s.adHistory...)
	a.prevAO = s.prevAO
	a.prevAOReady = s.prevAOReady
	a.latestAO = s.latestAO
	a.prevJurik = s.prevJurik
	a.jurikPrevBar = s.jurikPrevBar
	a.jurikValue = s.jurikValue
	a.jurikIsRising = s.jurikIsRising
	a.prevFalconRed = s.prevFalconRed
	a.prevFalconGreen = s.prevFalconGreen
	a.prevFalconBlue = s.prevFalconBlue
	a.redLineCrossGreenUp = s.redLineCrossGreenUp
	a.redLineCrossGreenDown = s.redLineCrossGreenDown
	a.wozduxVolumeSpikeUp = s.wozduxVolumeSpikeUp
	a.wozduxVolumeSpikeDown = s.wozduxVolumeSpikeDown
	a.accumulationRising = s.accumulationRising
	a.distributionFalling = s.distributionFalling
	a.aoCrossZeroUp = s.aoCrossZeroUp
	a.aoCrossZeroDown = s.aoCrossZeroDown
	a.volatilityState = s.volatilityState
	a.divSignal = s.divSignal
	a.Annotations = append([]ChartAnnotation(nil), s.annotations...)
	a.restoreDataBusFromSnapLocked(s, live)
}

func (a *Marker) saveLayer2StreamingState() {
	a.alignAllDataBusToKlinesLocked()

	if a.volEngine != nil {
		a.volEngine.SaveState()
	}
	if a.orangeRsi != nil {
		a.orangeRsi.SaveState()
	}
	if a.ad != nil {
		a.ad.SaveState()
	}
	if a.stoch != nil {
		a.stoch.SaveState()
	}
	if a.ao != nil {
		a.ao.SaveState()
	}
	if a.divEngine != nil {
		a.divEngine.SaveState()
	}

	a.layer2Snap = layer2StreamingSnapshot{
		adHistory:             append([]float64(nil), a.adHistory...),
		prevAO:                a.prevAO,
		prevAOReady:           a.prevAOReady,
		latestAO:              a.latestAO,
		prevJurik:             a.prevJurik,
		jurikPrevBar:          a.jurikPrevBar,
		jurikValue:            a.jurikValue,
		jurikIsRising:         a.jurikIsRising,
		prevFalconRed:         a.prevFalconRed,
		prevFalconGreen:       a.prevFalconGreen,
		prevFalconBlue:        a.prevFalconBlue,
		redLineCrossGreenUp:   a.redLineCrossGreenUp,
		redLineCrossGreenDown: a.redLineCrossGreenDown,
		wozduxVolumeSpikeUp:   a.wozduxVolumeSpikeUp,
		wozduxVolumeSpikeDown: a.wozduxVolumeSpikeDown,
		accumulationRising:    a.accumulationRising,
		distributionFalling:   a.distributionFalling,
		aoCrossZeroUp:         a.aoCrossZeroUp,
		aoCrossZeroDown:       a.aoCrossZeroDown,
		volatilityState:       a.volatilityState,
		divSignal:             a.divSignal,
		annotations:           append([]ChartAnnotation(nil), a.Annotations...),
		jurikLines:            append([]float64(nil), a.JurikLines...),
		wozduhRed:             append([]float64(nil), a.WozduhRed...),
		wozduhGreen:           append([]float64(nil), a.WozduhGreen...),
		chartExportPoints:     append([]BacktestChartPoint(nil), a.chartExportPoints...),
	}
}

func (a *Marker) evaluateFalconSignalsLocked(k exchange.Kline, barIndex int, isClosed bool) {
	// Shot 9F: ChartOnly skips Falcon.Evaluate; DAG always runs for Projector/WS plots.
	if EngineAllowsStrategies() && a.falcon != nil {
		a.falcon.RestoreState()
		a.falconSignals = a.falcon.Evaluate(k.High, k.Low, k.Close, k.Volume)
		if isClosed {
			a.falcon.SaveState()
		}
	}
	a.runDAGShadowLocked(k, barIndex, isClosed)
}

func (a *Marker) evaluateTickLocked(k exchange.Kline, barIndex int, isClosed bool) {
	if EngineAllowsStrategies() && !a.bulkReplayMode {
		a.restoreLayer2StreamingState()
	}
	// DAG (and Falcon when Live) — chart delivery oxygen.
	a.evaluateFalconSignalsLocked(k, barIndex, isClosed)
	if !EngineAllowsStrategies() {
		return
	}

	curRed := a.falconSignals.RedLine
	curGreen := a.falconSignals.GreenLine
	a.redLineCrossGreenUp = detectRedLineCrossGreenUp(a.prevFalconRed, a.prevFalconGreen, curRed, curGreen)
	a.redLineCrossGreenDown = detectRedLineCrossGreenDown(a.prevFalconRed, a.prevFalconGreen, curRed, curGreen)
	curBlue := a.falconSignals.BlueLine
	a.wozduxVolumeSpikeUp = detectWozduxVolumeSpikeUp(a.prevFalconBlue, curBlue, curRed)
	a.wozduxVolumeSpikeDown = detectWozduxVolumeSpikeDown(a.prevFalconBlue, curBlue, curRed)
	a.prevFalconRed = curRed
	a.prevFalconGreen = curGreen
	a.prevFalconBlue = curBlue

	curJurik := a.falconSignals.JurikRSX
	a.jurikPrevBar = a.prevJurik
	a.jurikValue = curJurik
	a.jurikIsRising = curJurik > a.prevJurik
	a.prevJurik = curJurik

	a.volatilityState = a.volEngine.Evaluate(k.High, k.Low, k.Close, k.Volume, curJurik)

	hl2 := (k.High + k.Low) / 2
	orange := a.orangeRsi.Update(k.Close)
	a.divEngine.UpdateMicroTick(orange, a.falconSignals.RedLine)

	adVal := a.ad.UpdateCandle(k.High, k.Low, k.Close)
	a.accumulationRising, a.distributionFalling = detectADFlow(adVal, a.adHistory)
	a.adHistory = appendADHistory(a.adHistory, adVal, adTrendLookback+1)

	curAO := a.ao.Update(hl2)
	if a.prevAOReady {
		a.aoCrossZeroUp = a.prevAO <= 0 && curAO > 0
		a.aoCrossZeroDown = a.prevAO >= 0 && curAO < 0
	} else {
		a.aoCrossZeroUp = false
		a.aoCrossZeroDown = false
	}
	a.prevAO = curAO
	a.prevAOReady = true
	a.latestAO = curAO
	stochVal := a.stoch.UpdateCandle(k.High, k.Low, k.Close)

	zzUpd := a.zigzag.UpdateCandle(k.High, k.Low, k.Close, a.falconSignals.JurikRSX)
	a.zigZagState = ZigZagState{
		Direction: zzUpd.Direction,
		LastNode:  zzUpd.Node,
	}

	if a.isNewZigZagNode(zzUpd) {
		a.divEngine.UpdateSnapshot(indicators.Snapshot{
			Index:      barIndex,
			IsHigh:     zzUpd.Node.IsHigh,
			Price:      zzUpd.Node.Price,
			Jurik:      a.falconSignals.JurikRSX,
			OrangeRSI:  orange,
			RedRSI:     a.falconSignals.RedLine,
			BlueVolume: a.falconSignals.BlueLine,
			BurgundyAD: adVal,
			AO:         a.latestAO,
			MACD:       a.falconSignals.BlackLine,
			Stoch:      stochVal,
		})
		a.geometry.onSwingNode(barIndex, zzUpd.Node)
		if a.prevZigHas {
			a.fibWaveStart = a.prevZigNode.Price
			a.fibWaveEnd = zzUpd.Node.Price
			a.fibWaveReady = true
		}
		a.prevZigNode = zzUpd.Node
		a.prevZigHas = true
	}

	atr := a.volatilityState.ATR
	if a.fibWaveReady {
		a.fibZones = a.fibEngine.CalculatePriceZones(a.fibWaveStart, a.fibWaveEnd, k.Close, atr)
	}
	a.geometryState = a.geometry.updateBar(barIndex, k.High, k.Low, k.Close, k.Open, k.Volume, atr)
	a.geometryBounceUp = a.geometryState.BounceUp
	a.geometryBounceDown = a.geometryState.BounceDown
	a.geometryTriangle = a.geometryState.TriangleKind != ""
	a.recordDataBusBarLocked(barIndex, a.falconSignals)

	var divAnn *indicators.DivAnnotation
	a.divSignal, divAnn = a.divEngine.AnalyzeWithRSX(a, barIndex)
	a.cachedRSXMarkerBar = barIndex
	a.cachedRSXMarkerLabel = ""
	if divAnn != nil && divAnn.Label != "" {
		a.cachedRSXMarkerLabel = divAnn.Label
	}
	if isClosed && divAnn != nil {
		a.appendRSXAnnotationLocked(a.chartAnnotationFromDivAnn(*divAnn))
	}

	if barIndex >= 0 && barIndex < len(a.klines) {
		a.recordChartExportPointLocked(barIndex, k, a.falconSignals)
	}

	if isClosed && !a.bulkReplayMode {
		a.saveLayer2StreamingState()
	}
}

func (a *Marker) isNewZigZagNode(upd indicators.ZigZagUpdate) bool {
	if !upd.Node.Confirmed {
		return false
	}
	if !a.prevZigHas {
		return true
	}
	return upd.Node.Price != a.prevZigNode.Price || upd.Node.IsHigh != a.prevZigNode.IsHigh
}

func detectRedLineCrossGreenUp(prevRed, prevGreen, curRed, curGreen float64) bool {
	return prevRed <= prevGreen && curRed > curGreen
}

func detectRedLineCrossGreenDown(prevRed, prevGreen, curRed, curGreen float64) bool {
	return prevRed >= prevGreen && curRed < curGreen
}

func detectWozduxVolumeSpikeUp(prevBlue, curBlue, redLine float64) bool {
	return DetectWozduxVolumeSpikeUp(prevBlue, curBlue, redLine)
}

func detectWozduxVolumeSpikeDown(prevBlue, curBlue, redLine float64) bool {
	return DetectWozduxVolumeSpikeDown(prevBlue, curBlue, redLine)
}

// DetectWozduxVolumeSpikeUp reports an anomalous volume spike (no RSI zone veto).
func DetectWozduxVolumeSpikeUp(prevBlue, curBlue, _ float64) bool {
	return curBlue-prevBlue > wozduxVolumeSpikeDelta
}

// DetectWozduxVolumeSpikeDown reports an anomalous volume spike (no RSI zone veto).
func DetectWozduxVolumeSpikeDown(prevBlue, curBlue, _ float64) bool {
	return prevBlue-curBlue > wozduxVolumeSpikeDelta
}

func appendADHistory(history []float64, value float64, maxLen int) []float64 {
	history = append(history, value)
	if len(history) > maxLen {
		history = history[len(history)-maxLen:]
	}
	return history
}

func detectADFlow(curAD float64, history []float64) (rising, falling bool) {
	if len(history) == 0 {
		return false, false
	}

	prev := history[len(history)-1]
	if curAD > prev {
		rising = true
	}
	if curAD < prev {
		falling = true
	}

	if len(history) >= adTrendLookback {
		start := len(history) - adTrendLookback
		sum := 0.0
		for _, v := range history[start:] {
			sum += v
		}
		avg := sum / float64(adTrendLookback)
		if curAD > avg {
			rising = true
		}
		if curAD < avg {
			falling = true
		}
	}

	return rising, falling
}

func combineDivSignals(macro indicators.DivSignal, microScore int) indicators.DivSignal {
	score := macro.Score + microScore
	if score > 100 {
		score = 100
	}
	if score < -100 {
		score = -100
	}

	desc := macro.Description
	if microScore != 0 {
		microPart := fmt.Sprintf("Micro (%+d)", microScore)
		if desc != "" {
			desc = strings.Join([]string{desc, microPart}, "; ")
		} else {
			desc = microPart
		}
	}

	return indicators.DivSignal{
		Score:       score,
		Description: desc,
	}
}
