package futgg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Recorte da resposta real de /api/fut/metarank/player/245830/?game=27 (Yui
// Hasegawa, 23/09/2026): a página da carta mostra CM 85.8, CDM 85.5, CAM
// 81.2 — o maior score entre as funções de cada posição, em qualquer estilo
// de química. A função 999 não existe na tabela e tem de ser ignorada.
const metarankHasegawa = `{"data":{"eaId":245830,"scores":[
	{"role":24,"chemistryStyle":1,"score":84.9,"rank":31,"isPlus":false,"isPlusPlus":true},
	{"role":24,"chemistryStyle":19,"score":85.83,"rank":31,"isPlus":false,"isPlusPlus":true},
	{"role":27,"chemistryStyle":19,"score":84.3,"rank":89,"isPlus":false,"isPlusPlus":false},
	{"role":15,"chemistryStyle":19,"score":85.53,"rank":45,"isPlus":true,"isPlusPlus":false},
	{"role":14,"chemistryStyle":1,"score":81.81,"rank":54,"isPlus":false,"isPlusPlus":false},
	{"role":75,"chemistryStyle":18,"score":81.52,"rank":66,"isPlus":false,"isPlusPlus":false},
	{"role":37,"chemistryStyle":19,"score":81.2,"rank":150,"isPlus":true,"isPlusPlus":false},
	{"role":999,"chemistryStyle":1,"score":99.0,"rank":1,"isPlus":false,"isPlusPlus":false}
]}}`

func servidorMetarank(t *testing.T, body string) (*Client, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/fut/metarank/player/245830/" || r.URL.Query().Get("game") != "27" {
			http.NotFound(w, r)
			return
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	// Config sem a rota, como um config.json anterior a ela: New tem de
	// acrescentar o padrão sozinho.
	return New(Config{BaseURL: srv.URL, Cycle: "27", Endpoints: map[string]string{"club": "/api/gg-club/{gamertag}/players/"}}), &calls
}

func clubeComHasegawa(ggPublicado float64) domain.Club {
	carta := domain.Player{ID: 245830, CommonName: "Yui Hasegawa", Position: domain.CDM, AltPositions: []domain.Position{domain.CAM, domain.CM}, GGRating: ggPublicado, GGRatingPos: domain.CM}
	return domain.Club{Players: []domain.ClubPlayer{
		{Player: carta, ClubItemID: "copia-1"},
		{Player: carta, ClubItemID: "copia-2"},
	}}
}

func TestGGPorPosicaoUsaAMaiorFuncaoDeCadaVagaEmTodasAsCopias(t *testing.T) {
	c, calls := servidorMetarank(t, metarankHasegawa)
	club := clubeComHasegawa(85.83)

	res := c.PreencherGGPorPosicao(context.Background(), &club)
	if res.Preenchidas != 1 || res.Divergentes != 0 || res.Falhas != 0 || res.Aviso("27") != "" {
		t.Fatalf("resultado = %+v, aviso %q", res, res.Aviso("27"))
	}
	if *calls != 1 {
		t.Fatalf("duas cópias da mesma carta dividem a nota do eaId; esperava 1 requisição, houve %d", *calls)
	}
	for _, copia := range club.Players {
		notas := copia.GGRatingPorPosicao
		if notas[domain.CDM] != 85.53 || notas[domain.CM] != 85.83 || notas[domain.CAM] != 81.2 {
			t.Fatalf("%s: tabela = %v, esperava CDM 85.53 / CM 85.83 / CAM 81.2 como na página", copia.ClubItemID, notas)
		}
		if len(notas) != 3 {
			t.Fatalf("função desconhecida (999) não pode virar posição: %v", notas)
		}
		if v, ok := copia.GGRatingAt(domain.CDM); !ok || v != 85.53 {
			t.Fatalf("GGRatingAt(CDM) = %v, %v; a CDM de ofício precisa ter nota", v, ok)
		}
	}
}

// Se o fut.gg renumerar as funções num ciclo novo, a tabela deixa de
// reproduzir o GG publicado. Aí ela é descartada — a carta volta a ter nota
// só na melhor posição, em vez de uma nota na posição errada — e a coleta
// avisa com o comando para conferir.
func TestGGPorPosicaoDescartaTabelaQueNaoReproduzOGGPublicado(t *testing.T) {
	c, _ := servidorMetarank(t, metarankHasegawa)
	club := clubeComHasegawa(88.1)

	res := c.PreencherGGPorPosicao(context.Background(), &club)
	if res.Divergentes != 1 || res.Preenchidas != 0 {
		t.Fatalf("resultado = %+v, esperava a tabela descartada", res)
	}
	for _, copia := range club.Players {
		if copia.GGRatingPorPosicao != nil {
			t.Fatalf("tabela que não confere foi gravada: %v", copia.GGRatingPorPosicao)
		}
	}
	if aviso := res.Aviso("27"); !strings.Contains(aviso, "descartada em 1 de 1") || !strings.Contains(aviso, "eafcbot discover") {
		t.Fatalf("o aviso precisa dizer o que aconteceu e como conferir: %q", aviso)
	}
}
