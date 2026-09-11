package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestAgendaRespondeComFaixas(t *testing.T) {
	srv, _ := newTestServer(t)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/agenda", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d %s", w.Code, w.Body.String())
	}
	if w.Body.Len() == 0 {
		t.Fatal("agenda vazia")
	}
}

// O checklist "Tarefas de hoje" marca concluída pelo status MAIS RECENTE de
// feedback (append-only) — um "adiada" antigo seguido de um "aceita" novo
// tem que contar como concluída, não a primeira entrada gravada.
func TestAgendaMarcaTarefaConcluidaPeloFeedbackMaisRecente(t *testing.T) {
	srv, st := newTestServer(t)

	w1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/api/agenda", nil))
	if w1.Code != http.StatusOK {
		t.Fatalf("status=%d %s", w1.Code, w1.Body.String())
	}
	first := decodeJSON[AgendaResponse](t, w1)
	if first.Total == 0 {
		t.Fatal("fixture não tem nenhuma ação na agenda — não dá pra testar o join")
	}
	if first.Concluidas != 0 {
		t.Fatalf("Concluidas = %d antes de qualquer feedback, esperava 0", first.Concluidas)
	}

	var alvo string
	for _, faixa := range [][]analyze.AcaoAgenda{first.Agenda.Agora, first.Agenda.EstaSemana, first.Agenda.Observando} {
		if len(faixa) > 0 {
			alvo = faixa[0].ID
			break
		}
	}
	if alvo == "" {
		t.Fatal("nenhuma faixa da agenda tem ação")
	}

	ctx := context.Background()
	if err := st.AppendFeedback(ctx, "26", domain.DecisionFeedback{ID: "f1", ActionID: alvo, Cycle: "26", Status: domain.FeedbackAdiada, RecordedAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatalf("AppendFeedback (adiada): %v", err)
	}
	if err := st.AppendFeedback(ctx, "26", domain.DecisionFeedback{ID: "f2", ActionID: alvo, Cycle: "26", Status: domain.FeedbackAceita, RecordedAt: time.Now()}); err != nil {
		t.Fatalf("AppendFeedback (aceita): %v", err)
	}

	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/agenda", nil))
	second := decodeJSON[AgendaResponse](t, w2)
	if second.Concluidas != 1 {
		t.Fatalf("Concluidas = %d depois do feedback mais recente ser 'aceita', esperava 1", second.Concluidas)
	}
	if second.Feedback[alvo] != domain.FeedbackAceita {
		t.Fatalf("Feedback[%s] = %q, esperava %q (o mais recente, não o primeiro 'adiada')", alvo, second.Feedback[alvo], domain.FeedbackAceita)
	}
}
