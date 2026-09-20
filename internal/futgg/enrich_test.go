package futgg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestColetaUsaNotaDaCartaSemConsultarMetarank(t *testing.T) {
	var consultas atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/clube":
			w.Write([]byte(`{"players":[{"eaId":168027413,"position":14,"ggRating":80.36,"ggRatingPosition":14}]}`))
		case "/elenco":
			w.Write([]byte(`{"data":{"activeGroupPositions":[{"group":"FIELD","positionIdx":0,"playerEaId":168027413}]}}`))
		case "/metarank":
			consultas.Add(1)
			w.Write([]byte(`{"data":[{"eaId":168027413,"position":14,"score":94.05}]}`))
		default:
			w.Write([]byte(`{"data":[]}`))
		}
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Endpoints: map[string]string{"club": "/clube", "club_squad": "/elenco", "metarank": "/metarank"},
	})

	snap, err := c.Collect(context.Background(), "clube-teste", PlayerFilter{Pages: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Club.Players) != 1 || len(snap.Club.Squad.Starters) != 1 {
		t.Fatalf("coleta não carregou carta e vaga para exercitar o enriquecimento antigo: %+v", snap.Club)
	}
	if consultas.Load() != 0 {
		t.Fatalf("consultou metarank %d vezes apesar de a fonte não ser referência de GG Rating", consultas.Load())
	}
	p := snap.Club.Players[0]
	if got, ok := p.GGRatingAt(domain.CM); !ok || got != 80.36 || len(p.GGRatings) != 0 {
		t.Fatalf("nota = %v/%v, metarank = %v; esperava somente GG Rating 80,36", got, ok, p.GGRatings)
	}
}
