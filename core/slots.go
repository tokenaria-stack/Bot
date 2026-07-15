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
	SlotWozduhFast
	SlotWozduhSlow

	SlotDivScore
	SlotDivState
	SlotMicroDivScore
	SlotTotalScore

	// Chaos atoms (Layer 2) — DDR debt: slots reserved; DAG node wiring TBD.
	SlotAO
	SlotAD
	SlotStoch
	SlotOrangeRSI

	// Wozduh Pine atoms (Great Purge Stage 2) — writers land in Stage 3 WozduhNode.
	SlotWozduhRsiPrice
	SlotWozduhEmaRsi
	SlotWozduhRsiRsi
	SlotWozduhRsiHl2
	SlotWozduhMacdRsi
	SlotWozduhRsiAd
	SlotWozduhRsiHl2Vol
	SlotWozduhVolChanMid
	SlotWozduhVolChanUp
	SlotWozduhVolChanDn
	SlotWozduhPriceChanMid
	SlotWozduhPriceChanUp
	SlotWozduhPriceChanDn
	SlotWozduhVolCross

	// SlotCount is the number of defined slots (valid indices: 0 .. SlotCount-1).
	SlotCount
)
