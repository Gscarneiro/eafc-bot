package futgg

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// O bot não infere tendência do próprio histórico esparso (1 ponto/carta/
// dia na operação normal) — lê o momentumPercentage que o fut.gg já
// calcula. Testado ao vivo em 22/08/2026: mesmo envelope de
// /api/fut/players/v2/26/, sem precisar de field_maps novo.
func TestMomentumLeMomentumPctDoMesmoEnvelopeDePlayers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":[
 {"id":1,"overall":90,"position":"ST","commonName":"Descontado","price":50000,"momentumPercentage":22.4},
 {"id":2,"overall":91,"position":"ST","commonName":"SemMomentum","price":60000}],
 "next":null,"currentPage":1,"total":2}`)
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Endpoints: map[string]string{"momentum": "/api/fut/players/v2/momentum/{hours}/"},
	})

	players, err := c.Momentum(context.Background(), MomentumOptions{Pages: 1})
	if err != nil {
		t.Fatalf("Momentum: %v", err)
	}
	if len(players) != 2 {
		t.Fatalf("esperava 2 cartas, veio %d", len(players))
	}

	byID := map[int64]float64{}
	for _, p := range players {
		byID[p.ID] = p.MomentumPct
	}
	if byID[1] != 22.4 {
		t.Errorf("MomentumPct da carta 1 = %v, esperava 22.4", byID[1])
	}
	if byID[2] != 0 {
		t.Errorf("carta sem momentumPercentage deveria ficar em 0 (fonte não trouxe), veio %v", byID[2])
	}
}

// hours é SEGMENTO DE PATH, não query param — a primeira versão deste
// código mandava "?hours=N" e o site respondia 404 em produção. Achado
// lendo o bundle JS de produção do próprio fut.gg em 22/08/2026
// (searchMomentum: sc({path: "players/v2/momentum/:hours/"})) e
// confirmado ao vivo contra /api/fut/players/v2/momentum/24/?page=1
// (HTTP 200, dados reais). page continua sendo query param.
func TestMomentumMandaHoursComoSegmentoDePathEPageComoQuery(t *testing.T) {
	var gotPaths, gotQueries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		gotQueries = append(gotQueries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":[],"next":null,"currentPage":1,"total":0}`)
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Endpoints: map[string]string{"momentum": "/api/fut/players/v2/momentum/{hours}/"},
	})

	if _, err := c.Momentum(context.Background(), MomentumOptions{Hours: 6, Pages: 1}); err != nil {
		t.Fatalf("Momentum: %v", err)
	}
	if len(gotPaths) != 1 || !strings.Contains(gotPaths[0], "/momentum/6/") {
		t.Fatalf("esperava hours=6 como segmento de path (/momentum/6/), veio path=%v", gotPaths)
	}
	if len(gotQueries) != 1 || !strings.Contains(gotQueries[0], "page=1") {
		t.Fatalf("esperava page=1 como query param, veio %v", gotQueries)
	}
	if strings.Contains(gotQueries[0], "hours") {
		t.Errorf("hours não deveria aparecer na query string, veio %q", gotQueries[0])
	}
}

// Pages limita quantas páginas o bot busca — o endpoint tem ~206 páginas
// no total (6178 cartas / 30), e já vem ordenado por maior desconto
// primeiro: não precisamos do catálogo inteiro pra achar os melhores
// candidatos.
func TestMomentumRespeitaLimiteDePaginas(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":[],"next":null,"currentPage":1,"total":6178}`)
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Endpoints: map[string]string{"momentum": "/api/fut/players/v2/momentum/{hours}/"},
	})

	if _, err := c.Momentum(context.Background(), MomentumOptions{Pages: 3}); err != nil {
		t.Fatalf("Momentum: %v", err)
	}
	if calls != 3 {
		t.Fatalf("esperava 3 chamadas (Pages: 3), veio %d", calls)
	}
}
