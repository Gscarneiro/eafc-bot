package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestFeedbackGameplayDeduplicaPorContextoDeComparacao(t *testing.T) {
	srv, _ := newTestServer(t)
	send := func(preference string) {
		t.Helper()
		body, err := json.Marshal(domain.FeedbackGameplay{
			ComparacaoID: "mbappe-haaland|26.1|ps5|ST", CartaA: "Mbappé", CartaB: "Haaland",
			Preferencia: preference, Patch: "26.1", Plataforma: "ps5", Posicao: domain.ST,
		})
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/feedback/gameplay", bytes.NewReader(body)))
		if w.Code != http.StatusOK {
			t.Fatalf("registrando feedback: status %d: %s", w.Code, w.Body.String())
		}
	}
	send("a")
	send("b")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/feedback/gameplay", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("lendo feedback: status %d: %s", w.Code, w.Body.String())
	}
	response := decodeJSON[struct {
		Value []domain.FeedbackGameplay `json:"value"`
		Count int                       `json:"@odata.count"`
	}](t, w)
	if response.Count != 1 || len(response.Value) != 1 || response.Value[0].Preferencia != "b" {
		t.Fatalf("deduplicação não preservou a última correção: %+v", response)
	}
}
