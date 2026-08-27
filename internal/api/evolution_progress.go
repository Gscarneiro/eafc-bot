package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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
