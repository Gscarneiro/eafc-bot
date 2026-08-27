package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPareamentoLANProtegeLeituraEEscrita(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.PairingToken = "par"
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status sem token=%d", w.Code)
	}
	w = httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.Header.Set("X-EAFC-Pairing-Token", "par")
	srv.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status com token=%d", w.Code)
	}
}
