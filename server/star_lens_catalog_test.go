package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trading_bot/forecast/starlens"
)

func TestStarLensCatalogAndSelectedField(t *testing.T) {
	before3, beforeM, beforeO := frozenFileHashes(t)
	store, err := loadLens()
	if err != nil {
		t.Skip(err)
	}
	d := &DashboardServer{}
	req := httptest.NewRequest(http.MethodGet, "/api/research/star-lens/catalog", nil)
	rec := httptest.NewRecorder()
	d.handleStarLensCatalog(rec, req)
	if rec.Code != 200 {
		t.Fatalf("catalog %d %s", rec.Code, rec.Body.String())
	}
	var cat struct {
		Count       int
		Coordinates []starlens.CatalogEntry `json:"coordinates"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cat); err != nil {
		t.Fatal(err)
	}
	if cat.Count != 191 || len(cat.Coordinates) != 191 {
		t.Fatalf("count %d %d", cat.Count, len(cat.Coordinates))
	}
	seen := map[string]struct{}{}
	for _, e := range cat.Coordinates {
		if _, ok := seen[e.ID]; ok {
			t.Fatalf("dup %s", e.ID)
		}
		seen[e.ID] = struct{}{}
	}
	if _, ok := seen["M15.Vwema"]; !ok {
		t.Fatal("M15.Vwema")
	}
	if _, ok := seen["M15H1Vwema"]; !ok {
		t.Fatal("M15H1Vwema")
	}

	uni := store.sources[lensSourceAll]
	evalSelected := func(clauses []starlens.Clause, field string) (lensPass int, pic *lensNumberWire) {
		t.Helper()
		body, _ := json.Marshal(lensEvalRequest{
			Source: "all", Clauses: clauses, SelectedField: field,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/research/star-lens/evaluate", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		d.handleStarLensEvaluate(rec, req)
		if rec.Code != 200 {
			t.Fatalf("eval %d %s", rec.Code, rec.Body.String())
		}
		var got struct {
			LensPass int `json:"lensPass"`
			Numbers  []lensNumberWire
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Numbers) < 5 {
			t.Fatalf("numbers %d", len(got.Numbers))
		}
		if got.Numbers[0].Field != starlens.FieldMFEATR {
			t.Fatalf("outcome picture lost %s", got.Numbers[0].Field)
		}
		var sel *lensNumberWire
		for i := range got.Numbers {
			if got.Numbers[i].Field == field {
				sel = &got.Numbers[i]
			}
		}
		if sel == nil || sel.Missing == starlens.UniverseStars || !sel.RangeOK {
			t.Fatalf("selected %+v", sel)
		}
		return got.LensPass, sel
	}

	pass1, pic1 := evalSelected(nil, "M15.Vwema")
	if pass1 != starlens.UniverseStars {
		t.Fatalf("empty lens %d", pass1)
	}

	cut := pic1.Min + (pic1.Max-pic1.Min)*0.8
	passCut, picCut := evalSelected([]starlens.Clause{
		starlens.Continuous("M15.Vwema", starlens.CmpGTE, cut),
	}, "M15.Vwema")
	if passCut >= pass1 || passCut == 0 {
		t.Fatalf("threshold did not change lens %d -> %d", pass1, passCut)
	}
	if picCut.Min > cut {
		t.Fatalf("self-exclusion collapsed picture min %v bound %v", picCut.Min, cut)
	}

	mixed3, _ := evalSelected([]starlens.Clause{
		starlens.REvent(1, "reached"),
		starlens.Continuous("M15.Vwema", starlens.CmpGTE, pic1.Min),
	}, "M15.Vwema")
	if mixed3 == 0 || mixed3 > 4160 {
		t.Fatalf("1R+vwema %d", mixed3)
	}

	_, picRel := evalSelected(nil, "M15H1Vwema")
	if picRel.Missing == starlens.UniverseStars {
		t.Fatal("matrix all missing")
	}
	mixedM, _ := evalSelected([]starlens.Clause{
		starlens.REvent(1, "reached"),
		starlens.Continuous("M15H1Vwema", starlens.CmpGTE, picRel.Min),
	}, "M15H1Vwema")
	if mixedM == 0 || mixedM > 4160 {
		t.Fatalf("1R+matrix %d", mixedM)
	}

	dir := t.TempDir()
	child, err := starlens.SaveChild(uni, store.rows, []starlens.Clause{
		starlens.Continuous("M15.Vwema", starlens.CmpGTE, cut),
	}, filepath.Join(dir, "s3.json"), time.Unix(4, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if child.Clauses[0].Field != "M15.Vwema" {
		t.Fatalf("saved schema3 %q", child.Clauses[0].Field)
	}
	mx, err := starlens.SaveChild(uni, store.rows, []starlens.Clause{
		starlens.REvent(1, "reached"),
		starlens.Continuous("M15H1Vwema", starlens.CmpGTE, picRel.Min),
	}, filepath.Join(dir, "mx.json"), time.Unix(4, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if mx.Clauses[1].Field != "M15H1Vwema" {
		t.Fatalf("saved matrix %+v", mx.Clauses)
	}
	if mx.Provenance.MatrixDigest != starlens.MatrixDigest {
		t.Fatalf("matrix provenance %+v", mx.Provenance)
	}
	bare := uni
	bare.Provenance.MatrixDigest = ""
	bare.Provenance.MatrixCommit = ""
	if _, err := starlens.SaveChild(bare, store.rows, []starlens.Clause{
		starlens.Continuous("M15H1Vwema", starlens.CmpGTE, 0),
	}, filepath.Join(dir, "bad.json"), time.Unix(4, 0).UTC()); err == nil {
		t.Fatal("matrix provenance optional")
	}

	after3, afterM, afterO := frozenFileHashes(t)
	if after3 != before3 || afterM != beforeM || afterO != beforeO {
		t.Fatal("frozen files changed")
	}
	popDir := filepath.Join(filepath.Dir(mustFindOutcome(t)), "populations")
	if _, err := os.Stat(popDir); err == nil {
		entries, _ := os.ReadDir(popDir)
		for _, e := range entries {
			if e.Name() == child.ID+".json" {
				t.Fatal("wrote live populations dir")
			}
		}
	}
}

func mustFindOutcome(t *testing.T) string {
	t.Helper()
	p, err := starlens.FindOutcomeFile()
	if err != nil {
		t.Fatal(err)
	}
	return p
}
