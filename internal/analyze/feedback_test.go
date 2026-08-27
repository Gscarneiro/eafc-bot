package analyze

import (
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"testing"
	"time"
)

func TestFeedbackNaoLiberaCalibracaoAntesDeTrintaDesfechos(t *testing.T) {
	entries := make([]domain.DecisionFeedback, 29)
	for i := range entries {
		entries[i] = domain.DecisionFeedback{Cycle: "26", Profile: string(DefaultBotScoreProfile), Status: domain.FeedbackAceita, LaterOutcome: "ok", RecordedAt: time.Now()}
	}
	got := AvaliarFeedback(entries, "26", string(DefaultBotScoreProfile))
	if got.CalibracaoDisponivel || got.ComDesfecho != 29 {
		t.Fatalf("metricas=%+v", got)
	}
}
