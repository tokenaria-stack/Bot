package nodes

import (
	"math"

	"trading_bot/core"
	"trading_bot/indicators"
)

// Wozduh Pine periods — must stay bit-identical to market/falcon.go defaults.
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
	bus  *core.Bus
	mask WozduhMask

	streamUpdates int

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

// NewWozduhNode creates a full Wozduh atom pipeline (explicit compute-all mask).
func NewWozduhNode() *WozduhNode {
	return NewWozduhNodeMasked(WozduhMaskAll)
}

// NewWozduhNodeMasked creates a Wozduh pipeline that runs only the given compute bits.
// Mask is fixed for the node's lifetime. Zero means no streams run.
func NewWozduhNodeMasked(mask WozduhMask) *WozduhNode {
	return &WozduhNode{
		mask:       mask,
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

	if n.mask&WozduhBitOrangeBase != 0 {
		rsiPrice := n.orangeRsi.Update(close)
		n.noteStream()
		cur.Set(core.SlotWozduhRsiPrice, rsiPrice)
		if n.mask&WozduhBitGreenEMA != 0 {
			cur.Set(core.SlotWozduhEmaRsi, n.greenEma.Update(rsiPrice))
			n.noteStream()
		}
		if n.mask&WozduhBitRsiOfRsi != 0 {
			cur.Set(core.SlotWozduhRsiRsi, n.rsiOfRsi.Update(rsiPrice))
			n.noteStream()
		}
		if n.mask&WozduhBitPriceChannel != 0 {
			priceChanMid := n.priceSMA.Update(rsiPrice)
			n.noteStream()
			priceOffs := wozduhChannelPhi * n.priceStdev.Update(rsiPrice)
			n.noteStream()
			cur.Set(core.SlotWozduhPriceChanMid, priceChanMid)
			cur.Set(core.SlotWozduhPriceChanUp, priceChanMid+priceOffs)
			cur.Set(core.SlotWozduhPriceChanDn, priceChanMid-priceOffs)
		}
	}

	if n.mask&WozduhBitRedRSI != 0 {
		cur.Set(core.SlotWozduhRsiHl2, n.redRsi.Update(hl2))
		n.noteStream()
	}

	if n.mask&WozduhBitBlackMACD != 0 {
		rsiForMacd := n.blackRsi.Update(close)
		n.noteStream()
		cur.Set(core.SlotWozduhMacdRsi, n.blackMacd.Update(rsiForMacd)+50.0)
		n.noteStream()
	}

	if n.mask&WozduhBitVolBase != 0 {
		volPrice := n.volVwap.Update(close, volume)
		n.noteStream()
		rsi11 := n.volRsi.Update(volPrice)
		n.noteStream()
		var wt11, wt22 float64
		if n.mask&WozduhBitWt11 != 0 {
			wt11 = n.wt11Ema.Update(rsi11)
			n.noteStream()
			cur.Set(core.SlotWozduhFast, wt11)
		}
		if n.mask&WozduhBitWt22 != 0 {
			wt22 = n.wt22Ema.Update(rsi11)
			n.noteStream()
			cur.Set(core.SlotWozduhSlow, wt22)
			if n.mask&WozduhBitVolChannel != 0 {
				volChanMid := n.wt22SMA.Update(wt22)
				n.noteStream()
				volOffs := wozduhChannelPhi * n.wt22Stdev.Update(wt22)
				n.noteStream()
				cur.Set(core.SlotWozduhVolChanMid, volChanMid)
				cur.Set(core.SlotWozduhVolChanUp, volChanMid+volOffs)
				cur.Set(core.SlotWozduhVolChanDn, volChanMid-volOffs)
			}
		}
		if n.mask&WozduhBitVolCrossPair != 0 {
			volCross := detectVolCrossCode(n.prevWt11, n.prevWt22, wt11, wt22, n.prevWtReady)
			n.prevWt11 = wt11
			n.prevWt22 = wt22
			n.prevWtReady = true
			cur.Set(core.SlotWozduhVolCross, volCross)
		}
	}

	if n.mask&WozduhBitNavyRSI != 0 {
		aaacc := n.navyVwp.Update(hl2, volume)
		n.noteStream()
		cur.Set(core.SlotWozduhRsiHl2Vol, n.navyRsi.Update(aaacc))
		n.noteStream()
	}

	if n.mask&WozduhBitADRSI != 0 {
		adVal := n.ad.UpdateCandle(high, low, close)
		n.noteStream()
		cur.Set(core.SlotWozduhRsiAd, n.adRsi.Update(adVal))
		n.noteStream()
	}

	n.failClosedInactive(cur)
}

func (n *WozduhNode) noteStream() {
	n.streamUpdates++
}

// StreamUpdates is the count of streaming sub-indicator Update calls (tests/measure).
func (n *WozduhNode) StreamUpdates() int {
	if n == nil {
		return 0
	}
	return n.streamUpdates
}

// Mask returns the fixed compute mask this node was created with.
func (n *WozduhNode) Mask() WozduhMask {
	if n == nil {
		return 0
	}
	return n.mask
}

func (n *WozduhNode) failClosedInactive(cur *core.TickFrame) {
	nan := math.NaN()
	if n.mask&WozduhBitOrangeBase == 0 {
		cur.Set(core.SlotWozduhRsiPrice, nan)
	}
	if n.mask&WozduhBitGreenEMA == 0 {
		cur.Set(core.SlotWozduhEmaRsi, nan)
	}
	if n.mask&WozduhBitRsiOfRsi == 0 {
		cur.Set(core.SlotWozduhRsiRsi, nan)
	}
	if n.mask&WozduhBitPriceChannel == 0 {
		cur.Set(core.SlotWozduhPriceChanMid, nan)
		cur.Set(core.SlotWozduhPriceChanUp, nan)
		cur.Set(core.SlotWozduhPriceChanDn, nan)
	}
	if n.mask&WozduhBitWt11 == 0 {
		cur.Set(core.SlotWozduhFast, nan)
	}
	if n.mask&WozduhBitWt22 == 0 {
		cur.Set(core.SlotWozduhSlow, nan)
	}
	if n.mask&WozduhBitVolChannel == 0 {
		cur.Set(core.SlotWozduhVolChanMid, nan)
		cur.Set(core.SlotWozduhVolChanUp, nan)
		cur.Set(core.SlotWozduhVolChanDn, nan)
	}
	if n.mask&WozduhBitVolCrossPair == 0 {
		cur.Set(core.SlotWozduhVolCross, nan)
	}
	if n.mask&WozduhBitRedRSI == 0 {
		cur.Set(core.SlotWozduhRsiHl2, nan)
	}
	if n.mask&WozduhBitBlackMACD == 0 {
		cur.Set(core.SlotWozduhMacdRsi, nan)
	}
	if n.mask&WozduhBitNavyRSI == 0 {
		cur.Set(core.SlotWozduhRsiHl2Vol, nan)
	}
	if n.mask&WozduhBitADRSI == 0 {
		cur.Set(core.SlotWozduhRsiAd, nan)
	}
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
