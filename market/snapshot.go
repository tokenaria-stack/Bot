package market

// streamingSnapshot holds Frame-level streaming state between closed bars.
type streamingSnapshot struct {
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
	annotations           []ChartAnnotation
	jurikLines            []float64
	wozduhRed             []float64
	wozduhGreen           []float64
}

func (a *Frame) restoreStreamingState() {
	live := a.captureDataBusLiveLocked()

	if a.volEngine != nil {
		a.volEngine.RestoreState()
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

	s := a.streamingSnap
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
	a.Annotations = append([]ChartAnnotation(nil), s.annotations...)
	a.restoreDataBusFromSnapLocked(s, live)
}

func (a *Frame) saveStreamingState() {
	a.alignAllDataBusToKlinesLocked()

	if a.volEngine != nil {
		a.volEngine.SaveState()
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

	a.streamingSnap = streamingSnapshot{
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
		annotations:           append([]ChartAnnotation(nil), a.Annotations...),
		jurikLines:            append([]float64(nil), a.JurikLines...),
		wozduhRed:             append([]float64(nil), a.WozduhRed...),
		wozduhGreen:           append([]float64(nil), a.WozduhGreen...),
	}
}
