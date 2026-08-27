package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestImportacaoLocalRecusaSegredoEPreservaVazio(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, body := range []string{`{"club":{"players":[]}}`, `{"token":"segredo"}`} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/import/club", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		srv.Handler().ServeHTTP(w, req)
		if w.Code < 400 {
			t.Fatalf("aceitou %s", body)
		}
	}
}
