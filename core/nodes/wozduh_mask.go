package nodes

// WozduhMask is a fixed compute-requirement bitmask for WozduhNode.
// Zero means no Wozduh streams run. WozduhMaskAll is the explicit compute-all default.
type WozduhMask uint32

const (
	WozduhBitOrangeBase WozduhMask = 1 << iota
	WozduhBitGreenEMA
	WozduhBitRsiOfRsi
	WozduhBitPriceChannel
	WozduhBitVolBase
	WozduhBitWt11
	WozduhBitWt22
	WozduhBitVolChannel
	WozduhBitVolCrossPair
	WozduhBitRedRSI
	WozduhBitBlackMACD
	WozduhBitNavyRSI
	WozduhBitADRSI
)

// WozduhMaskAll enables every Wozduh compute branch (default ReplayClosedBars / live Frame).
const WozduhMaskAll = WozduhBitOrangeBase |
	WozduhBitGreenEMA |
	WozduhBitRsiOfRsi |
	WozduhBitPriceChannel |
	WozduhBitVolBase |
	WozduhBitWt11 |
	WozduhBitWt22 |
	WozduhBitVolChannel |
	WozduhBitVolCrossPair |
	WozduhBitRedRSI |
	WozduhBitBlackMACD |
	WozduhBitNavyRSI |
	WozduhBitADRSI

// Plot → required compute bits. Compose/render IDs are not keys.
var wozduhPlotBits = map[string]WozduhMask{
	"woz_rsi_price":      WozduhBitOrangeBase,
	"woz_ema_rsi":        WozduhBitOrangeBase | WozduhBitGreenEMA,
	"woz_rsi_rsi":        WozduhBitOrangeBase | WozduhBitRsiOfRsi,
	"woz_price_chan_up":  WozduhBitOrangeBase | WozduhBitPriceChannel,
	"woz_price_chan_mid": WozduhBitOrangeBase | WozduhBitPriceChannel,
	"woz_price_chan_dn":  WozduhBitOrangeBase | WozduhBitPriceChannel,
	"woz_fast":           WozduhBitVolBase | WozduhBitWt11,
	"woz_slow":           WozduhBitVolBase | WozduhBitWt22,
	"woz_vol_chan_up":    WozduhBitVolBase | WozduhBitWt22 | WozduhBitVolChannel,
	"woz_vol_chan_mid":   WozduhBitVolBase | WozduhBitWt22 | WozduhBitVolChannel,
	"woz_vol_chan_dn":    WozduhBitVolBase | WozduhBitWt22 | WozduhBitVolChannel,
	"woz_vol_cross":      WozduhBitVolBase | WozduhBitWt11 | WozduhBitWt22 | WozduhBitVolCrossPair,
	"woz_rsi_hl2":        WozduhBitRedRSI,
	"woz_macd_rsi":       WozduhBitBlackMACD,
	"woz_rsi_hl2_vol":    WozduhBitNavyRSI,
	"woz_rsi_ad":         WozduhBitADRSI,
}

// WozduhMaskForPlots unions compute bits for requested scalar plot IDs.
// Unknown and non-Wozduh IDs contribute nothing.
func WozduhMaskForPlots(ids []string) WozduhMask {
	var m WozduhMask
	for _, id := range ids {
		if bits, ok := wozduhPlotBits[id]; ok {
			m |= bits
		}
	}
	return m
}

// WozduhDefaultVisibleMask is the compute closure of the current default-visible lines:
// woz_rsi_hl2, woz_slow, woz_fast, woz_rsi_price.
func WozduhDefaultVisibleMask() WozduhMask {
	return WozduhMaskForPlots([]string{"woz_rsi_hl2", "woz_slow", "woz_fast", "woz_rsi_price"})
}

// WozduhWakeReplayMask is the temp-node mask for a 0→1 transition: waking bits plus
// the prerequisites those bits need during a closed-bar replay. Live install still
// copies only wake bits, not already-active shared bases.
func WozduhWakeReplayMask(wake WozduhMask) WozduhMask {
	m := wake
	if wake&(WozduhBitGreenEMA|WozduhBitRsiOfRsi|WozduhBitPriceChannel) != 0 {
		m |= WozduhBitOrangeBase
	}
	if wake&(WozduhBitWt11|WozduhBitWt22|WozduhBitVolChannel|WozduhBitVolCrossPair) != 0 {
		m |= WozduhBitVolBase
	}
	if wake&WozduhBitVolChannel != 0 {
		m |= WozduhBitWt22
	}
	if wake&WozduhBitVolCrossPair != 0 {
		m |= WozduhBitWt11 | WozduhBitWt22
	}
	return m
}

// WozduhMaskFromClientSubscriptions ORs plot-ID lists from WS clients on one Frame.
// A nil or empty list is the WIRE-1 unfiltered contract (compute-all).
// No clients → 0.
func WozduhMaskFromClientSubscriptions(slotLists [][]string) WozduhMask {
	if len(slotLists) == 0 {
		return 0
	}
	var u WozduhMask
	for _, ids := range slotLists {
		if len(ids) == 0 {
			return WozduhMaskAll
		}
		u |= WozduhMaskForPlots(ids)
	}
	return u
}
