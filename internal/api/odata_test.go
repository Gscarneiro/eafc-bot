package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestColecoesRespondemEnvelopeOData(t *testing.T) {
	srv, _ := newTestServer(t)
	rotas := []string{
		"/api/mercado",
		"/api/evolucoes",
		"/api/elenco/titulares",
		"/api/elenco/reservas",
		"/api/capital/investimentos",
		"/api/capital/vendas",
		"/api/capital/sbcs",
		"/api/hoje/novidades",
		"/api/hoje/noticias",
		"/api/hoje/sbcs",
		"/api/hoje/objetivos",
		"/api/hoje/movimentacao",
		"/api/historico",
	}
	for _, rota := range rotas {
		t.Run(rota, func(t *testing.T) {
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, rota+"?$top=1&$count=true", nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if _, ok := body["value"]; !ok {
				t.Fatalf("rota sem value: %s", w.Body.String())
			}
			if _, ok := body["@odata.count"]; !ok {
				t.Fatalf("rota sem @odata.count: %s", w.Body.String())
			}
		})
	}
}

func TestColecaoRejeitaCampoDesconhecidoComCamposValidos(t *testing.T) {
	srv, _ := newTestServer(t)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/mercado?$filter=campo_inexistente%20eq%201", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if len(w.Body.String()) == 0 || !containsAny(w.Body.String(), "campos válidos", "valid_fields") {
		t.Fatalf("erro não ensinou a próxima consulta: %s", w.Body.String())
	}
}

func containsAny(value string, options ...string) bool {
	for _, option := range options {
		if len(value) >= len(option) {
			for i := 0; i+len(option) <= len(value); i++ {
				if value[i:i+len(option)] == option {
					return true
				}
			}
		}
	}
	return false
}
