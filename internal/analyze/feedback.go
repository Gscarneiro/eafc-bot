package analyze

import (
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"sort"
)

type FeedbackMetrics struct {
	Total                int    `json:"total"`
	Aceitas              int    `json:"aceitas"`
	Adiadas              int    `json:"adiadas"`
	Descartadas          int    `json:"descartadas"`
	ComDesfecho          int    `json:"com_desfecho"`
	Perfil               string `json:"perfil"`
	CalibracaoDisponivel bool   `json:"calibracao_disponivel"`
	Motivo               string `json:"motivo,omitempty"`
}

// AvaliarFeedback ordena por tempo e somente descreve o historico. Nunca
// ajusta peso: calibracao e sugestao humana, e so abre apos 30 decisoes com
// desfecho no mesmo perfil e ciclo.
func AvaliarFeedback(entries []domain.DecisionFeedback, cycle, profile string) FeedbackMetrics {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].RecordedAt.Before(entries[j].RecordedAt) })
	m := FeedbackMetrics{Perfil: profile}
	for _, e := range entries {
		if e.Cycle != cycle || e.Profile != profile {
			continue
		}
		m.Total++
		if e.LaterOutcome != "" {
			m.ComDesfecho++
		}
		switch e.Status {
		case domain.FeedbackAceita:
			m.Aceitas++
		case domain.FeedbackAdiada:
			m.Adiadas++
		case domain.FeedbackDescartada:
			m.Descartadas++
		}
	}
	if m.ComDesfecho >= 30 {
		m.CalibracaoDisponivel = true
	} else {
		m.Motivo = "calibracao requer 30 decisoes com desfecho no mesmo ciclo e perfil"
	}
	return m
}
