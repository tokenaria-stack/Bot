package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleStarStop_RejectsWriteAndBadIndex(t *testing.T) {
	d := &DashboardServer{}
	rec := httptest.NewRecorder()
	d.handleStarStop(rec, httptest.NewRequest(http.MethodPost, "/api/research/star-stop", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	d.handleStarStop(rec, httptest.NewRequest(http.MethodGet, "/api/research/star-stop?index=-1", nil))
	if rec.Code == http.StatusOK {
		t.Fatal("negative index returned a row")
	}
}

func TestHandleStarStop_AtUsesTheSameIndexAsTheNumberSelector(t *testing.T) {
	d := &DashboardServer{}
	rec := httptest.NewRecorder()
	d.handleStarStop(rec, httptest.NewRequest(http.MethodGet, "/api/research/star-stop?at=1655156700000", nil))
	if rec.Code == http.StatusNotFound && rec.Body.Len() > 0 && rec.Body.String() == "star-stop artifact missing\n" {
		t.Skip("certified star-stop artifact is not on disk")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("at %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Index int `json:"index"`
		Row   struct {
			DecisionAt int64  `json:"decisionAt"`
			Side       string `json:"side"`
			Status     string `json:"status"`
		} `json:"row"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Index != 3455 || got.Row.DecisionAt != 1655156700000 || got.Row.Side != "down" || got.Row.Status != "valid" {
		t.Fatalf("%+v", got)
	}
	rec = httptest.NewRecorder()
	d.handleStarStop(rec, httptest.NewRequest(http.MethodGet, "/api/research/star-stop?at=1", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing star %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	d.handleStarStop(rec, httptest.NewRequest(http.MethodGet, "/api/research/star-stop?at=nope", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad time %d", rec.Code)
	}
}
