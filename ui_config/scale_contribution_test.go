package ui_config

import (
	"encoding/json"
	"testing"
)

func TestMergeScaleContribution(t *testing.T) {
	raw := mergeScaleContribution(
		`{"color":"blue","lineWidth":2,"title":"wt11 (Blue)"}`,
		`{"type":"bounded","min":-5,"max":105}`,
	)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["title"] != "wt11 (Blue)" {
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
}

func TestWozduhSlowBoundedPeersIgnore(t *testing.T) {
	comps := WozduhComponents()
	var slowType string
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
		if c.ID == "woz_slow" {
			slowType = typ
			boundedCount++
			if sc["min"].(float64) != -5 || sc["max"].(float64) != 105 {
				t.Fatalf("woz_slow bounds=%v", sc)
			}
			continue
		}
		if typ != "ignore" {
			t.Fatalf("%s want ignore, got %v", c.ID, typ)
		}
		ignoreCount++
	}
	if slowType != "bounded" {
		t.Fatalf("woz_slow type=%q", slowType)
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
		"woz_vol_chan_mid", "woz_vol_chan_up", "woz_vol_chan_dn",
		"woz_price_chan_mid", "woz_price_chan_up", "woz_price_chan_dn",
	} {
		if kind[id] != "plot" || mode[id] != "scalar" {
			t.Fatalf("%s kind=%s mode=%s", id, kind[id], mode[id])
		}
	}
	for _, id := range []string{"woz_vol_chan", "woz_price_chan"} {
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
		"woz_slow",
		"woz_fast",
		"woz_vol_chan",
		"woz_rsi_hl2_vol",
		"woz_rsi_price",
		"woz_price_chan",
		"woz_rsi_rsi",
		"woz_macd_rsi",
		"woz_ema_rsi",
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
		if c.ID == "woz_vol_chan" || c.ID == "woz_price_chan" {
			if m["upperLineStyle"] != float64(0) || m["lowerLineStyle"] != float64(0) {
				t.Fatalf("%s line styles=%v %v", c.ID, m["upperLineStyle"], m["lowerLineStyle"])
			}
		}
	}
	if titles["woz_fast"] != "Woz fast (Blue)" {
		t.Fatalf("woz_fast title=%q", titles["woz_fast"])
	}
	if titles["woz_slow"] != "Woz slow (Aqua)" {
		t.Fatalf("woz_slow title=%q", titles["woz_slow"])
	}
	if titles["woz_fast"] == titles["woz_slow"] {
		t.Fatal("titles must stay distinct")
	}
}
