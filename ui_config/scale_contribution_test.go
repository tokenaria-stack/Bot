package ui_config

import (
	"encoding/json"
	"testing"
)

func TestMergeScaleContribution(t *testing.T) {
	raw := mergeScaleContribution(
		`{"color":"blue","lineWidth":2,"title":"Volume RSI EMA12"}`,
		`{"type":"bounded","min":-5,"max":105}`,
	)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["title"] != "Volume RSI EMA12" {
		t.Fatalf("title lost: %v", m["title"])
	}
	sc, ok := m["scaleContribution"].(map[string]any)
	if !ok {
		t.Fatalf("scaleContribution missing: %#v", m)
	}
	if sc["type"] != "bounded" {
		t.Fatalf("type=%v", sc["type"])
	}
	if sc["min"].(float64) != -5 || sc["max"].(float64) != 105 {
		t.Fatalf("bounds=%v %v", sc["min"], sc["max"])
	}
}

func TestRSXComponentsScaleContribution(t *testing.T) {
	comps := RSXComponents()
	var primary, signal map[string]any
	for _, c := range comps {
		var m map[string]any
		if err := json.Unmarshal(c.RenderOpts, &m); err != nil {
			t.Fatal(c.ID, err)
		}
		switch c.ID {
		case "line_rsx":
			primary = m
		case "line_rsx_signal":
			signal = m
		}
	}
	if primary == nil || signal == nil {
		t.Fatal("missing rsx components")
	}
	p := primary["scaleContribution"].(map[string]any)
	s := signal["scaleContribution"].(map[string]any)
	if p["type"] != "bounded" || p["min"].(float64) != -5 || p["max"].(float64) != 105 {
		t.Fatalf("primary=%v", p)
	}
	if s["type"] != "ignore" {
		t.Fatalf("signal=%v", s)
	}
	if primary["color"] != "#512DA8" {
		t.Fatalf("line_rsx default color=%v", primary["color"])
	}
	if signal["color"] != "#8B9BB4" {
		t.Fatalf("line_rsx_signal color=%v", signal["color"])
	}
	if primary["lastValueVisible"] != false || primary["priceLineVisible"] != false {
		t.Fatalf("line_rsx last-value chrome=%v %v", primary["lastValueVisible"], primary["priceLineVisible"])
	}
	if signal["lastValueVisible"] != false || signal["priceLineVisible"] != false {
		t.Fatalf("line_rsx_signal last-value chrome=%v %v", signal["lastValueVisible"], signal["priceLineVisible"])
	}
}

func TestWozduhVolRsiEma5BoundedPeersIgnore(t *testing.T) {
	comps := WozduhComponents()
	var ownerType string
	boundedCount := 0
	ignoreCount := 0
	for _, c := range comps {
		var m map[string]any
		if err := json.Unmarshal(c.RenderOpts, &m); err != nil {
			t.Fatal(c.ID, err)
		}
		sc, ok := m["scaleContribution"].(map[string]any)
		if !ok {
			t.Fatalf("%s missing scaleContribution", c.ID)
		}
		typ, _ := sc["type"].(string)
		if c.ID == "woz_vol_rsi_ema5" {
			ownerType = typ
			boundedCount++
			if sc["min"].(float64) != -5 || sc["max"].(float64) != 105 {
				t.Fatalf("woz_vol_rsi_ema5 bounds=%v", sc)
			}
			continue
		}
		if typ != "ignore" {
			t.Fatalf("%s want ignore, got %v", c.ID, typ)
		}
		ignoreCount++
	}
	if ownerType != "bounded" {
		t.Fatalf("woz_vol_rsi_ema5 type=%q", ownerType)
	}
	if boundedCount != 1 {
		t.Fatalf("expected exactly one bounded Wozduh owner, got %d", boundedCount)
	}
	if ignoreCount < 10 {
		t.Fatalf("expected many ignore peers, got %d", ignoreCount)
	}
}

func TestWozduhChannelPaintComponents(t *testing.T) {
	comps := WozduhComponents()
	kind := map[string]string{}
	mode := map[string]string{}
	for _, c := range comps {
		kind[c.ID] = c.Kind
		mode[c.ID] = c.DataMode
	}
	for _, id := range []string{
		"woz_vol_rsi_ema5_chan_mid", "woz_vol_rsi_ema5_chan_up", "woz_vol_rsi_ema5_chan_dn",
		"woz_rsi_close_chan_mid", "woz_rsi_close_chan_up", "woz_rsi_close_chan_dn",
	} {
		if kind[id] != "plot" || mode[id] != "scalar" {
			t.Fatalf("%s kind=%s mode=%s", id, kind[id], mode[id])
		}
	}
	for _, id := range []string{"woz_vol_rsi_ema5_chan", "woz_rsi_close_chan"} {
		if kind[id] != "channel" || mode[id] != "compose" {
			t.Fatalf("%s kind=%s mode=%s", id, kind[id], mode[id])
		}
		if kind[id] == "line" {
			t.Fatalf("%s must not be a LineSeries component", id)
		}
	}
}

func TestWozduhPineRenderMountOrder(t *testing.T) {
	comps := WozduhComponents()
	var mounted []string
	for _, c := range comps {
		if c.Kind == "plot" {
			continue
		}
		mounted = append(mounted, c.ID)
	}
	want := []string{
		"woz_rsi_ad",
		"woz_rsi_hl2",
		"woz_vol_rsi_ema5",
		"woz_vol_rsi_ema12",
		"woz_vol_rsi_ema5_chan",
		"woz_rsi_hl2_vwema",
		"woz_rsi_close",
		"woz_rsi_close_chan",
		"woz_rsi_rsi_close",
		"woz_macd_rsi_close",
		"woz_rsi_close_ema7",
	}
	if len(mounted) != len(want) {
		t.Fatalf("mounted=%v want=%v", mounted, want)
	}
	for i := range want {
		if mounted[i] != want[i] {
			t.Fatalf("mount[%d]=%s want %s (full=%v)", i, mounted[i], want[i], mounted)
		}
	}
}

func TestWozduhMenuTitlesAndSolidChannelBounds(t *testing.T) {
	comps := WozduhComponents()
	titles := map[string]string{}
	for _, c := range comps {
		var m map[string]any
		if err := json.Unmarshal(c.RenderOpts, &m); err != nil {
			t.Fatal(c.ID, err)
		}
		if title, ok := m["title"].(string); ok {
			titles[c.ID] = title
		}
		if c.ID == "woz_vol_rsi_ema5_chan" || c.ID == "woz_rsi_close_chan" {
			if m["upperLineStyle"] != float64(0) || m["lowerLineStyle"] != float64(0) {
				t.Fatalf("%s line styles=%v %v", c.ID, m["upperLineStyle"], m["lowerLineStyle"])
			}
		}
	}
	if titles["woz_vol_rsi_ema12"] != "Volume RSI EMA12" {
		t.Fatalf("woz_vol_rsi_ema12 title=%q", titles["woz_vol_rsi_ema12"])
	}
	if titles["woz_vol_rsi_ema5"] != "Volume RSI EMA5" {
		t.Fatalf("woz_vol_rsi_ema5 title=%q", titles["woz_vol_rsi_ema5"])
	}
	if titles["woz_rsi_close"] != "RSI close" {
		t.Fatalf("woz_rsi_close title=%q", titles["woz_rsi_close"])
	}
	if titles["woz_rsi_close_ema7"] != "RSI close EMA7" {
		t.Fatalf("woz_rsi_close_ema7 title=%q", titles["woz_rsi_close_ema7"])
	}
	if titles["woz_rsi_rsi_close"] != "RSI of RSI(close)" {
		t.Fatalf("woz_rsi_rsi_close title=%q", titles["woz_rsi_rsi_close"])
	}
	if titles["woz_rsi_close_chan"] != "RSI close channel" {
		t.Fatalf("woz_rsi_close_chan title=%q", titles["woz_rsi_close_chan"])
	}
	if titles["woz_macd_rsi_close"] != "MACD RSI(close)+50" {
		t.Fatalf("woz_macd_rsi_close title=%q", titles["woz_macd_rsi_close"])
	}
	if titles["woz_vol_rsi_ema5_chan"] != "Volume RSI EMA5 channel" {
		t.Fatalf("woz_vol_rsi_ema5_chan title=%q", titles["woz_vol_rsi_ema5_chan"])
	}
	if titles["woz_rsi_hl2_vwema"] != "RSI VWEMA(HL2)" {
		t.Fatalf("woz_rsi_hl2_vwema title=%q", titles["woz_rsi_hl2_vwema"])
	}
	if titles["woz_rsi_hl2"] != "RSI HL2" {
		t.Fatalf("woz_rsi_hl2 title=%q", titles["woz_rsi_hl2"])
	}
	if titles["woz_rsi_ad"] != "RSI AD" {
		t.Fatalf("woz_rsi_ad title=%q", titles["woz_rsi_ad"])
	}
	if titles["woz_vol_rsi_ema12"] == titles["woz_vol_rsi_ema5"] {
		t.Fatal("titles must stay distinct")
	}
}

func TestWozduhChannelFillCapabilities(t *testing.T) {
	comps := WozduhComponents()
	byID := map[string]map[string]any{}
	for _, c := range comps {
		var m map[string]any
		if err := json.Unmarshal(c.RenderOpts, &m); err != nil {
			t.Fatal(c.ID, err)
		}
		byID[c.ID] = m
		_, whole := m["fillColor"]
		_, upSplit := m["upperFillColor"]
		_, dnSplit := m["lowerFillColor"]
		if whole && (upSplit || dnSplit) {
			t.Fatalf("%s advertises fillColor together with split fills", c.ID)
		}
		_, bound := m["boundColor"]
		_, up := m["upperColor"]
		_, dn := m["lowerColor"]
		if bound && (up || dn) {
			t.Fatalf("%s advertises boundColor together with edge colors", c.ID)
		}
	}
	rsi := byID["woz_rsi_close_chan"]
	if rsi["boundColor"] != "blue" {
		t.Fatalf("rsi boundColor=%v", rsi["boundColor"])
	}
	if rsi["upperFillColor"] != "rgba(128,0,0,0.12)" || rsi["lowerFillColor"] != "rgba(128,0,0,0.12)" {
		t.Fatalf("rsi split fills=%v %v", rsi["upperFillColor"], rsi["lowerFillColor"])
	}
	for _, k := range []string{"fillColor", "upperColor", "lowerColor", "upperOuterFillColor", "lowerOuterFillColor"} {
		if _, ok := rsi[k]; ok {
			t.Fatalf("woz_rsi_close_chan must omit %s", k)
		}
	}
	vol := byID["woz_vol_rsi_ema5_chan"]
	if vol["boundColor"] != "blue" {
		t.Fatalf("vol boundColor=%v", vol["boundColor"])
	}
	if vol["fillColor"] != "rgba(0,136,255,0.12)" {
		t.Fatalf("vol fillColor=%v", vol["fillColor"])
	}
	if vol["midColor"] != "orange" {
		t.Fatalf("vol midColor=%v", vol["midColor"])
	}
	for _, k := range []string{"upperColor", "lowerColor", "upperFillColor", "lowerFillColor", "upperOuterFillColor", "lowerOuterFillColor"} {
		if _, ok := vol[k]; ok {
			t.Fatalf("volume channel must omit %s", k)
		}
	}
}
