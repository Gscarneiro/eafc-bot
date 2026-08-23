package futgg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Testado ao vivo em 22/08/2026: o fut.gg ignora o parâmetro price__lte que
// PlayerFilter.query manda — 82 das 240 cartas de uma coleta com
// max_price=100000 voltaram acima do teto, a mais cara a 10.000.000. Players()
// precisa cortar de verdade do lado de cá, sem descartar a carta sem cotação
// que AllowUnpriced existe para tratar.
func TestMaxPriceDescartaAcimaDoTetoMasMantemCartaSemPreco(t *testing.T) {
	body := `{"data":[
 {"id":1,"overall":90,"position":"ST","commonName":"Barato","price":50000},
 {"id":2,"overall":91,"position":"ST","commonName":"Caro","price":200000},
 {"id":3,"overall":92,"position":"ST","commonName":"SemPreco"},
 {"id":4,"overall":93,"position":"ST","commonName":"NoTeto","price":100000}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Endpoints: map[string]string{"players": "/api/fut/players/"},
		Wrappers:  map[string]string{"players": "data"},
	})

	players, err := c.Players(context.Background(), PlayerFilter{MaxPrice: 100_000, Pages: 1})
	if err != nil {
		t.Fatalf("Players: %v", err)
	}

	got := map[int64]bool{}
	for _, p := range players {
		got[p.ID] = true
	}
	if got[2] {
		t.Error("carta acima do max_price (200000 > 100000) não deveria ter passado")
	}
	if !got[1] || !got[4] {
		t.Errorf("cartas dentro do teto (inclusive no limite) deveriam ter passado: %v", got)
	}
	if !got[3] {
		t.Error("carta sem cotação não pode ser descartada pelo filtro de preço — AllowUnpriced existe para ela")
	}
	if len(players) != 3 {
		t.Fatalf("esperava 3 cartas (id 2 descartada), veio %d: %v", len(players), players)
	}

	if skipped := c.Stats().MarketPriceSkipped; skipped != 1 {
		t.Errorf("MarketPriceSkipped = %d, esperava 1", skipped)
	}
}
