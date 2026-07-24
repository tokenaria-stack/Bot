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

func TestWozduhFastBoundedPeersIgnore(t *testing.T) {
	comps := WozduhComponents()
	var fastType string
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
		if c.ID == "woz_fast" {
			fastType = typ
			if sc["min"].(float64) != -5 || sc["max"].(float64) != 105 {
				t.Fatalf("woz_fast bounds=%v", sc)
			}
			continue
		}
		if typ != "ignore" {
			t.Fatalf("%s want ignore, got %v", c.ID, typ)
		}
		ignoreCount++
	}
	if fastType != "bounded" {
		t.Fatalf("woz_fast type=%q", fastType)
	}
	if ignoreCount < 10 {
		t.Fatalf("expected many ignore peers, got %d", ignoreCount)
	}
}
