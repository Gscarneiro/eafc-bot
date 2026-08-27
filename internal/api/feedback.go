package api

import (
	"encoding/json"
	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"net/http"
	"time"
)

type FeedbackResponse struct {
	Entries []domain.DecisionFeedback `json:"entries"`
	Metrics analyze.FeedbackMetrics   `json:"metrics"`
}

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	entries, err := s.Store.ListFeedback(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, "lendo feedback local: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, FeedbackResponse{Entries: entries, Metrics: analyze.AvaliarFeedback(entries, s.Cycle, string(analyze.DefaultBotScoreProfile))})
}
func (s *Server) handleFeedbackAppend(w http.ResponseWriter, r *http.Request) {
	var entry domain.DecisionFeedback
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "corpo de feedback invalido: "+err.Error(), http.StatusBadRequest)
		return
	}
	if entry.ID == "" {
		entry.ID = localID("feedback")
	}
	entry.Cycle = s.Cycle
	if entry.Profile == "" {
		entry.Profile = string(analyze.DefaultBotScoreProfile)
	}
	if entry.RecordedAt.IsZero() {
		entry.RecordedAt = time.Now()
	}
	if err := s.Store.AppendFeedback(r.Context(), s.Cycle, entry); err != nil {
		http.Error(w, "gravando feedback: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, entry)
}
