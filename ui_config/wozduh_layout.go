package ui_config

import (
	"encoding/json"

	"trading_bot/core"
)

const (
	scaleBoundedOsc = `{"type":"bounded","min":-5,"max":105}`
	scaleIgnore     = `{"type":"ignore"}`
)

// WozduhComponents returns DDR bindings for the full Wozduh Pine atom set.
// Visibility toggles are driven by Configurable + SettingsRenderer (no FE line hardcode).
// ADR-022: woz_fast is the bounded Auto anchor; peers declare ignore (no heuristics).
func WozduhComponents() []core.UIComponent {
	return []core.UIComponent{
		wozLine("woz_rsi_price", core.SlotWozduhRsiPrice, scaleIgnore,
			`{"color":"#f23645","lineWidth":2,"title":"RSI Price (Red)","defaultVisible":true,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_ema_rsi", core.SlotWozduhEmaRsi, scaleIgnore,
			`{"color":"green","lineWidth":2,"title":"EMA RSI (Green)","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_rsi_rsi", core.SlotWozduhRsiRsi, scaleIgnore,
			`{"color":"orange","lineWidth":2,"title":"RSI of RSI (Orange)","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_rsi_hl2", core.SlotWozduhRsiHl2, scaleIgnore,
			`{"color":"purple","lineWidth":2,"title":"RSI HL2 (Purple)","defaultVisible":true,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_macd_rsi", core.SlotWozduhMacdRsi, scaleIgnore,
			`{"color":"black","lineWidth":2,"title":"MACD RSI (Black)","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_fast", core.SlotWozduhFast, scaleBoundedOsc,
			`{"color":"blue","lineWidth":2,"title":"wt11 (Blue)","defaultVisible":true,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_slow", core.SlotWozduhSlow, scaleIgnore,
			`{"color":"aqua","lineWidth":2,"title":"wt22 (Aqua)","defaultVisible":true,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_rsi_ad", core.SlotWozduhRsiAd, scaleIgnore,
			`{"color":"maroon","lineWidth":1,"title":"RSI AD (Maroon)","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_rsi_hl2_vol", core.SlotWozduhRsiHl2Vol, scaleIgnore,
			`{"color":"navy","lineWidth":1,"title":"RSI HL2×Vol (Navy)","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozPlot("woz_vol_chan_mid", core.SlotWozduhVolChanMid),
		wozPlot("woz_vol_chan_up", core.SlotWozduhVolChanUp),
		wozPlot("woz_vol_chan_dn", core.SlotWozduhVolChanDn),
		wozPlot("woz_price_chan_mid", core.SlotWozduhPriceChanMid),
		wozPlot("woz_price_chan_up", core.SlotWozduhPriceChanUp),
		wozPlot("woz_price_chan_dn", core.SlotWozduhPriceChanDn),
		wozChannel("woz_vol_chan",
			`{"title":"Vol Chan","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false,"plots":{"upper":"woz_vol_chan_up","mid":"woz_vol_chan_mid","lower":"woz_vol_chan_dn"},"upperColor":"blue","midColor":"orange","lowerColor":"blue","fillColor":"rgba(0,136,255,0.12)","lineWidth":1,"midLineWidth":1,"upperLineStyle":2,"lowerLineStyle":2}`),
		wozChannel("woz_price_chan",
			`{"title":"Price Chan","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false,"plots":{"upper":"woz_price_chan_up","mid":"woz_price_chan_mid","lower":"woz_price_chan_dn"},"upperColor":"blue","midColor":"maroon","lowerColor":"blue","fillColor":"rgba(128,0,0,0.12)","lineWidth":1,"midLineWidth":1,"upperLineStyle":2,"lowerLineStyle":2}`),
		// VolCross is a marker/code atom — not mounted as an LWC line (Stage 4+ annotations).
	}
}

// wozPlot is a projector-only scalar column (wire/store id). DDR does not mount it as a LineSeries.
func wozPlot(id string, slot core.Slot) core.UIComponent {
	opts := mergeScaleContribution(
		`{"defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`,
		scaleIgnore,
	)
	return core.UIComponent{
		ID:           id,
		Pane:         "pane_osc",
		HostID:       "wozduh",
		Kind:         "plot",
		DataMode:     "scalar",
		Slot:         slot,
		Configurable: false,
		RenderOpts:   opts,
	}
}

func wozChannel(id, renderOpts string) core.UIComponent {
	opts := mergeScaleContribution(renderOpts, scaleIgnore)
	return core.UIComponent{
		ID:           id,
		Pane:         "pane_osc",
		HostID:       "wozduh",
		Kind:         "channel",
		DataMode:     "compose",
		Configurable: true,
		RenderOpts:   opts,
	}
}

func wozLine(id string, slot core.Slot, scaleContributionJSON, renderOpts string) core.UIComponent {
	opts := mergeScaleContribution(renderOpts, scaleContributionJSON)
	return core.UIComponent{
		ID:           id,
		Pane:         "pane_osc",
		HostID:       "wozduh",
		Kind:         "line",
		DataMode:     "scalar",
		Slot:         slot,
		Configurable: true,
		RenderOpts:   opts,
	}
}

// mergeScaleContribution injects renderOptions.scaleContribution without hostId heuristics.
func mergeScaleContribution(renderOptsJSON, scaleContributionJSON string) json.RawMessage {
	var base map[string]any
	if err := json.Unmarshal([]byte(renderOptsJSON), &base); err != nil || base == nil {
		base = map[string]any{}
	}
	var contrib any
	if err := json.Unmarshal([]byte(scaleContributionJSON), &contrib); err == nil {
		base["scaleContribution"] = contrib
	}
	out, err := json.Marshal(base)
	if err != nil {
		return json.RawMessage(renderOptsJSON)
	}
	return out
}
