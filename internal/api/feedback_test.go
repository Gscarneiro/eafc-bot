package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFeedbackLocalEAppendOnly(t *testing.T) {
	srv, _ := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", bytes.NewBufferString(`{"action_id":"mercado:comprar:1:x","status":"aceita","reason":"teste"}`))
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/feedback", nil))
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("calibracao_disponivel")) {
		t.Fatalf("resposta=%d %s", w.Code, w.Body.String())
	}
}
