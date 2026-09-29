package starlens

import "testing"

func TestResearchCatalog191(t *testing.T) {
	cat := ResearchCatalog()
	if len(cat) != Schema3CoordCount+MatrixCoordCount {
		t.Fatalf("catalog %d", len(cat))
	}
	seen := map[string]struct{}{}
	var vwema, rel bool
	for _, e := range cat {
		if e.ID == "" || e.Label == "" {
			t.Fatalf("empty %+v", e)
		}
		if _, ok := seen[e.ID]; ok {
			t.Fatalf("dup %s", e.ID)
		}
		seen[e.ID] = struct{}{}
		if e.ID == "M15.Vwema" {
			vwema = true
			if e.Group != "Schema 3" || e.Section != "15m" {
				t.Fatalf("vwema meta %+v", e)
			}
		}
		if e.ID == "H1RSX.Value" && e.Section != "RSX" {
			t.Fatalf("h1 rsx %+v", e)
		}
		if e.ID == "M15H1Vwema" {
			rel = true
			if e.Group != "Matrix" {
				t.Fatalf("matrix meta %+v", e)
			}
		}
	}
	if !vwema || !rel {
		t.Fatal("required ids missing")
	}
	if _, ok := seen["M15H4Vwema"]; ok {
		t.Fatal("M15H4")
	}
}
