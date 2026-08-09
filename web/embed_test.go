package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServesIndex(t *testing.T) {
	h := Handler()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if body == "" {
		t.Error("index body empty")
	}
}

func TestSPAFallback(t *testing.T) {
	h := Handler()
	req := httptest.NewRequest("GET", "/some/spa/route", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200 (fallback)", rec.Code)
	}
}
