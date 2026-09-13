package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleStructuralStop1_MissingSnapshot(t *testing.T) {
	t.Parallel()
	d := &DashboardServer{}
	rec := httptest.NewRecorder()
	d.handleStructuralStop1(rec, httptest.NewRequest(http.MethodGet, "/api/research/structural-stop-1", nil))
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}
