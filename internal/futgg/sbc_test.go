package futgg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// challenges[] existia na resposta real do fut.gg (autoconfig já tinha
// aprendido field_maps.sbcs.challenges -> "challenges") mas ninguém lia —
// o requisito e o preço da solução mais barata (que o fut.gg já resolve)
// ficavam presos na resposta crua. Console é o padrão quando Platform não
// está configurada.
func TestSBCsLeChallengesEUsaConsolePorPadrao(t *testing.T) {
	body := `{"data":[
 {"slug":"portugal-icons","name":"Portugal Icons","category":"Nation",
  "isRepeatable":false,"cost":48450,"endTime":"2026-09-11T17:00:01Z",
  "challenges":[
    {"name":"Portugal","requirementsText":["Min. 1 Players from: Portugal","Min. Team Rating: 89"],
     "cheapestSolutionPrice":46900,"cheapestSolutionPricePc":48450}
  ]}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Endpoints: map[string]string{"sbcs": "/api/fut/sbc/"},
	})

	sbcs, err := c.SBCs(context.Background())
	if err != nil {
		t.Fatalf("SBCs: %v", err)
	}
	if len(sbcs) != 1 {
		t.Fatalf("esperava 1 SBC, veio %d", len(sbcs))
	}
	if len(sbcs[0].Challenges) != 1 {
		t.Fatalf("esperava 1 challenge, veio %d", len(sbcs[0].Challenges))
	}
	ch := sbcs[0].Challenges[0]
	if ch.Name != "Portugal" {
		t.Errorf("nome do challenge = %q, esperava \"Portugal\"", ch.Name)
	}
	if len(ch.RequirementsText) != 2 {
		t.Fatalf("esperava 2 linhas de requisito, veio %d: %v", len(ch.RequirementsText), ch.RequirementsText)
	}
	if ch.CheapestSolutionCoins != 46900 {
		t.Errorf("sem platform configurada deveria ler o preço de console (46900), veio %d", ch.CheapestSolutionCoins)
	}
}

// Com futgg.Config.Platform = "PC", lê cheapestSolutionPricePc em vez do
// preço de console — testado ao vivo em 22/08/2026 divergindo de verdade
// (46900 console / 48450 PC nesta mesma SBC).
func TestSBCsUsaPrecoDePCQuandoPlataformaConfigurada(t *testing.T) {
	body := `{"data":[
 {"slug":"portugal-icons","name":"Portugal Icons",
  "challenges":[{"name":"Portugal","cheapestSolutionPrice":46900,"cheapestSolutionPricePc":48450}]}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Platform:  "PC",
		Endpoints: map[string]string{"sbcs": "/api/fut/sbc/"},
	})

	sbcs, err := c.SBCs(context.Background())
	if err != nil {
		t.Fatalf("SBCs: %v", err)
	}
	if got := sbcs[0].Challenges[0].CheapestSolutionCoins; got != 48450 {
		t.Errorf("com Platform=PC deveria ler 48450, veio %d", got)
	}
}
