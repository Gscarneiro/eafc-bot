package domain

import (
	"fmt"
	"strings"
	"time"
)

type FeedbackStatus string

const (
	FeedbackAceita     FeedbackStatus = "aceita"
	FeedbackAdiada     FeedbackStatus = "adiada"
	FeedbackDescartada FeedbackStatus = "descartada"
)

// DecisionFeedback e append-only: mudar opiniao cria outro evento, para a
// avaliacao temporal nunca reescrever o que era conhecido naquele dia.
type DecisionFeedback struct {
	ID           string         `json:"id"`
	ActionID     string         `json:"action_id"`
	Cycle        string         `json:"cycle"`
	Profile      string         `json:"profile"`
	Status       FeedbackStatus `json:"status"`
	Reason       string         `json:"reason,omitempty"`
	LaterOutcome string         `json:"later_outcome,omitempty"`
	SnapshotDate string         `json:"snapshot_date,omitempty"`
	RecordedAt   time.Time      `json:"recorded_at"`
}

func (f DecisionFeedback) Validate() error {
	if strings.TrimSpace(f.ID) == "" || strings.TrimSpace(f.ActionID) == "" || strings.TrimSpace(f.Cycle) == "" {
		return fmt.Errorf("feedback precisa de id, action_id e ciclo")
	}
	switch f.Status {
	case FeedbackAceita, FeedbackAdiada, FeedbackDescartada:
	default:
		return fmt.Errorf("status de feedback invalido: %q", f.Status)
	}
	return nil
}
