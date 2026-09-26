package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/formations"
)

func TestCatalogoDeFormacoesAbreSemSnapshot(t *testing.T) {
	s := &Server{}
	w := httptest.NewRecorder()
	s.handleFormations(w, httptest.NewRequest(http.MethodGet, "/api/formacoes", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var response struct {
		Cycle string                 `json:"ciclo"`
		Value []formations.Formation `json:"value"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Cycle != "27" || len(response.Value) != 29 {
		t.Fatalf("resposta inesperada: ciclo %q, %d formações", response.Cycle, len(response.Value))
	}
}

func TestSnapshotAntigo4213GanhaNomeNasTelasSemAlterarXI(t *testing.T) {
	positions := []domain.Position{domain.GK, domain.RB, domain.CB, domain.CB, domain.LB, domain.CDM, domain.CDM, domain.CAM, domain.RW, domain.ST, domain.LW}
	squad := domain.Squad{Starters: make([]domain.SquadSlot, len(positions))}
	for i, position := range positions {
		squad.Starters[i] = domain.SquadSlot{Index: i, Position: position}
	}
	if got := inferFormation(squad.Starters); got != "4-2-1-3" {
		t.Errorf("Meu Time e Editor: %q", got)
	}
	if got := formationForGauntlet("", squad); got != "4-2-1-3" {
		t.Errorf("Gauntlet: %q", got)
	}
	if got := formationForGauntlet("formação persistida", squad); got != "formação persistida" {
		t.Errorf("formação persistida foi sobrescrita: %q", got)
	}
}
