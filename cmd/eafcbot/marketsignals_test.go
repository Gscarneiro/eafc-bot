package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// refreshMarketSignals grava momentum e custo de SBC de uma coleta só —
// é o job que o ciclo rápido chama em vez do runJob completo. A
// correção de SaveSBCCost em si (chave certa, valor certo) já tem teste
// direto em internal/store; aqui o que importa é confirmar a FIAÇÃO:
// refreshMarketSignals de fato chama as duas rotas e repassa pro Store.
func TestRefreshMarketSignalsGravaMomentumECustoDeSBC(t *testing.T) {
	sbcHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/fut/players/v2/momentum/"):
			w.Write([]byte(`{"data":[{"id":1,"overall":90,"position":"ST","momentumPercentage":18.5}],
				"next":null,"currentPage":1,"total":1}`))
		case r.URL.Path == "/api/fut/sbc/":
			sbcHits++
			w.Write([]byte(`{"data":[{"slug":"weekend-sbc","name":"Weekend SBC",
				"challenges":[{"name":"83-Rated Squad","cheapestSolutionPrice":20000}]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.FutGG.BaseURL = srv.URL
	cfg.FutGG.Endpoints = map[string]string{
		"momentum": "/api/fut/players/v2/momentum/{hours}/",
		"sbcs":     "/api/fut/sbc/",
	}
	cfg.Serve.MomentumWindowHours = 24
	cfg.FutGG.RequestsPerSec = 200 // servidor local de teste, sem motivo pra ser educado

	st, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	defer st.Close()

	refreshMarketSignals(context.Background(), cfg, st)

	momentum, err := st.LatestMomentum(context.Background(), cfg.FutGG.Cycle)
	if err != nil {
		t.Fatalf("LatestMomentum: %v", err)
	}
	if len(momentum) != 1 || momentum[0].MomentumPct != 18.5 {
		t.Fatalf("esperava 1 carta com momentum 18.5, veio %+v", momentum)
	}
	if sbcHits != 1 {
		t.Fatalf("esperava a rota de SBC ser batida 1 vez, veio %d", sbcHits)
	}
}

// Momentum falhando (404 simulado — futgg.Client.GetRaw não faz retry
// nesse status, o que mantém o teste rápido; 5xx teria o mesmo efeito só
// que com backoff de verdade) não pode impedir o custo de SBC de ser
// gravado — mesmo princípio de tolerância a falha parcial que
// futgg.Collect já segue pras fontes da coleta completa. Prova por
// contagem de chamada: se refreshMarketSignals abortasse no primeiro
// erro, a rota de SBC nunca seria batida.
func TestRefreshMarketSignalsMomentumFalhandoNaoImpedeSBC(t *testing.T) {
	sbcHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/fut/players/v2/momentum/"):
			http.NotFound(w, r)
		case r.URL.Path == "/api/fut/sbc/":
			sbcHits++
			w.Write([]byte(`{"data":[{"slug":"weekend-sbc","name":"Weekend SBC",
				"challenges":[{"name":"83-Rated Squad","cheapestSolutionPrice":20000}]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.FutGG.BaseURL = srv.URL
	cfg.FutGG.Endpoints = map[string]string{
		"momentum": "/api/fut/players/v2/momentum/{hours}/",
		"sbcs":     "/api/fut/sbc/",
	}
	cfg.FutGG.RequestsPerSec = 200 // servidor local de teste, sem motivo pra ser educado

	st, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	defer st.Close()

	refreshMarketSignals(context.Background(), cfg, st)

	if sbcHits != 1 {
		t.Fatalf("esperava a rota de SBC ser batida mesmo com momentum falhando, veio %d chamadas", sbcHits)
	}
	momentum, _ := st.LatestMomentum(context.Background(), cfg.FutGG.Cycle)
	if len(momentum) != 0 {
		t.Fatalf("momentum devia continuar vazio (a busca falhou), veio %+v", momentum)
	}
}
