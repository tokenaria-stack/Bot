package core

// Slot is a compile-time index into the tick frame and HistoryBus rings.
// Wire keys and JSON field names live in the UI manifest, not here.
type Slot uint16

const (
	SlotPriceOpen Slot = iota
	SlotPriceHigh
	SlotPriceLow
	SlotPriceClose
	SlotVolume

	SlotJurikRSX
	SlotJurikSignal
	SlotWozduhVolRsiEma12
	SlotWozduhVolRsiEma5

	// Chaos atoms (Layer 2) — DDR debt: slots reserved; DAG node wiring TBD.
	SlotAO
	SlotAD
	SlotStoch
	SlotOrangeRSI

	// Wozduh numeric atoms — writers live in WozduhNode. Iota order is frozen.
	SlotWozduhRsiClose
	SlotWozduhRsiCloseEma7
	SlotWozduhRsiRsiClose
	SlotWozduhRsiHl2
	SlotWozduhMacdRsiClose
	SlotWozduhRsiAd
	SlotWozduhRsiHl2Vwema
	SlotWozduhVolRsiEma5ChanMid
	SlotWozduhVolRsiEma5ChanUp
	SlotWozduhVolRsiEma5ChanDn
	SlotWozduhRsiCloseChanMid
	SlotWozduhRsiCloseChanUp
	SlotWozduhRsiCloseChanDn
	SlotWozduhVolCross

	// SlotCount is the number of defined slots (valid indices: 0 .. SlotCount-1).
	SlotCount
)
