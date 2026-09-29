package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"trading_bot/forecast/starlens"
)

func TestStarLensEvaluateUsesEvaluator(t *testing.T) {
	store, err := loadLens()
	if err != nil {
		t.Skip(err)
	}
	d := &DashboardServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/research/star-lens/sources", d.handleStarLensSources)
	mux.HandleFunc("/api/research/star-lens/evaluate", d.handleStarLensEvaluate)
	req := httptest.NewRequest(http.MethodGet, "/api/research/star-lens/sources", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("sources %d %s", rec.Code, rec.Body.String())
	}
	var src struct {
		Populations []lensSourceInfo `json:"populations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &src); err != nil {
		t.Fatal(err)
	}
	if len(src.Populations) < 8 || src.Populations[0].Count != starlens.UniverseStars {
		t.Fatalf("sources %+v", src.Populations)
	}
	body, _ := json.Marshal(lensEvalRequest{
		Source:  "all",
		Clauses: []starlens.Clause{starlens.REvent(1, "reached")},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/research/star-lens/evaluate", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("eval %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Population int `json:"population"`
		LensPass   int `json:"lensPass"`
		Events     []lensEventWire
		Numbers    []lensNumberWire
		Pass       []starlens.StarRef
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Population != starlens.UniverseStars || got.LensPass != 4160 || len(got.Pass) != 4160 {
		t.Fatalf("pass %d pop %d", got.LensPass, got.Population)
	}
	if len(got.Events) == 0 || got.Events[0].Reached != 4160 || got.Events[0].Undefined != 47 || got.Events[0].Rate == 1 {
		t.Fatalf("events %+v", got.Events)
	}
	var mfe *lensNumberWire
	for i := range got.Numbers {
		if got.Numbers[i].Field == starlens.FieldMFEATR {
			mfe = &got.Numbers[i]
		}
	}
	if mfe == nil || mfe.Missing != 1 {
		t.Fatalf("mfe %+v", mfe)
	}
	_ = store
}

func TestStarLensViewportDoesNotChangePass(t *testing.T) {
	if _, err := loadLens(); err != nil {
		t.Skip(err)
	}
	d := &DashboardServer{}
	body, _ := json.Marshal(lensEvalRequest{
		Source:   "all",
		Clauses:  []starlens.Clause{starlens.REvent(1, "reached")},
		ViewFrom: 1,
		ViewTo:   2,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/research/star-lens/evaluate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	d.handleStarLensEvaluate(rec, req)
	var got struct {
		LensPass  int `json:"lensPass"`
		ChartView int `json:"chartView"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.LensPass != 4160 {
		t.Fatalf("lens %d", got.LensPass)
	}
	if got.ChartView != 0 {
		t.Fatalf("view %d", got.ChartView)
	}
}
