package api

import (
	"net/http"
	"strings"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// EstimatedEvolution é uma evolução que passa nas regras do catálogo
// (analyze.Eligible) mas que o grafo confirmado (cards.CardReport.Graph)
// não tem nenhuma transição para ela — a "elegibilidade estimada" que o
// contrato da fase pede pra não sumir em silêncio, sempre rotulada como tal
// e nunca misturada com Graph.
type EstimatedEvolution struct {
	Evolution   domain.Evolution      `json:"evolution"`
	Acquisition string                `json:"acquisition"`
	Status      cards.EvolutionStatus `json:"status"` // sempre no_path ou fetch_error aqui
}

// EvolutionPlanResponse é o Workbench de UMA carta: o grafo confirmado
// (quando existe), as evoluções estimadas sem caminho confirmado, e o
// progresso que o usuário marcou manualmente — nunca aplicado na conta EA.
type EvolutionPlanResponse struct {
	Slug          string                 `json:"slug"`
	Status        cards.EvolutionStatus  `json:"status"`
	Error         string                 `json:"error,omitempty"`
	Graph         *domain.EvolutionGraph `json:"graph,omitempty"`
	EstimatedOnly []EstimatedEvolution   `json:"estimated_only,omitempty"`
	Completed     []string               `json:"completed,omitempty"`
}

func (s *Server) handleEvolucoesPlano(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	slug := r.PathValue("slug")
	entries := cards.BuildCatalog(snap.Club, snap.Cards, snap.RoleCatalog)
	entry, ok := cards.FindCatalog(entries, slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var completed []string
	if s.Config != nil && s.Config.GetProgress != nil {
		completed = s.Config.GetProgress(slug)
	}
	writeJSON(w, buildEvolutionPlan(entry.Report, snap.Evolutions, completed))
}

// buildEvolutionPlan une o grafo confirmado com a elegibilidade estimada do
// catálogo, mostrando qual fonte sustentou cada afirmação (contrato da
// fase). analyze.Eligible fica de fora de internal/cards de propósito:
// internal/analyze já importa internal/cards (sell.go) — cards importar
// analyze de volta seria um ciclo. A composição mora aqui, no mesmo estilo
// de evoMatchView/confirmedEvoViews.
func buildEvolutionPlan(report *cards.CardReport, evolutions []domain.Evolution, completed []string) EvolutionPlanResponse {
	resp := EvolutionPlanResponse{
		Slug:      report.Slug,
		Status:    report.EvolutionStatus,
		Error:     report.EvolutionError,
		Completed: completed,
	}
	if report.EvolutionStatus == cards.EvolutionNotEligible || report.EvolutionStatus == cards.EvolutionNotChecked {
		return resp // carta nem é candidata a evolução — nada pra cruzar
	}
	resp.Graph = report.Graph

	confirmed := map[string]bool{}
	if report.Graph != nil {
		for _, t := range report.Graph.Transitions {
			if t.Source == nil {
				continue
			}
			for _, name := range t.Source.Chain {
				confirmed[strings.ToLower(strings.TrimSpace(name))] = true
			}
		}
	}

	// Sem grafo (erro de coleta), não sabemos se há caminho ou não — a
	// afirmação honesta é "erro de coleta", nunca "sem caminho".
	fallbackStatus := cards.EvolutionNoPath
	if report.EvolutionStatus == cards.EvolutionFetchError {
		fallbackStatus = cards.EvolutionFetchError
	}
	for _, evo := range evolutions {
		if !analyze.Eligible(report.Player.Player, evo) {
			continue
		}
		if confirmed[strings.ToLower(strings.TrimSpace(evo.Name))] {
			continue
		}
		resp.EstimatedOnly = append(resp.EstimatedOnly, EstimatedEvolution{
			Evolution:   evo,
			Acquisition: analyze.EvolutionAcquisition(evo),
			Status:      fallbackStatus,
		})
	}
	return resp
}
