package futgg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// evoPathsFixture imita a resposta real de /evolutions/v2/26/paths/v2/{id}/:
// vários caminhos para o MESMO jogador-base, cada um começando de uma carta
// (versão) diferente dele. "id" no passo é o id INTERNO (não o eaId) — é o
// caso que testamos abaixo, porque mapPlayer sozinho pegaria o errado.
const evoPathsFixture = `{"data":[
 {"path":[
   {"id":136512,"eaId":50537761,"overall":85,"commonName":"Gnabry",
    "playstyles":[37],"playstylesPlus":[]},
   {"id":136599,"eaId":50537761,"overall":98,"ggRating":97.5,"commonName":"Gnabry",
    "playstyles":[37,8],"playstylesPlus":[5]}
 ],"coinsCost":10000,"pointsCost":0,"isExpired":false,
  "readableTrainingTime":"3 days","evolutions":[{"name":"Gold Standard"}]},
 {"path":[
   {"id":900001,"eaId":117646625,"overall":92,"commonName":"Gnabry"},
   {"id":900002,"eaId":117646625,"overall":95,"ggRating":94.0,"commonName":"Gnabry"}
 ],"coinsCost":50000,"pointsCost":0,"isExpired":false,
  "readableTrainingTime":"5 days","evolutions":[{"name":"Class On Grass"}]}
]}`

func TestEvolutionPathsFiltraPelaCartaCerta(t *testing.T) {
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

	// Pedindo a carta de eaId 50537761: só o primeiro path bate — o "id"
	// interno (136512) tem que ser ignorado a favor do "eaId" pro filtro.
	paths, err := c.EvolutionPaths(context.Background(), 206113, 50537761)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("achou %d caminhos, esperava 1 (o da carta 50537761)", len(paths))
	}
	if got := paths[0].Final().Rating; got != 98 {
		t.Errorf("overall final = %d, esperava 98", got)
	}
	if got := paths[0].Final().GGRating; got != 97.5 {
		t.Errorf("GG Rating final = %v, esperava 97.5", got)
	}
	if paths[0].CoinsCost != 10000 {
		t.Errorf("custo = %d, esperava 10000", paths[0].CoinsCost)
	}

	// A outra carta do mesmo jogador-base (117646625) pede o outro path.
	paths2, err := c.EvolutionPaths(context.Background(), 206113, 117646625)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths2) != 1 || paths2[0].CoinsCost != 50000 {
		t.Fatalf("esperava 1 caminho de custo 50000, achou %+v", paths2)
	}
}

// Carta sem nenhum caminho correspondente (já está no teto que as
// evoluções ativas alcançam) devolve lista vazia, não erro.
func TestEvolutionPathsSemCorrespondenciaDevolveVazio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(evoPathsFixture))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Cycle: "26", Endpoints: map[string]string{
		"evolution_paths": "/api/fut/evolutions/v2/26/paths/v2/{id}/",
	}})

	paths, err := c.EvolutionPaths(context.Background(), 206113, 999999999)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Errorf("achou %d caminhos pra uma carta que não existe na resposta", len(paths))
	}
}
