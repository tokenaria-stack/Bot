package ui_config

import (
	"encoding/json"

	"trading_bot/core"
)

// WozduhComponents returns DDR bindings for the full Wozduh Pine atom set.
// Visibility toggles are driven by Configurable + SettingsRenderer (no FE line hardcode).
func WozduhComponents() []core.UIComponent {
	return []core.UIComponent{
		wozLine("woz_rsi_price", core.SlotWozduhRsiPrice,
			`{"color":"#f23645","lineWidth":2,"title":"RSI Price (Red)","defaultVisible":true,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_ema_rsi", core.SlotWozduhEmaRsi,
			`{"color":"green","lineWidth":2,"title":"EMA RSI (Green)","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_rsi_rsi", core.SlotWozduhRsiRsi,
			`{"color":"orange","lineWidth":2,"title":"RSI of RSI (Orange)","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_rsi_hl2", core.SlotWozduhRsiHl2,
			`{"color":"purple","lineWidth":2,"title":"RSI HL2 (Purple)","defaultVisible":true,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_macd_rsi", core.SlotWozduhMacdRsi,
			`{"color":"black","lineWidth":2,"title":"MACD RSI (Black)","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_fast", core.SlotWozduhFast,
			`{"color":"blue","lineWidth":2,"title":"wt11 (Blue)","defaultVisible":true,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_slow", core.SlotWozduhSlow,
			`{"color":"aqua","lineWidth":2,"title":"wt22 (Aqua)","defaultVisible":true,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_rsi_ad", core.SlotWozduhRsiAd,
			`{"color":"maroon","lineWidth":1,"title":"RSI AD (Maroon)","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_rsi_hl2_vol", core.SlotWozduhRsiHl2Vol,
			`{"color":"navy","lineWidth":1,"title":"RSI HL2×Vol (Navy)","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_vol_chan_mid", core.SlotWozduhVolChanMid,
			`{"color":"orange","lineWidth":1,"title":"Vol Chan Mid","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_vol_chan_up", core.SlotWozduhVolChanUp,
			`{"color":"blue","lineWidth":1,"lineStyle":2,"title":"Vol Chan Up","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_vol_chan_dn", core.SlotWozduhVolChanDn,
			`{"color":"blue","lineWidth":1,"lineStyle":2,"title":"Vol Chan Dn","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_price_chan_mid", core.SlotWozduhPriceChanMid,
			`{"color":"maroon","lineWidth":1,"title":"Price Chan Mid","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_price_chan_up", core.SlotWozduhPriceChanUp,
			`{"color":"blue","lineWidth":1,"lineStyle":2,"title":"Price Chan Up","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_price_chan_dn", core.SlotWozduhPriceChanDn,
			`{"color":"blue","lineWidth":1,"lineStyle":2,"title":"Price Chan Dn","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		// VolCross is a marker/code atom — not mounted as an LWC line (Stage 4+ annotations).
	}
}

func wozLine(id string, slot core.Slot, renderOpts string) core.UIComponent {
	return core.UIComponent{
		ID:           id,
		Pane:         "pane_osc",
		HostID:       "wozduh",
		Kind:         "line",
		DataMode:     "scalar",
		Slot:         slot,
		Configurable: true,
		RenderOpts:   json.RawMessage(renderOpts),
	}
}
