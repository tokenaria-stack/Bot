package ui_config

import (
	"encoding/json"

	"trading_bot/core"
)

const (
	scaleBoundedOsc = `{"type":"bounded","min":-5,"max":105}`
	scaleIgnore     = `{"type":"ignore"}`
)

// WozduhComponents returns DDR bindings for the Wozduh numeric atom set.
// Visibility toggles are driven by Configurable + SettingsRenderer (no FE line hardcode).
// ADR-022: woz_vol_rsi_ema5 is the bounded Auto anchor; peers declare ignore (no heuristics).
//
// Mount order = historical Pine plot() back→front (docs/PINE_INDICATOR_SOURCES.md RSIVolume_2graf.02).
// Later LWC series paint on top. Projector-only plots are not mounted.
func WozduhComponents() []core.UIComponent {
	return []core.UIComponent{
		// No matching displayed Pine plot — keep behind the principal pair.
		wozLine("woz_rsi_ad", core.SlotWozduhRsiAd, scaleIgnore,
			`{"color":"maroon","lineWidth":1,"title":"RSI AD","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_rsi_hl2", core.SlotWozduhRsiHl2, scaleIgnore,
			`{"color":"purple","lineWidth":2,"title":"RSI HL2","defaultVisible":true,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_vol_rsi_ema5", core.SlotWozduhVolRsiEma5, scaleBoundedOsc,
			`{"color":"aqua","lineWidth":2,"title":"Volume RSI EMA5","defaultVisible":true,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_vol_rsi_ema12", core.SlotWozduhVolRsiEma12, scaleIgnore,
			`{"color":"blue","lineWidth":2,"title":"Volume RSI EMA12","defaultVisible":true,"lastValueVisible":false,"priceLineVisible":false}`),
		wozPlot("woz_vol_rsi_ema5_chan_mid", core.SlotWozduhVolRsiEma5ChanMid),
		wozPlot("woz_vol_rsi_ema5_chan_up", core.SlotWozduhVolRsiEma5ChanUp),
		wozPlot("woz_vol_rsi_ema5_chan_dn", core.SlotWozduhVolRsiEma5ChanDn),
		wozChannel("woz_vol_rsi_ema5_chan",
			`{"title":"Volume RSI EMA5 channel","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false,"plots":{"upper":"woz_vol_rsi_ema5_chan_up","mid":"woz_vol_rsi_ema5_chan_mid","lower":"woz_vol_rsi_ema5_chan_dn"},"boundColor":"blue","midColor":"orange","fillColor":"rgba(0,136,255,0.12)","lineWidth":1,"midLineWidth":1,"upperLineStyle":0,"lowerLineStyle":0}`),
		wozLine("woz_rsi_hl2_vwema", core.SlotWozduhRsiHl2Vwema, scaleIgnore,
			`{"color":"navy","lineWidth":1,"title":"RSI VWEMA(HL2)","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_rsi_close", core.SlotWozduhRsiClose, scaleIgnore,
			`{"color":"#f23645","lineWidth":2,"title":"RSI close","defaultVisible":true,"lastValueVisible":false,"priceLineVisible":false}`),
		wozPlot("woz_rsi_close_chan_mid", core.SlotWozduhRsiCloseChanMid),
		wozPlot("woz_rsi_close_chan_up", core.SlotWozduhRsiCloseChanUp),
		wozPlot("woz_rsi_close_chan_dn", core.SlotWozduhRsiCloseChanDn),
		wozChannel("woz_rsi_close_chan",
			`{"title":"RSI close channel","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false,"plots":{"upper":"woz_rsi_close_chan_up","mid":"woz_rsi_close_chan_mid","lower":"woz_rsi_close_chan_dn"},"boundColor":"blue","midColor":"maroon","upperFillColor":"rgba(128,0,0,0.12)","lowerFillColor":"rgba(128,0,0,0.12)","lineWidth":1,"midLineWidth":1,"upperLineStyle":0,"lowerLineStyle":0}`),
		wozLine("woz_rsi_rsi_close", core.SlotWozduhRsiRsiClose, scaleIgnore,
			`{"color":"orange","lineWidth":2,"title":"RSI of RSI(close)","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_macd_rsi_close", core.SlotWozduhMacdRsiClose, scaleIgnore,
			`{"color":"black","lineWidth":2,"title":"MACD RSI(close)+50","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
		wozLine("woz_rsi_close_ema7", core.SlotWozduhRsiCloseEma7, scaleIgnore,
			`{"color":"green","lineWidth":2,"title":"RSI close EMA7","defaultVisible":false,"lastValueVisible":false,"priceLineVisible":false}`),
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
