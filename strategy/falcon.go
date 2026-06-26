package strategy

import "trading_bot/indicators"

const (
	wozduhChannelPeriod = 24
	wozduhChannelPhi    = 1.6185
	wozduhLenVol        = 24 // lenvol — RSI(volume-weighted close)
	wozduhWt11Period    = 12 // oo1 — EMA smoothing for wt11 (blue)
	wozduhWt22Period    = 5  // oo2 — EMA smoothing for wt22 (aqua)
	wozduhGreenEma      = 7  // ll — EMA(RSI close)
)

// FalconSignals holds the Wozdux/Wozduh dashboard values for a single tick.
type FalconSignals struct {
	JurikRSX       float64 // Main trend quality filter (hlc3 → Jurik RSX)
	JurikRSXSignal float64 // SMA(RSX, rsxSignalPeriod) signal line

	// Legacy names — used by scoring / FSM (unchanged semantics).
	RedLine   float64 // RSI(HL2)
	GreenLine float64 // EMA(RSI close)
	BlackLine float64 // MACD(RSI close) + 50
	BlueLine  float64 // wt11 / emarr — volume RSI fast line

	// Wozduh dashboard layers (TradingView reference colors).
	RsiPrice   float64 // RSI(close) — red
	EmaRsi     float64 // EMA(RSI close) — green
	RsiRsi     float64 // RSI(RSI close) — orange
	RsiHl2     float64 // RSI(HL2) — purple
	RsiVolFast float64 // wt11 = EMA(RSI(volClose), 12) — blue
	RsiVolSlow float64 // wt22 = EMA(RSI(volClose), 5) — aqua
	MacdRsi    float64 // MACD(RSI close) + 50 — black
	RsiAd      float64 // RSI(AD) — maroon
	RsiHl2Vol  float64 // RSI(VWEMA(HL2)) — navy
	VolCrossMarker string // "lime" bullish / "red" bearish wt11×wt22 cross

	// Vol channel (wt22 basis).
	VolChanMid float64 // SMA(wt22, 24) — orange
	VolChanUp  float64 // mid + φ·σ — blue dashed
	VolChanDn  float64 // mid - φ·σ — blue dashed

	// Price channel (rsiPrice basis).
	PriceChanMid float64 // SMA(rsiPrice, 24) — maroon
	PriceChanUp  float64 // mid + φ·σ — blue dashed
	PriceChanDn  float64 // mid - φ·σ — blue dashed
}

// FalconEngine runs the Jurik RSX + Wozdux indicator pipeline on each tick.
type FalconEngine struct {
	rsx        *indicators.JurikRSX
	rsxSignal  *indicators.RSXSignalLine
	redRsi     *indicators.RSI
	orangeRsi  *indicators.RSI
	greenEma   *indicators.EMA
	rsiOfRsi   *indicators.RSI
	blackRsi   *indicators.RSI
	blackMacd  *indicators.MACD
	volVwap    *indicators.VolumeWeightedEMA
	volRsi     *indicators.RSI
	wt11Ema    *indicators.EMA
	wt22Ema    *indicators.EMA
	navyVwp    *indicators.VolumeWeightedEMA
	navyRsi    *indicators.RSI
	wt22SMA    *indicators.SMA
	wt22Stdev  *indicators.RollingStDev
	priceSMA   *indicators.SMA
	priceStdev *indicators.RollingStDev
	ad         *indicators.AD
	adRsi      *indicators.RSI

	prevWt11    float64
	prevWt22    float64
	prevWtReady bool

	snapPrevWt11    float64
	snapPrevWt22    float64
	snapPrevWtReady bool
}

// NewFalconEngine creates a FalconEngine with default Wozdux/Jurik parameters.
func NewFalconEngine() *FalconEngine {
	settings := GetRSXSettings()
	return &FalconEngine{
		rsx:        indicators.NewJurikRSX(settings.Length),
		rsxSignal:  indicators.NewRSXSignalLine(settings.SignalLength),
		redRsi:     indicators.NewRSI(14),
		orangeRsi:  indicators.NewRSI(14),
		greenEma:   indicators.NewEMA(wozduhGreenEma),
		rsiOfRsi:   indicators.NewRSI(14),
		blackRsi:   indicators.NewRSI(wozduhLenVol),
		blackMacd:  indicators.NewMACD(7, wozduhLenVol, 9),
		volVwap:    indicators.NewVolumeWeightedEMA(wozduhLenVol),
		volRsi:     indicators.NewRSI(wozduhLenVol),
		wt11Ema:    indicators.NewEMA(wozduhWt11Period),
		wt22Ema:    indicators.NewEMA(wozduhWt22Period),
		navyVwp:    indicators.NewVolumeWeightedEMA(wozduhLenVol),
		navyRsi:    indicators.NewRSI(wozduhLenVol),
		wt22SMA:    indicators.NewSMA(wozduhChannelPeriod),
		wt22Stdev:  indicators.NewRollingStDev(wozduhChannelPeriod),
		priceSMA:   indicators.NewSMA(wozduhChannelPeriod),
		priceStdev: indicators.NewRollingStDev(wozduhChannelPeriod),
		ad:         indicators.NewAD(),
		adRsi:      indicators.NewRSI(14),
	}
}

func DetectVolCross(prevWt11, prevWt22, wt11, wt22 float64, ready bool) string {
	if !ready {
		return ""
	}
	if prevWt11 <= prevWt22 && wt11 > wt22 {
		return "lime"
	}
	if prevWt11 >= prevWt22 && wt11 < wt22 {
		return "red"
	}
	return ""
}

// SaveState stores indicator state at the last closed bar boundary.
func (e *FalconEngine) SaveState() {
	if e == nil {
		return
	}
	e.rsx.SaveState()
	e.rsxSignal.SaveState()
	e.redRsi.SaveState()
	e.orangeRsi.SaveState()
	e.greenEma.SaveState()
	e.rsiOfRsi.SaveState()
	e.blackRsi.SaveState()
	e.blackMacd.SaveState()
	e.volVwap.SaveState()
	e.volRsi.SaveState()
	e.wt11Ema.SaveState()
	e.wt22Ema.SaveState()
	e.navyVwp.SaveState()
	e.navyRsi.SaveState()
	e.wt22SMA.SaveState()
	e.wt22Stdev.SaveState()
	e.priceSMA.SaveState()
	e.priceStdev.SaveState()
	e.ad.SaveState()
	e.adRsi.SaveState()
	e.snapPrevWt11 = e.prevWt11
	e.snapPrevWt22 = e.prevWt22
	e.snapPrevWtReady = e.prevWtReady
}

// RestoreState rolls back to the last SaveState snapshot (before open-bar mutation).
func (e *FalconEngine) RestoreState() {
	if e == nil {
		return
	}
	e.rsx.RestoreState()
	e.rsxSignal.RestoreState()
	e.redRsi.RestoreState()
	e.orangeRsi.RestoreState()
	e.greenEma.RestoreState()
	e.rsiOfRsi.RestoreState()
	e.blackRsi.RestoreState()
	e.blackMacd.RestoreState()
	e.volVwap.RestoreState()
	e.volRsi.RestoreState()
	e.wt11Ema.RestoreState()
	e.wt22Ema.RestoreState()
	e.navyVwp.RestoreState()
	e.navyRsi.RestoreState()
	e.wt22SMA.RestoreState()
	e.wt22Stdev.RestoreState()
	e.priceSMA.RestoreState()
	e.priceStdev.RestoreState()
	e.ad.RestoreState()
	e.adRsi.RestoreState()
	e.prevWt11 = e.snapPrevWt11
	e.prevWt22 = e.snapPrevWt22
	e.prevWtReady = e.snapPrevWtReady
}

// Evaluate updates all streaming indicators and returns the current dashboard.
func (e *FalconEngine) Evaluate(high, low, close, volume float64) FalconSignals {
	hl2 := (high + low) / 2

	rsiPrice := e.orangeRsi.Update(close)
	emaRsi := e.greenEma.Update(rsiPrice)
	rsiRsi := e.rsiOfRsi.Update(rsiPrice)
	rsiHl2 := e.redRsi.Update(hl2)

	rsiForMacd := e.blackRsi.Update(close)
	macdRsi := e.blackMacd.Update(rsiForMacd) + 50.0

	// Pine: rsi11 = rsi(VWEMA(close), lenvol); wt11 = ema(rsi11, 12); wt22 = ema(rsi11, 5)
	volPrice := e.volVwap.Update(close, volume)
	rsi11 := e.volRsi.Update(volPrice)
	wt11 := e.wt11Ema.Update(rsi11)
	wt22 := e.wt22Ema.Update(rsi11)

	volCross := DetectVolCross(e.prevWt11, e.prevWt22, wt11, wt22, e.prevWtReady)
	e.prevWt11 = wt11
	e.prevWt22 = wt22
	e.prevWtReady = true

	aaacc := e.navyVwp.Update(hl2, volume)
	rsiHl2Vol := e.navyRsi.Update(aaacc)

	volChanMid := e.wt22SMA.Update(wt22)
	volOffs := wozduhChannelPhi * e.wt22Stdev.Update(wt22)

	priceChanMid := e.priceSMA.Update(rsiPrice)
	priceOffs := wozduhChannelPhi * e.priceStdev.Update(rsiPrice)

	adVal := e.ad.UpdateCandle(high, low, close)
	rsiAd := e.adRsi.Update(adVal)

	jurikRSX := e.rsx.Update(RSXSourcePrice(high, low, close, GetRSXSettings().Source))

	return FalconSignals{
		JurikRSX:       jurikRSX,
		JurikRSXSignal: e.rsxSignal.Update(jurikRSX),
		RedLine:        rsiHl2,
		GreenLine:      emaRsi,
		BlackLine:      macdRsi,
		BlueLine:       wt11,
		RsiPrice:       rsiPrice,
		EmaRsi:         emaRsi,
		RsiRsi:         rsiRsi,
		RsiHl2:         rsiHl2,
		RsiVolFast:     wt11,
		RsiVolSlow:     wt22,
		MacdRsi:        macdRsi,
		RsiAd:          rsiAd,
		RsiHl2Vol:      rsiHl2Vol,
		VolCrossMarker: volCross,
		VolChanMid:     volChanMid,
		VolChanUp:      volChanMid + volOffs,
		VolChanDn:      volChanMid - volOffs,
		PriceChanMid:   priceChanMid,
		PriceChanUp:    priceChanMid + priceOffs,
		PriceChanDn:    priceChanMid - priceOffs,
	}
}

// SetRSXSignalLength reconfigures the RSX signal line SMA and clears its buffer.
func (e *FalconEngine) SetRSXSignalLength(period int) {
	if e.rsxSignal == nil {
		e.rsxSignal = indicators.NewRSXSignalLine(period)
		return
	}
	e.rsxSignal.Reconfigure(period)
}

// SetRSXLength reconfigures the Jurik RSX smoothing period and clears its buffer.
func (e *FalconEngine) SetRSXLength(length int) {
	if e.rsx == nil {
		e.rsx = indicators.NewJurikRSX(length)
		return
	}
	e.rsx.Reconfigure(length)
}
