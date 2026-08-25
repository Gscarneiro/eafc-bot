package futgg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Collect precisa terminar com um mapa de Capabilities que separa o que deu
// certo (clube, confirmado) do que falhou (mercado, erro) — é o que
// /api/saude expõe sem exigir que quem lê vasculhe a lista plana de Errors
// atrás do nome certo.
func TestCollectPopulaCapabilitiesComErroPorFonte(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "gg-club"):
			w.Write([]byte(clubPlayerReal))
		case strings.Contains(r.URL.Path, "players"):
			// 404 devolve ErrNotFound sem retry (backoff só entra em 429/5xx) —
			// é o jeito rápido de simular uma fonte que falhou neste teste.
			w.WriteHeader(http.StatusNotFound)
		default:
			w.Write([]byte(`{"data":[]}`))
		}
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL: srv.URL,
		Cycle:   "26",
		Endpoints: map[string]string{
			"club":       "/api/gg-club/{gamertag}/players/",
			"players":    "/api/fut/players/",
			"evolutions": "/api/fut/evolutions/",
			"objectives": "/api/fut/objectives/",
			"sbcs":       "/api/fut/sbc/sets/",
			"news":       "/api/fut/news/",
		},
	})

	snap, err := c.Collect(context.Background(), "BilingualBee", PlayerFilter{Pages: 1})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	clube := snap.Capabilities["clube"]
	if clube.Status != StatusConfirmado || clube.Coverage != 1 {
		t.Fatalf("clube = %+v, esperava confirmado com cobertura 1", clube)
	}
	mercado := snap.Capabilities["mercado"]
	if mercado.Status != StatusErro || mercado.Error == "" {
		t.Fatalf("mercado = %+v, esperava erro não vazio", mercado)
	}
	for _, key := range []string{"evoluções", "objetivos", "SBCs", "notícias"} {
		if got := snap.Capabilities[key]; got.Status != StatusConfirmado {
			t.Errorf("%s = %+v, esperava confirmado (lista vazia é resposta válida, não falha)", key, got)
		}
	}
}
