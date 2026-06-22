package strategy

import "trading_bot/indicators"

const (
	defaultATRPeriod   = 14
	defaultATRSMAPeriod = 20
	defaultVolSMAPeriod = 20

	climaxATRMult    = 2.0
	climaxVolMult    = 2.0
	climaxOscHigh    = 80.0
	climaxOscLow     = 20.0
	squeezeATRMult   = 0.8
	expansionStopMult = 1.5
	climaxStopMult   = 2.0

	climaxLotModifier  = 0.3
	squeezeLotModifier = 1.2
	expansionLotModifier = 1.0
)

// MarketRegime classifies the current volatility environment.
type MarketRegime string

const (
	RegimeSqueeze    MarketRegime = "SQUEEZE"
	RegimeExpansion  MarketRegime = "EXPANSION"
	RegimeClimax     MarketRegime = "CLIMAX"
)

// VolatilityState holds regime classification and risk-adjustment outputs.
type VolatilityState struct {
	Regime       MarketRegime
	ATR          float64
	SafeStopDist float64
	LotModifier  float64
}

// VolatilityEngine streams ATR/volume baselines and classifies market regime per tick.
type VolatilityEngine struct {
	atr    *indicators.ATR
	atrSma *indicators.SMA
	volSma *indicators.SMA
}

// NewVolatilityEngine creates a volatility engine with default Layer 1 indicators.
func NewVolatilityEngine() *VolatilityEngine {
	return &VolatilityEngine{
		atr:    indicators.NewATR(defaultATRPeriod),
		atrSma: indicators.NewSMA(defaultATRSMAPeriod),
		volSma: indicators.NewSMA(defaultVolSMAPeriod),
	}
}

// Evaluate updates streaming indicators and returns the current volatility state.
// primaryOscillator is the main oscillator (e.g. Jurik RSX or RSI) for climax detection.
func (e *VolatilityEngine) Evaluate(high, low, close, volume, primaryOscillator float64) VolatilityState {
	atrVal := e.atr.UpdateCandle(high, low, close)
	baselineATR := e.atrSma.Update(atrVal)
	baselineVol := e.volSma.Update(volume)

	state := defaultExpansionState(atrVal)

	if atrVal <= 0 || baselineATR <= 0 || baselineVol <= 0 {
		return state
	}

	if isClimax(atrVal, baselineATR, volume, baselineVol, primaryOscillator) {
		state.Regime = RegimeClimax
		state.LotModifier = climaxLotModifier
		state.SafeStopDist = atrVal * climaxStopMult
		return state
	}

	if atrVal < baselineATR*squeezeATRMult {
		state.Regime = RegimeSqueeze
		state.LotModifier = squeezeLotModifier
		state.SafeStopDist = atrVal * expansionStopMult
		return state
	}

	return state
}

func defaultExpansionState(atrVal float64) VolatilityState {
	stop := 0.0
	if atrVal > 0 {
		stop = atrVal * expansionStopMult
	}
	return VolatilityState{
		Regime:       RegimeExpansion,
		ATR:          atrVal,
		SafeStopDist: stop,
		LotModifier:  expansionLotModifier,
	}
}

func isClimax(atr, baselineATR, volume, baselineVol, osc float64) bool {
	return atr > baselineATR*climaxATRMult &&
		volume > baselineVol*climaxVolMult &&
		(osc > climaxOscHigh || osc < climaxOscLow)
}
