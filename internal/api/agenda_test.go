package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgendaRespondeComFaixas(t *testing.T) {
	srv, _ := newTestServer(t)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/agenda", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d %s", w.Code, w.Body.String())
	}
	if w.Body.Len() == 0 {
		t.Fatal("agenda vazia")
	}
}
