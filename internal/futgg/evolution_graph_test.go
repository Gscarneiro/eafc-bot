package futgg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// TestEvolutionGraphEquivaleAEvolutionPathsParaMesmoPayload prova que o
// grafo não muda nada do comportamento hoje: para o mesmo payload real
// (evoPathsFixture, já usado por TestEvolutionPathsFiltraPelaCartaCerta),
// EvolutionGraph().LinearPaths() carrega os mesmos custo/prazo/expiração/
// cadeia que EvolutionPaths() devolve direto — só a carta-raiz muda, de
// propósito: aqui vem de "current" (com GG Rating real), não de
// path.Steps[0] (sempre sem nota).
func TestEvolutionGraphEquivaleAEvolutionPathsParaMesmoPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "playstyles") {
			w.Write([]byte(`{"data":[]}`))
			return
		}
		w.Write([]byte(evoPathsFixture))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Cycle: "26", Endpoints: map[string]string{
		"evolution_paths": "/api/fut/evolutions/v2/26/paths/v2/{id}/",
		"playstyles":      "/api/fut/playstyles/",
	}})

	current := domain.Player{ID: 50537761, BasePlayerEaID: 206113, Cycle: "26", GGRating: 84.0}

	direct, err := c.EvolutionPaths(context.Background(), current.BasePlayerEaID, current.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(direct) != 1 {
		t.Fatalf("esperava 1 caminho direto, achou %d", len(direct))
	}

	graph, err := c.EvolutionGraph(context.Background(), current)
	if err != nil {
		t.Fatal(err)
	}
	got := graph.LinearPaths()
	if len(got) != 1 {
		t.Fatalf("esperava 1 caminho no grafo, achou %d", len(got))
	}

	want := direct[0]
	if got[0].CoinsCost != want.CoinsCost {
		t.Errorf("CoinsCost = %d, esperava %d", got[0].CoinsCost, want.CoinsCost)
	}
	if got[0].PointsCost != want.PointsCost {
		t.Errorf("PointsCost = %d, esperava %d", got[0].PointsCost, want.PointsCost)
	}
	if got[0].IsExpired != want.IsExpired {
		t.Errorf("IsExpired = %v, esperava %v", got[0].IsExpired, want.IsExpired)
	}
	if got[0].TrainingTime != want.TrainingTime {
		t.Errorf("TrainingTime = %q, esperava %q", got[0].TrainingTime, want.TrainingTime)
	}
	if gotFinal, wantFinal := got[0].Final(), want.Final(); gotFinal.Rating != wantFinal.Rating || gotFinal.GGRating != wantFinal.GGRating {
		t.Errorf("carta final = %+v, esperava %+v", gotFinal, wantFinal)
	}
	// A raiz do grafo é "current" (GG Rating real), nunca path.Steps[0]
	// (sempre sem nota) — a diferença deliberada em relação ao caminho direto.
	if gotRoot := got[0].Initial(); gotRoot.GGRating != current.GGRating {
		t.Errorf("raiz do grafo devia usar o GG Rating de current (%v), veio %v", current.GGRating, gotRoot.GGRating)
	}
}

func TestEvolutionGraphPropagaErroDeEvolutionPaths(t *testing.T) {
	// Sem "evolution_paths" configurado, Client.URL falha antes de
	// qualquer rede — EvolutionGraph precisa propagar esse erro, não
	// devolver um grafo vazio silencioso. BaseURL precisa vir preenchido
	// (mesmo que fictício): New() cai para DefaultConfig() — que TEM
	// "evolution_paths" configurado, contra o fut.gg de verdade — sempre
	// que BaseURL vem vazio, o que mascararia este teste como uma chamada
	// de rede de verdade em vez do erro determinístico que ele quer provar.
	c := New(Config{BaseURL: "http://127.0.0.1:0", Cycle: "26"})
	current := domain.Player{ID: 50537761, BasePlayerEaID: 206113, Cycle: "26"}

	g, err := c.EvolutionGraph(context.Background(), current)
	if err == nil {
		t.Fatal("esperava erro propagado de EvolutionPaths, veio nil")
	}
	if len(g.Nodes) != 0 || len(g.Transitions) != 0 {
		t.Errorf("erro deveria devolver grafo zerado, veio %+v", g)
	}
}
