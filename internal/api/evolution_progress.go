package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gscarneiro/eafc-bot/internal/store"
)

// EvolutionProgressResponse é a lista de nomes de evolução que o usuário
// marcou como já concluídos PARA UMA CARTA — nunca aplicado na conta EA,
// só uma anotação local (ver ConfigEditor.GetProgress/UpdateProgress).
type EvolutionProgressResponse struct {
	Completed []string `json:"completed"`
}

// maxEvolutionProgressItems é menor que o teto de favoritos (500): isto é
// progresso de UMA carta, não uma lista global.
const maxEvolutionProgressItems = 100

func (s *Server) handleEvolucoesProgressoUpdate(w http.ResponseWriter, r *http.Request) {
	if s.Config == nil || s.Config.UpdateProgress == nil {
		http.Error(w, "progresso indisponível", http.StatusNotImplemented)
		return
	}
	slug := r.PathValue("slug")
	var in EvolutionProgressResponse
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, fmt.Sprintf("lendo progresso: %v", err), http.StatusBadRequest)
		return
	}
	if len(in.Completed) > maxEvolutionProgressItems {
		http.Error(w, "lista de progresso grande demais", http.StatusRequestEntityTooLarge)
		return
	}
	if err := s.Config.UpdateProgress(slug, in.Completed); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, EvolutionProgressResponse{Completed: in.Completed})
}

// EvolutionProgressItem é uma linha do painel "Evoluções / pipeline" de
// Hoje: um path SALVO (ver /api/evolucoes/caminhos/salvos) com o progresso
// já cruzado contra ConfigEditor.AllProgress — Total vem de Path.Chain (os
// passos do caminho salvo), Completed conta quantos desses nomes o usuário
// já marcou pra ESTA carta (Config.Progress é chaveado por slug).
type EvolutionProgressItem struct {
	CardSlug  string   `json:"card_slug"`
	Name      string   `json:"name"`
	Completed int      `json:"completed"`
	Total     int      `json:"total"`
	Steps     []string `json:"steps"`
	Done      []string `json:"done"`
}

type EvolutionProgressListResponse struct {
	Items []EvolutionProgressItem `json:"items"`
}

func (s *Server) handleEvolucoesProgressoList(w http.ResponseWriter, r *http.Request) {
	if s.Config == nil || s.Config.AllProgress == nil {
		http.Error(w, "progresso indisponível", http.StatusNotImplemented)
		return
	}
	backend, ok := s.Store.(store.SavedEvolutionPathStore)
	if !ok {
		http.Error(w, "paths salvos indisponíveis neste armazenamento", http.StatusNotImplemented)
		return
	}
	saved, err := backend.ListSavedEvolutionPaths(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, "lendo paths salvos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	all := s.Config.AllProgress()
	items := make([]EvolutionProgressItem, 0, len(saved))
	for _, path := range saved {
		steps := path.Path.Chain
		done := all[path.CardSlug]
		doneSet := make(map[string]bool, len(done))
		for _, name := range done {
			doneSet[name] = true
		}
		completed := 0
		for _, step := range steps {
			if doneSet[step] {
				completed++
			}
		}
		items = append(items, EvolutionProgressItem{
			CardSlug: path.CardSlug, Name: path.Player.Display(),
			Completed: completed, Total: len(steps), Steps: steps, Done: done,
		})
	}
	writeJSON(w, EvolutionProgressListResponse{Items: items})
}
