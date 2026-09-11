package api

import (
	"net/http"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

type EvaluationCatalogResponse struct {
	Active   domain.ContextoAvaliacao `json:"active"`
	Profiles []domain.PerfilMeta      `json:"profiles"`
	Sources  []EvaluationSourceView   `json:"sources"`
}

type EvaluationSourceView struct {
	ID          domain.FonteAvaliacao `json:"id"`
	Name        string                `json:"name"`
	Available   bool                  `json:"available"`
	Description string                `json:"description"`
}

// handleEvaluationCatalog deixa explícito que FUTBIN e FUTWIZ usam seus
// adaptadores posicionais, não médias calculadas contra o FUT.GG.
func (s *Server) handleEvaluationCatalog(w http.ResponseWriter, r *http.Request) {
	var profiles []domain.PerfilMeta
	if evaluator := s.resolveEvaluator(); evaluator != nil {
		profiles = evaluator.Perfis()
	}
	writeJSON(w, EvaluationCatalogResponse{
		Active: s.resolveEvaluationContext(), Profiles: profiles,
		Sources: []EvaluationSourceView{
			{ID: domain.FonteFutGG, Name: "FUT.GG", Available: true, Description: "nota posicional publicada pela fonte"},
			{ID: domain.FonteBot, Name: "Nota do bot", Available: true, Description: "perfil local, determinístico e explicável"},
			{ID: domain.FonteFUTBIN, Name: "FUTBIN", Available: true, Description: "importa nota posicional com identidade, métrica, escala e contexto comprovados"},
			{ID: domain.FonteFUTWIZ, Name: "FUTWIZ", Available: true, Description: "importa nota posicional com identidade, métrica, escala e contexto comprovados"},
		},
	})
}
