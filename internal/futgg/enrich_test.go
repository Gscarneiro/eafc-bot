package futgg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Ter duas CÓPIAS da mesma carta no clube é normal no FUT, e o elenco real
// tem várias. A nota por posição precisa chegar nas duas: parando na
// primeira, a segunda ficava só com a nota escalar e chegava mais fraca do
// que é na hora de escalar (analyze.gauntletValue), quando não sumia do pool.
func TestEnrichPositionRatingsAtualizaTodasAsCopiasDaMesmaCarta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"eaId":168027413,"position":14,"score":94.05}]}`))
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Endpoints: map[string]string{"metarank": "/api/fut/metarank/players/"},
	})

	carta := domain.ClubPlayer{Player: domain.Player{ID: 168027413, Position: domain.CM}}
	club := domain.Club{
		Players: []domain.ClubPlayer{carta, carta},
		Squad:   domain.Squad{Starters: []domain.SquadSlot{{Index: 0, Position: domain.CM, PlayerID: 168027413}}},
	}

	if err := c.enrichPositionRatings(context.Background(), &club); err != nil {
		t.Fatal(err)
	}
	for i, p := range club.Players {
		got, ok := p.GGRatingAt(domain.CM)
		if !ok || got != 94.05 {
			t.Fatalf("cópia %d ficou com GGRatingAt(CM) = %v/%v, esperava 94.05 — a nota parou na primeira linha", i, got, ok)
		}
	}
}
