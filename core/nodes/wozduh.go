package nodes

import (
	"math"

	"trading_bot/core"
	"trading_bot/indicators"
)

// Wozduh periods — SSOT for WozduhNode (historical Pine RSIVol_2graf.02 values, unchanged).
const (
	wozduhChannelPeriod     = 24
	wozduhChannelPhi        = 1.6185
	wozduhLenVol            = 24 // RSI length on volume-weighted close / HL2 VWEMA
	wozduhVolRsiEma12Period = 12 // EMA of RSI24(VWEMA24(close, volume))
	wozduhVolRsiEma5Period  = 5  // EMA of the same RSI24(VWEMA24(close, volume))
	wozduhRsiCloseEma7      = 7  // EMA(RSI14(close))
	wozduhRsiPeriod         = 14
	wozduhMacdFast          = 7
	wozduhMacdSlow          = wozduhLenVol
	wozduhMacdSignal        = 9
)

// WozduhNode computes the Wozduh numeric atom set into the data bus.
// Jurik RSX lives in RSXNode — not duplicated here.
type WozduhNode struct {
	bus  *core.Bus
	mask WozduhMask

	streamUpdates int
	wakeInstalls  int

	rsiHl2         *indicators.RSI // RSI14(HL2)
	rsiClose       *indicators.RSI // RSI14(close)
	rsiCloseEma7   *indicators.EMA // EMA7(RSI14(close))
	rsiOfRsi       *indicators.RSI // RSI14(RSI14(close))
	macdRsiClose   *indicators.RSI // RSI24(close) feed for MACD
	macdOnRsiClose *indicators.MACD

	volVwema    *indicators.VolumeWeightedEMA
	volRsi      *indicators.RSI
	volRsiEma12 *indicators.EMA
	volRsiEma5  *indicators.EMA

	rsiHl2Vwema    *indicators.VolumeWeightedEMA
	rsiHl2VwemaRsi *indicators.RSI

	volRsiEma5ChanSMA   *indicators.SMA
	volRsiEma5ChanStDev *indicators.RollingStDev
	rsiCloseChanSMA     *indicators.SMA
	rsiCloseChanStDev   *indicators.RollingStDev

	ad    *indicators.AD
	adRsi *indicators.RSI

	// Closed-bar A×B prev for presentation crossovers. Snapshot/Restore with streams.
	crossPrevA     [4]float64
	crossPrevB     [4]float64
	crossReady     [4]bool
	crossSnapA     [4]float64
	crossSnapB     [4]float64
	crossSnapReady [4]bool
	lastCrossHits  []WozduhCrossoverHit
}

// NewWozduhNode creates a full Wozduh atom pipeline (explicit compute-all mask).
func NewWozduhNode() *WozduhNode {
	return NewWozduhNodeMasked(WozduhMaskAll)
}

// NewWozduhNodeMasked creates a Wozduh pipeline that runs only the given compute bits.
// Zero means no streams run. ApplyMask may change the mask later (live Frame demand).
func NewWozduhNodeMasked(mask WozduhMask) *WozduhNode {
	return &WozduhNode{
		mask:                mask,
		rsiHl2:              indicators.NewRSI(wozduhRsiPeriod),
		rsiClose:            indicators.NewRSI(wozduhRsiPeriod),
		rsiCloseEma7:        indicators.NewEMA(wozduhRsiCloseEma7),
		rsiOfRsi:            indicators.NewRSI(wozduhRsiPeriod),
		macdRsiClose:        indicators.NewRSI(wozduhLenVol),
		macdOnRsiClose:      indicators.NewMACD(wozduhMacdFast, wozduhMacdSlow, wozduhMacdSignal),
		volVwema:            indicators.NewVolumeWeightedEMA(wozduhLenVol),
		volRsi:              indicators.NewRSI(wozduhLenVol),
		volRsiEma12:         indicators.NewEMA(wozduhVolRsiEma12Period),
		volRsiEma5:          indicators.NewEMA(wozduhVolRsiEma5Period),
		rsiHl2Vwema:         indicators.NewVolumeWeightedEMA(wozduhLenVol),
		rsiHl2VwemaRsi:      indicators.NewRSI(wozduhLenVol),
		volRsiEma5ChanSMA:   indicators.NewSMA(wozduhChannelPeriod),
		volRsiEma5ChanStDev: indicators.NewRollingStDev(wozduhChannelPeriod),
		rsiCloseChanSMA:     indicators.NewSMA(wozduhChannelPeriod),
		rsiCloseChanStDev:   indicators.NewRollingStDev(wozduhChannelPeriod),
		ad:                  indicators.NewAD(),
		adRsi:               indicators.NewRSI(wozduhRsiPeriod),
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

	if n.mask&WozduhBitRsiClose != 0 {
		rsiClose := n.rsiClose.Update(close)
		n.noteStream()
		cur.Set(core.SlotWozduhRsiClose, rsiClose)
		if n.mask&WozduhBitRsiCloseEma7 != 0 {
			cur.Set(core.SlotWozduhRsiCloseEma7, n.rsiCloseEma7.Update(rsiClose))
			n.noteStream()
		}
		if n.mask&WozduhBitRsiOfRsi != 0 {
			cur.Set(core.SlotWozduhRsiRsiClose, n.rsiOfRsi.Update(rsiClose))
			n.noteStream()
		}
		if n.mask&WozduhBitRsiCloseChan != 0 {
			rsiCloseChanMid := n.rsiCloseChanSMA.Update(rsiClose)
			n.noteStream()
			rsiCloseChanOffs := wozduhChannelPhi * n.rsiCloseChanStDev.Update(rsiClose)
			n.noteStream()
			cur.Set(core.SlotWozduhRsiCloseChanMid, rsiCloseChanMid)
			cur.Set(core.SlotWozduhRsiCloseChanUp, rsiCloseChanMid+rsiCloseChanOffs)
			cur.Set(core.SlotWozduhRsiCloseChanDn, rsiCloseChanMid-rsiCloseChanOffs)
		}
	}

	if n.mask&WozduhBitRsiHl2 != 0 {
		cur.Set(core.SlotWozduhRsiHl2, n.rsiHl2.Update(hl2))
		n.noteStream()
	}

	if n.mask&WozduhBitMacdRsiClose != 0 {
		rsiForMacd := n.macdRsiClose.Update(close)
		n.noteStream()
		cur.Set(core.SlotWozduhMacdRsiClose, n.macdOnRsiClose.Update(rsiForMacd)+50.0)
		n.noteStream()
	}

	if n.mask&WozduhBitVolBase != 0 {
		volPrice := n.volVwema.Update(close, volume)
		n.noteStream()
		volRsi := n.volRsi.Update(volPrice)
		n.noteStream()
		var volRsiEma12, volRsiEma5 float64
		if n.mask&WozduhBitVolRsiEma12 != 0 {
			volRsiEma12 = n.volRsiEma12.Update(volRsi)
			n.noteStream()
			cur.Set(core.SlotWozduhVolRsiEma12, volRsiEma12)
		}
		if n.mask&WozduhBitVolRsiEma5 != 0 {
			volRsiEma5 = n.volRsiEma5.Update(volRsi)
			n.noteStream()
			cur.Set(core.SlotWozduhVolRsiEma5, volRsiEma5)
			if n.mask&WozduhBitVolRsiEma5Chan != 0 {
				volChanMid := n.volRsiEma5ChanSMA.Update(volRsiEma5)
				n.noteStream()
				volOffs := wozduhChannelPhi * n.volRsiEma5ChanStDev.Update(volRsiEma5)
				n.noteStream()
				cur.Set(core.SlotWozduhVolRsiEma5ChanMid, volChanMid)
				cur.Set(core.SlotWozduhVolRsiEma5ChanUp, volChanMid+volOffs)
				cur.Set(core.SlotWozduhVolRsiEma5ChanDn, volChanMid-volOffs)
			}
		}
	}

	if n.mask&WozduhBitRsiHl2Vwema != 0 {
		vwemaHl2 := n.rsiHl2Vwema.Update(hl2, volume)
		n.noteStream()
		cur.Set(core.SlotWozduhRsiHl2Vwema, n.rsiHl2VwemaRsi.Update(vwemaHl2))
		n.noteStream()
	}

	if n.mask&WozduhBitRsiAd != 0 {
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

func (n *WozduhNode) Mask() WozduhMask {
	if n == nil {
		return 0
	}
	return n.mask
}

func (n *WozduhNode) Slot(slot core.Slot) float64 {
	if n == nil || n.bus == nil || n.bus.Cur == nil {
		return math.NaN()
	}
	return n.bus.Cur.Get(slot)
}

func (n *WozduhNode) failClosedInactive(cur *core.TickFrame) {
	nan := math.NaN()
	if n.mask&WozduhBitRsiClose == 0 {
		cur.Set(core.SlotWozduhRsiClose, nan)
	}
	if n.mask&WozduhBitRsiCloseEma7 == 0 {
		cur.Set(core.SlotWozduhRsiCloseEma7, nan)
	}
	if n.mask&WozduhBitRsiOfRsi == 0 {
		cur.Set(core.SlotWozduhRsiRsiClose, nan)
	}
	if n.mask&WozduhBitRsiCloseChan == 0 {
		cur.Set(core.SlotWozduhRsiCloseChanMid, nan)
		cur.Set(core.SlotWozduhRsiCloseChanUp, nan)
		cur.Set(core.SlotWozduhRsiCloseChanDn, nan)
	}
	if n.mask&WozduhBitVolRsiEma12 == 0 {
		cur.Set(core.SlotWozduhVolRsiEma12, nan)
	}
	if n.mask&WozduhBitVolRsiEma5 == 0 {
		cur.Set(core.SlotWozduhVolRsiEma5, nan)
	}
	if n.mask&WozduhBitVolRsiEma5Chan == 0 {
		cur.Set(core.SlotWozduhVolRsiEma5ChanMid, nan)
		cur.Set(core.SlotWozduhVolRsiEma5ChanUp, nan)
		cur.Set(core.SlotWozduhVolRsiEma5ChanDn, nan)
	}
	if n.mask&WozduhBitRsiHl2 == 0 {
		cur.Set(core.SlotWozduhRsiHl2, nan)
	}
	if n.mask&WozduhBitMacdRsiClose == 0 {
		cur.Set(core.SlotWozduhMacdRsiClose, nan)
	}
	if n.mask&WozduhBitRsiHl2Vwema == 0 {
		cur.Set(core.SlotWozduhRsiHl2Vwema, nan)
	}
	if n.mask&WozduhBitRsiAd == 0 {
		cur.Set(core.SlotWozduhRsiAd, nan)
	}
}

// ApplyMask installs a new compute mask and NaNs newly inactive output slots immediately.
func (n *WozduhNode) ApplyMask(mask WozduhMask) {
	if n == nil {
		return
	}
	n.mask = mask
	if n.bus != nil && n.bus.Cur != nil {
		n.failClosedInactive(n.bus.Cur)
	}
}

// InstallWokenFields copies only newly activated stateful fields from a temp replay node.
func (n *WozduhNode) InstallWokenFields(src *WozduhNode, wake WozduhMask) {
	if n == nil || src == nil || wake == 0 {
		return
	}
	if wake&WozduhBitRsiClose != 0 {
		n.rsiClose = src.rsiClose
	}
	if wake&WozduhBitRsiCloseEma7 != 0 {
		n.rsiCloseEma7 = src.rsiCloseEma7
	}
	if wake&WozduhBitRsiOfRsi != 0 {
		n.rsiOfRsi = src.rsiOfRsi
	}
	if wake&WozduhBitRsiCloseChan != 0 {
		n.rsiCloseChanSMA = src.rsiCloseChanSMA
		n.rsiCloseChanStDev = src.rsiCloseChanStDev
	}
	if wake&WozduhBitVolBase != 0 {
		n.volVwema = src.volVwema
		n.volRsi = src.volRsi
	}
	if wake&WozduhBitVolRsiEma12 != 0 {
		n.volRsiEma12 = src.volRsiEma12
	}
	if wake&WozduhBitVolRsiEma5 != 0 {
		n.volRsiEma5 = src.volRsiEma5
	}
	if wake&WozduhBitVolRsiEma5Chan != 0 {
		n.volRsiEma5ChanSMA = src.volRsiEma5ChanSMA
		n.volRsiEma5ChanStDev = src.volRsiEma5ChanStDev
	}
	if wake&WozduhBitRsiHl2 != 0 {
		n.rsiHl2 = src.rsiHl2
	}
	if wake&WozduhBitMacdRsiClose != 0 {
		n.macdRsiClose = src.macdRsiClose
		n.macdOnRsiClose = src.macdOnRsiClose
	}
	if wake&WozduhBitRsiHl2Vwema != 0 {
		n.rsiHl2Vwema = src.rsiHl2Vwema
		n.rsiHl2VwemaRsi = src.rsiHl2VwemaRsi
	}
	if wake&WozduhBitRsiAd != 0 {
		n.ad = src.ad
		n.adRsi = src.adRsi
	}
	n.wakeInstalls++
}

// WakeInstalls is the number of InstallWokenFields calls (tests).
func (n *WozduhNode) WakeInstalls() int {
	if n == nil {
		return 0
	}
	return n.wakeInstalls
}

// RsiClosePtr exposes the RSI(close) object for shared-base identity tests.
func (n *WozduhNode) RsiClosePtr() *indicators.RSI {
	if n == nil {
		return nil
	}
	return n.rsiClose
}

// VolRsiEma12Ptr exposes volume-RSI EMA12 for shared-base identity tests.
func (n *WozduhNode) VolRsiEma12Ptr() *indicators.EMA {
	if n == nil {
		return nil
	}
	return n.volRsiEma12
}

func (n *WozduhNode) SaveState() {
	if n == nil {
		return
	}
	n.rsiHl2.SaveState()
	n.rsiClose.SaveState()
	n.rsiCloseEma7.SaveState()
	n.rsiOfRsi.SaveState()
	n.macdRsiClose.SaveState()
	n.macdOnRsiClose.SaveState()
	n.volVwema.SaveState()
	n.volRsi.SaveState()
	n.volRsiEma12.SaveState()
	n.volRsiEma5.SaveState()
	n.rsiHl2Vwema.SaveState()
	n.rsiHl2VwemaRsi.SaveState()
	n.volRsiEma5ChanSMA.SaveState()
	n.volRsiEma5ChanStDev.SaveState()
	n.rsiCloseChanSMA.SaveState()
	n.rsiCloseChanStDev.SaveState()
	n.ad.SaveState()
	n.adRsi.SaveState()
	n.lastCrossHits = nil
	if n.bus != nil && n.bus.Cur != nil {
		n.lastCrossHits = detectWozduhCrossoversFromFrame(n.bus.Cur, &n.crossPrevA, &n.crossPrevB, &n.crossReady)
	}
	n.crossSnapA = n.crossPrevA
	n.crossSnapB = n.crossPrevB
	n.crossSnapReady = n.crossReady
}

func (n *WozduhNode) RestoreState() {
	if n == nil {
		return
	}
	n.rsiHl2.RestoreState()
	n.rsiClose.RestoreState()
	n.rsiCloseEma7.RestoreState()
	n.rsiOfRsi.RestoreState()
	n.macdRsiClose.RestoreState()
	n.macdOnRsiClose.RestoreState()
	n.volVwema.RestoreState()
	n.volRsi.RestoreState()
	n.volRsiEma12.RestoreState()
	n.volRsiEma5.RestoreState()
	n.rsiHl2Vwema.RestoreState()
	n.rsiHl2VwemaRsi.RestoreState()
	n.volRsiEma5ChanSMA.RestoreState()
	n.volRsiEma5ChanStDev.RestoreState()
	n.rsiCloseChanSMA.RestoreState()
	n.rsiCloseChanStDev.RestoreState()
	n.ad.RestoreState()
	n.adRsi.RestoreState()
	n.crossPrevA = n.crossSnapA
	n.crossPrevB = n.crossSnapB
	n.crossReady = n.crossSnapReady
	n.lastCrossHits = nil
}

// LastClosedCrossovers is the closed bar just committed by SaveState. Empty on forming ticks.
func (n *WozduhNode) LastClosedCrossovers() []WozduhCrossoverHit {
	if n == nil || len(n.lastCrossHits) == 0 {
		return nil
	}
	out := make([]WozduhCrossoverHit, len(n.lastCrossHits))
	copy(out, n.lastCrossHits)
	return out
}

func (n *WozduhNode) OnConfigChange(any) error { return nil }

// VolRsiEma12Value exposes the volume-RSI EMA12 state (shadow validation / tests).
func (n *WozduhNode) VolRsiEma12Value() float64 {
	if n.volRsiEma12 == nil {
		return 0
	}
	return n.volRsiEma12.Value()
}
