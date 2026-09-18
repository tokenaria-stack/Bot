package nodes

// WozduhMask is a fixed compute-requirement bitmask for WozduhNode.
// Zero means no Wozduh streams run. WozduhMaskAll is the explicit compute-all default.
type WozduhMask uint32

const (
	WozduhBitRsiClose WozduhMask = 1 << iota
	WozduhBitRsiCloseEma7
	WozduhBitRsiOfRsi
	WozduhBitRsiCloseChan
	// WozduhBitVolBase is a DAG compute-mask bit: volume-RSI / Wozduh volume
	// streams must run. It is NOT Binance kline field "V" (taker-buy base).
	WozduhBitVolBase
	WozduhBitVolRsiEma12
	WozduhBitVolRsiEma5
	WozduhBitVolRsiEma5Chan
	WozduhBitRsiHl2
	WozduhBitMacdRsiClose
	WozduhBitRsiHl2Vwema
	WozduhBitRsiAd
)

// WozduhMaskAll enables every Wozduh compute branch (default ReplayClosedBars / live Frame).
const WozduhMaskAll = WozduhBitRsiClose |
	WozduhBitRsiCloseEma7 |
	WozduhBitRsiOfRsi |
	WozduhBitRsiCloseChan |
	WozduhBitVolBase |
	WozduhBitVolRsiEma12 |
	WozduhBitVolRsiEma5 |
	WozduhBitVolRsiEma5Chan |
	WozduhBitRsiHl2 |
	WozduhBitMacdRsiClose |
	WozduhBitRsiHl2Vwema |
	WozduhBitRsiAd

// Plot → required compute bits. Compose/render IDs are not keys.
var wozduhPlotBits = map[string]WozduhMask{
	"woz_rsi_close":             WozduhBitRsiClose,
	"woz_rsi_close_ema7":        WozduhBitRsiClose | WozduhBitRsiCloseEma7,
	"woz_rsi_rsi_close":         WozduhBitRsiClose | WozduhBitRsiOfRsi,
	"woz_rsi_close_chan_up":     WozduhBitRsiClose | WozduhBitRsiCloseChan,
	"woz_rsi_close_chan_mid":    WozduhBitRsiClose | WozduhBitRsiCloseChan,
	"woz_rsi_close_chan_dn":     WozduhBitRsiClose | WozduhBitRsiCloseChan,
	"woz_vol_rsi_ema12":         WozduhBitVolBase | WozduhBitVolRsiEma12,
	"woz_vol_rsi_ema5":          WozduhBitVolBase | WozduhBitVolRsiEma5,
	"woz_vol_rsi_ema5_chan_up":  WozduhBitVolBase | WozduhBitVolRsiEma5 | WozduhBitVolRsiEma5Chan,
	"woz_vol_rsi_ema5_chan_mid": WozduhBitVolBase | WozduhBitVolRsiEma5 | WozduhBitVolRsiEma5Chan,
	"woz_vol_rsi_ema5_chan_dn":  WozduhBitVolBase | WozduhBitVolRsiEma5 | WozduhBitVolRsiEma5Chan,
	"woz_rsi_hl2":               WozduhBitRsiHl2,
	"woz_macd_rsi_close":        WozduhBitMacdRsiClose,
	"woz_rsi_hl2_vwema":         WozduhBitRsiHl2Vwema,
	"woz_rsi_ad":                WozduhBitRsiAd,
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
// woz_rsi_hl2, woz_vol_rsi_ema5, woz_vol_rsi_ema12, woz_rsi_close.
func WozduhDefaultVisibleMask() WozduhMask {
	return WozduhMaskForPlots([]string{"woz_rsi_hl2", "woz_vol_rsi_ema5", "woz_vol_rsi_ema12", "woz_rsi_close"})
}

// WozduhWakeReplayMask is the temp-node mask for a 0→1 transition: waking bits plus
// the prerequisites those bits need during a closed-bar replay. Live install still
// copies only wake bits, not already-active shared bases.
func WozduhWakeReplayMask(wake WozduhMask) WozduhMask {
	m := wake
	if wake&(WozduhBitRsiCloseEma7|WozduhBitRsiOfRsi|WozduhBitRsiCloseChan) != 0 {
		m |= WozduhBitRsiClose
	}
	if wake&(WozduhBitVolRsiEma12|WozduhBitVolRsiEma5|WozduhBitVolRsiEma5Chan) != 0 {
		m |= WozduhBitVolBase
	}
	if wake&WozduhBitVolRsiEma5Chan != 0 {
		m |= WozduhBitVolRsiEma5
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
