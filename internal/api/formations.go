package api

import (
	"net/http"

	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/formations"
)

// O catálogo está embutido no binário para o editor abrir antes da primeira
// coleta e para nenhuma navegação do campo depender da rede do fut.gg.
func (s *Server) handleFormations(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, struct {
		Cycle string                 `json:"ciclo"`
		Value []formations.Formation `json:"value"`
	}{Cycle: "27", Value: formations.Catalog()})
}

func formationForGauntlet(name string, squad domain.Squad) string {
	if name != "" {
		return name
	}
	if squad.Formation != "" {
		return squad.Formation
	}
	return formations.Infer(squad.Starters)
}
