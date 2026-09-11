package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// TestResumoContaAvisosTrocasViaveisEEvolucoesNoXI trava três leituras que a
// topbar e o rail dependem: TrocasViaveis só conta upgrade que cabe no bolso
// E tem cotação, cada erro de coleta vira um Aviso, e o selo "Análise" bate
// com o mesmo resumo que a tela de Análise usa — sem recalcular elegibilidade
// de evolução aqui (isso já tem teste próprio em evolution_paths_test.go).
func TestResumoContaAvisosTrocasViaveisEEvolucoesNoXI(t *testing.T) {
	snap := fixtureSnapshot()
	snap.Errors = []string{"fut.gg: timeout na coleta de mercado"}
	snap.Upgrades = []analyze.Upgrade{
		{Slot: domain.CM, Candidate: domain.Player{ID: 3, Name: "Viável"}, Affordable: true, Unpriced: false},
		{Slot: domain.CB, Candidate: domain.Player{ID: 4, Name: "Sem cotação"}, Affordable: true, Unpriced: true},
		{Slot: domain.CB, Candidate: domain.Player{ID: 5, Name: "Fora do bolso"}, Affordable: false, Unpriced: false},
	}

	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/resumo", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d %s", w.Code, w.Body.String())
	}
	got := decodeJSON[ResumoResponse](t, w)

	if got.TrocasViaveis != 1 {
		t.Errorf("TrocasViaveis = %d, esperava 1 (só o upgrade afordável e cotado)", got.TrocasViaveis)
	}

	foundErro := false
	for _, aviso := range got.Avisos {
		if aviso.Kind == "coleta" && aviso.Detail == snap.Errors[0] {
			foundErro = true
		}
	}
	if !foundErro {
		t.Errorf("avisos = %+v, esperava um aviso de coleta com o erro do snapshot", got.Avisos)
	}

	wantEntraNoXI := evolutionPathsSummary(buildEvolutionPlayerAnalyses(snap)).EntraNoXI
	if got.AnaliseEntraNoXI != wantEntraNoXI {
		t.Errorf("AnaliseEntraNoXI = %d, esperava %d (mesmo resumo da tela de Análise)", got.AnaliseEntraNoXI, wantEntraNoXI)
	}
}

func TestResumoUsaSaldoManualNoTopoENoCapital(t *testing.T) {
	snap := fixtureSnapshot()
	snap.Club.Coins = 0
	snap.Capital = domain.Capital{}

	saldo := 123_456
	srv, _ := newTestServerWithSnapshot(t, snap)
	srv.Config = &ConfigEditor{GetCoins: func() *int { return &saldo }}

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/resumo", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d %s", w.Code, w.Body.String())
	}
	got := decodeJSON[ResumoResponse](t, w)
	if got.Coins != saldo || got.Capital.Cash != saldo {
		t.Fatalf("saldo manual não chegou ao resumo: coins=%d capital=%+v", got.Coins, got.Capital)
	}
}
