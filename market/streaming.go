package market

import (
	"fmt"
	"strings"
	"time"

	"trading_bot/data"
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

func (a *Frame) resetStreamingEngines() {
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
	a.lastCommittedOpenTime = 0
}

// warmupStreaming replays Frame bars with live candle lifecycle (ADR-016):
// closed bars committed, optional forming tip evaluated isClosed=false and not committed.
func (a *Frame) warmupStreaming(klines []exchange.Kline) {
	a.resetStreamingEngines()
	a.replayLifecycleLocked(klines, time.Now().UnixMilli())
}

// replayStreamingLocked rebuilds streaming engines from a.klines using the same
// closed→forming lifecycle as live boot (ADR-016 Replay Lifecycle Ownership).
func (a *Frame) replayStreamingLocked() {
	klines := a.klines
	a.resetStreamingEngines()
	a.replayLifecycleLocked(klines, time.Now().UnixMilli())
	a.alignAllDataBusToKlinesLocked()
	a.clampDataBusToKlinesLocked()
}

// replayLifecycleLocked reproduces live tick semantics:
//   closed prefix → evaluate(isClosed=true) → commit last closed
//   optional forming tip → evaluate(isClosed=false) → never commit
func (a *Frame) replayLifecycleLocked(klines []exchange.Kline, nowMs int64) {
	closed, forming := splitLiveTail(klines, nowMs)
	for i, k := range closed {
		a.evaluateTickLocked(k, i, true)
	}
	a.markTailCommittedLocked(closed)
	if forming != nil {
		a.evaluateTickLocked(*forming, len(closed), false)
	}
}

// splitLiveTail partitions runtime bars by Cap forming predicate (data.IsFormingCloseTime).
// At most one trailing forming bar; no "last bar is forming" heuristic.
func splitLiveTail(klines []exchange.Kline, nowMs int64) (closed []exchange.Kline, forming *exchange.Kline) {
	n := len(klines)
	if n == 0 {
		return nil, nil
	}
	last := exchange.NormalizeKline(klines[n-1])
	if !data.IsFormingCloseTime(last.CloseTime, nowMs) {
		return klines, nil
	}
	tip := last
	if n == 1 {
		return nil, &tip
	}
	return klines[:n-1], &tip
}

// markTailCommittedLocked pins lastCommittedOpenTime to the tail bar of a closed-bar replay.
// Callers must pass closed bars only — never a forming tip (ADR-016).
func (a *Frame) markTailCommittedLocked(klines []exchange.Kline) {
	if len(klines) == 0 {
		return
	}
	a.lastCommittedOpenTime = klines[len(klines)-1].OpenTime
}

func (a *Frame) evaluateFalconSignalsLocked(k exchange.Kline, barIndex int, isClosed bool) {
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

func (a *Frame) evaluateTickLocked(k exchange.Kline, barIndex int, isClosed bool) {
	if EngineAllowsStrategies() && !a.bulkReplayMode {
		a.restoreStreamingState()
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

	// Divergence math still runs; RSX L/LL/S/SS chart labels purged in Phase F.
	a.divSignal, _ = a.divEngine.AnalyzeWithRSX(a, barIndex)
	a.cachedRSXMarkerBar = barIndex
	a.cachedRSXMarkerLabel = ""

	if isClosed && !a.bulkReplayMode {
		a.saveStreamingState()
	}
}

func (a *Frame) isNewZigZagNode(upd indicators.ZigZagUpdate) bool {
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
