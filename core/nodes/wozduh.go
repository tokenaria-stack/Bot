package nodes

import (
	"trading_bot/core"
	"trading_bot/indicators"
)

// Wozduh Pine periods — must stay bit-identical to strategy/falcon.go defaults.
const (
	wozduhChannelPeriod = 24
	wozduhChannelPhi    = 1.6185
	wozduhLenVol        = 24 // lenvol — RSI(volume-weighted close)
	wozduhWt11Period    = 12 // oo1 — EMA smoothing for wt11 (blue)
	wozduhWt22Period    = 5  // oo2 — EMA smoothing for wt22 (aqua)
	wozduhGreenEma      = 7  // ll — EMA(RSI close)
	wozduhRsiPeriod     = 14
	wozduhMacdFast      = 7
	wozduhMacdSlow      = wozduhLenVol
	wozduhMacdSignal    = 9

	// SlotWozduhVolCross encoding (bus is float64-only).
	wozduhVolCrossNone = 0.0
	wozduhVolCrossLime = 1.0
	wozduhVolCrossRed  = -1.0
)

// WozduhNode computes the full Wozduh / Wozdux Pine atom set into the data bus.
// Jurik RSX lives in RSXNode — not duplicated here.
type WozduhNode struct {
	bus *core.Bus

	redRsi    *indicators.RSI // RSI(HL2)
	orangeRsi *indicators.RSI // RSI(close)
	greenEma  *indicators.EMA // EMA(RSI close)
	rsiOfRsi  *indicators.RSI // RSI(RSI close)
	blackRsi  *indicators.RSI // RSI(close) feed for MACD
	blackMacd *indicators.MACD

	volVwap *indicators.VolumeWeightedEMA
	volRsi  *indicators.RSI
	wt11Ema *indicators.EMA
	wt22Ema *indicators.EMA

	navyVwp *indicators.VolumeWeightedEMA
	navyRsi *indicators.RSI

	wt22SMA    *indicators.SMA
	wt22Stdev  *indicators.RollingStDev
	priceSMA   *indicators.SMA
	priceStdev *indicators.RollingStDev

	ad    *indicators.AD
	adRsi *indicators.RSI

	prevWt11    float64
	prevWt22    float64
	prevWtReady bool

	snapPrevWt11    float64
	snapPrevWt22    float64
	snapPrevWtReady bool
}

// NewWozduhNode creates a full Wozduh atom pipeline with Falcon-default periods.
func NewWozduhNode() *WozduhNode {
	return &WozduhNode{
		redRsi:     indicators.NewRSI(wozduhRsiPeriod),
		orangeRsi:  indicators.NewRSI(wozduhRsiPeriod),
		greenEma:   indicators.NewEMA(wozduhGreenEma),
		rsiOfRsi:   indicators.NewRSI(wozduhRsiPeriod),
		blackRsi:   indicators.NewRSI(wozduhLenVol),
		blackMacd:  indicators.NewMACD(wozduhMacdFast, wozduhMacdSlow, wozduhMacdSignal),
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
		adRsi:      indicators.NewRSI(wozduhRsiPeriod),
	}
}

func (n *WozduhNode) Name() string { return "wozduh" }

func (n *WozduhNode) Init(bus *core.Bus) { n.bus = bus }

func (n *WozduhNode) Update() {
	if n.bus == nil || n.bus.Cur == nil {
		return
	}
	cur := n.bus.Cur
	high := cur.Get(core.SlotPriceHigh)
	low := cur.Get(core.SlotPriceLow)
	close := cur.Get(core.SlotPriceClose)
	volume := cur.Get(core.SlotVolume)
	hl2 := (high + low) / 2

	rsiPrice := n.orangeRsi.Update(close)
	emaRsi := n.greenEma.Update(rsiPrice)
	rsiRsi := n.rsiOfRsi.Update(rsiPrice)
	rsiHl2 := n.redRsi.Update(hl2)

	rsiForMacd := n.blackRsi.Update(close)
	macdRsi := n.blackMacd.Update(rsiForMacd) + 50.0

	// Pine: rsi11 = rsi(VWEMA(close), lenvol); wt11 = ema(rsi11, 12); wt22 = ema(rsi11, 5)
	volPrice := n.volVwap.Update(close, volume)
	rsi11 := n.volRsi.Update(volPrice)
	wt11 := n.wt11Ema.Update(rsi11)
	wt22 := n.wt22Ema.Update(rsi11)

	volCross := detectVolCrossCode(n.prevWt11, n.prevWt22, wt11, wt22, n.prevWtReady)
	n.prevWt11 = wt11
	n.prevWt22 = wt22
	n.prevWtReady = true

	aaacc := n.navyVwp.Update(hl2, volume)
	rsiHl2Vol := n.navyRsi.Update(aaacc)

	volChanMid := n.wt22SMA.Update(wt22)
	volOffs := wozduhChannelPhi * n.wt22Stdev.Update(wt22)

	priceChanMid := n.priceSMA.Update(rsiPrice)
	priceOffs := wozduhChannelPhi * n.priceStdev.Update(rsiPrice)

	adVal := n.ad.UpdateCandle(high, low, close)
	rsiAd := n.adRsi.Update(adVal)

	cur.Set(core.SlotWozduhRsiPrice, rsiPrice)
	cur.Set(core.SlotWozduhEmaRsi, emaRsi)
	cur.Set(core.SlotWozduhRsiRsi, rsiRsi)
	cur.Set(core.SlotWozduhRsiHl2, rsiHl2)
	cur.Set(core.SlotWozduhMacdRsi, macdRsi)
	cur.Set(core.SlotWozduhFast, wt11)
	cur.Set(core.SlotWozduhSlow, wt22)
	cur.Set(core.SlotWozduhRsiAd, rsiAd)
	cur.Set(core.SlotWozduhRsiHl2Vol, rsiHl2Vol)
	cur.Set(core.SlotWozduhVolChanMid, volChanMid)
	cur.Set(core.SlotWozduhVolChanUp, volChanMid+volOffs)
	cur.Set(core.SlotWozduhVolChanDn, volChanMid-volOffs)
	cur.Set(core.SlotWozduhPriceChanMid, priceChanMid)
	cur.Set(core.SlotWozduhPriceChanUp, priceChanMid+priceOffs)
	cur.Set(core.SlotWozduhPriceChanDn, priceChanMid-priceOffs)
	cur.Set(core.SlotWozduhVolCross, volCross)
}

func detectVolCrossCode(prevWt11, prevWt22, wt11, wt22 float64, ready bool) float64 {
	if !ready {
		return wozduhVolCrossNone
	}
	if prevWt11 <= prevWt22 && wt11 > wt22 {
		return wozduhVolCrossLime
	}
	if prevWt11 >= prevWt22 && wt11 < wt22 {
		return wozduhVolCrossRed
	}
	return wozduhVolCrossNone
}

func (n *WozduhNode) SaveState() {
	if n == nil {
		return
	}
	n.redRsi.SaveState()
	n.orangeRsi.SaveState()
	n.greenEma.SaveState()
	n.rsiOfRsi.SaveState()
	n.blackRsi.SaveState()
	n.blackMacd.SaveState()
	n.volVwap.SaveState()
	n.volRsi.SaveState()
	n.wt11Ema.SaveState()
	n.wt22Ema.SaveState()
	n.navyVwp.SaveState()
	n.navyRsi.SaveState()
	n.wt22SMA.SaveState()
	n.wt22Stdev.SaveState()
	n.priceSMA.SaveState()
	n.priceStdev.SaveState()
	n.ad.SaveState()
	n.adRsi.SaveState()
	n.snapPrevWt11 = n.prevWt11
	n.snapPrevWt22 = n.prevWt22
	n.snapPrevWtReady = n.prevWtReady
}

func (n *WozduhNode) RestoreState() {
	if n == nil {
		return
	}
	n.redRsi.RestoreState()
	n.orangeRsi.RestoreState()
	n.greenEma.RestoreState()
	n.rsiOfRsi.RestoreState()
	n.blackRsi.RestoreState()
	n.blackMacd.RestoreState()
	n.volVwap.RestoreState()
	n.volRsi.RestoreState()
	n.wt11Ema.RestoreState()
	n.wt22Ema.RestoreState()
	n.navyVwp.RestoreState()
	n.navyRsi.RestoreState()
	n.wt22SMA.RestoreState()
	n.wt22Stdev.RestoreState()
	n.priceSMA.RestoreState()
	n.priceStdev.RestoreState()
	n.ad.RestoreState()
	n.adRsi.RestoreState()
	n.prevWt11 = n.snapPrevWt11
	n.prevWt22 = n.snapPrevWt22
	n.prevWtReady = n.snapPrevWtReady
}

func (n *WozduhNode) OnConfigChange(any) error { return nil }

// Wt11Value exposes the wt11 EMA state (shadow validation / tests).
func (n *WozduhNode) Wt11Value() float64 {
	if n.wt11Ema == nil {
		return 0
	}
	return n.wt11Ema.Value()
}
