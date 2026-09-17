package market

import (
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
	a.volEngine = NewVolatilityEngine()
	a.zigzag = indicators.NewZigZag(indicators.DefaultATRPeriod)
	a.zigzag.SetSensitivity(defaultZigZagSensitivity)
	a.geometry = newGeometryTracker()
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
	a.Annotations = nil
	a.rsxTVFacts = nil
	a.rstv = nil
	a.rsxZZFacts = nil
	a.zzCollector = ZZDivFactCollector{}
	a.rsxFractalFacts = nil
	a.rsxFractalPrices = nil
	a.rsxFractalOsc = nil
	a.rsxFractalOpens = nil
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
//
//	closed prefix → evaluate(isClosed=true) → commit last closed
//	optional forming tip → evaluate(isClosed=false) → never commit
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
	last := klines[n-1]
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
	a.runDAGShadowLocked(k, barIndex, isClosed)
	a.noteRSTVFactLocked(isClosed, barIndex)
	a.noteRSTZZFactLocked(isClosed, barIndex)
	a.noteRSTFractalFactLocked(isClosed, barIndex)
}

func (a *Frame) evaluateTickLocked(k exchange.Kline, barIndex int, isClosed bool) {
	a.evaluateFalconSignalsLocked(k, barIndex, isClosed)
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
