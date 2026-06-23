package strategy

import (
	"fmt"
	"strings"

	"trading_bot/exchange"
	"trading_bot/indicators"
)

const (
	defaultZigZagSensitivity = 0.5
	falconCrossZoneCeil      = 40.0
	falconCrossZoneFloor     = 60.0
	wozduxVolumeSpikeDelta   = 15.0
	adTrendLookback          = 3
)

// ZigZagState holds the latest confirmed swing from the adaptive ZigZag.
type ZigZagState struct {
	Direction indicators.ZigZagDirection
	LastNode  indicators.ZigZagNode
}

func (a *Marker) resetStreamingEngines() {
	a.falcon = NewFalconEngine()
	a.volEngine = NewVolatilityEngine()
	a.divEngine = indicators.NewSmartDivergenceEngine()
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
	a.rsxMarkers = newRSXMarkerState(GetRSXSettings().DivLookback)
	settings := GetRSXSettings()
	a.falcon.SetRSXLength(settings.Length)
	a.falcon.SetRSXSignalLength(settings.SignalLength)
}

func (a *Marker) warmupStreaming(klines []exchange.Kline) {
	a.resetStreamingEngines()
	for i, k := range klines {
		a.evaluateTickLocked(k, i)
	}
}

func (a *Marker) replayStreamingLocked() {
	klines := a.klines
	a.resetStreamingEngines()
	for i, k := range klines {
		a.evaluateTickLocked(k, i)
	}
}

func (a *Marker) evaluateTickLocked(k exchange.Kline, barIndex int) {
	a.falconSignals = a.falcon.Evaluate(k.High, k.Low, k.Close, k.Volume)
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
	a.divSignal = combineDivSignals(a.divEngine.AnalyzeMacro(), a.divEngine.AnalyzeMicroCombined())
	a.rsxMarkers.appendBar(k.High, k.Low, k.Close, a.falconSignals.JurikRSX)
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
	return prevRed <= prevGreen && curRed > curGreen && curRed < falconCrossZoneCeil
}

func detectRedLineCrossGreenDown(prevRed, prevGreen, curRed, curGreen float64) bool {
	return prevRed >= prevGreen && curRed < curGreen && curRed > falconCrossZoneFloor
}

func detectWozduxVolumeSpikeUp(prevBlue, curBlue, redLine float64) bool {
	return DetectWozduxVolumeSpikeUp(prevBlue, curBlue, redLine)
}

func detectWozduxVolumeSpikeDown(prevBlue, curBlue, redLine float64) bool {
	return DetectWozduxVolumeSpikeDown(prevBlue, curBlue, redLine)
}

// DetectWozduxVolumeSpikeUp reports an anomalous volume spike at oversold levels.
func DetectWozduxVolumeSpikeUp(prevBlue, curBlue, redLine float64) bool {
	return curBlue-prevBlue > wozduxVolumeSpikeDelta && redLine < falconCrossZoneCeil
}

// DetectWozduxVolumeSpikeDown reports an anomalous volume spike at overbought levels.
func DetectWozduxVolumeSpikeDown(prevBlue, curBlue, redLine float64) bool {
	return prevBlue-curBlue > wozduxVolumeSpikeDelta && redLine > falconCrossZoneFloor
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
