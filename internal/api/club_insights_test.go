package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestColecaoEInsightsDoClubeSaoColecoesOData(t *testing.T) {
	snap := fixtureSnapshot()
	snap.Club.SyncedAt = time.Now()
	snap.Club.Source = "futgg"
	snap.Club.Players = append(snap.Club.Players, domain.ClubPlayer{Player: domain.Player{ID: 7, Name: "Duplicada", CommonName: "Duplicada", Rating: 84, Position: domain.CB, Cycle: "26"}})
	srv, _ := newTestServerWithSnapshot(t, snap)

	for _, path := range []string{"/api/clube/colecao?$orderby=count%20desc", "/api/clube/insights?$filter=kind%20eq%20'fodder_value'"} {
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", path, w.Code, w.Body.String())
		}
		var page struct {
			Value []any `json:"value"`
			Count int   `json:"@odata.count"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Value) == 0 || page.Count == 0 {
			t.Fatalf("%s devolveu página vazia: %s", path, w.Body.String())
		}
	}
}
