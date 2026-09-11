package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// buildAgenda monta a Agenda do dia a partir do snapshot — extraído de
// handleAgenda para que /api/resumo (avisos, selo de conflito) reuse a MESMA
// montagem em vez de recalcular o plano de mercado com outra combinação de
// insumos, o que produziria conflitos que não batem com os da tela /agenda.
func (s *Server) buildAgenda(ctx context.Context, snap store.Snapshot) (analyze.Agenda, error) {
	watch, err := s.Store.ListWatchlist(ctx, s.Cycle)
	if err != nil {
		return analyze.Agenda{}, err
	}
	ledger, err := s.Store.ListLedger(ctx, s.Cycle)
	if err != nil {
		return analyze.Agenda{}, err
	}
	prices := marketPriceAssessments(snap, time.Now())
	by := map[int64]analyze.PriceAssessment{}
	for _, p := range prices {
		by[p.EAID] = p
	}
	sp := analyze.DefaultSquadPlanRequest()
	sp.ChemistryModel = s.resolveChemistryModel()
	squad := analyze.BuildSquadPlan(snap.Club, sp)
	needs := make([]analyze.MarketNeed, 0, len(squad.Needs))
	for _, n := range squad.Needs {
		needs = append(needs, analyze.MarketNeed{Position: n.Position, Reason: n.Reason})
	}
	sells, _ := analyze.FindSellCandidates(snap.Club, snap.Cards, snap.SquadSwaps, analyze.DefaultSellOptions())
	market := analyze.PlanMarket(analyze.MarketPlanInput{Capital: snap.Club.Capital(s.EvolutionExtraBudget, s.MarketReserve, domain.SummarizeLedger(ledger).Committed), Needs: needs, Upgrades: snap.Upgrades, Evolutions: snap.EvoMatches, Sells: sells, Watchlist: watch, Prices: by, GeneratedAt: time.Now()})
	return analyze.MontarAgenda(analyze.AgendaInput{Mercado: market, Evolucoes: snap.EvoMatches, SBCs: snap.SBCs, Watchlist: watch, Agora: time.Now()}), nil
}

// AgendaResponse é a Agenda mais o feedback já cruzado por ActionID — o
// checklist "Tarefas de hoje" de Hoje não devia buscar /api/agenda e
// /api/feedback separado só pra fazer esse join na mão.
type AgendaResponse struct {
	Agenda analyze.Agenda `json:"agenda"`
	// Feedback é o status mais recente por ActionID — feedback é
	// append-only (ver domain.DecisionFeedback), então mudar de ideia
	// grava outro evento em vez de reescrever; aqui só o último vale.
	Feedback   map[string]domain.FeedbackStatus `json:"feedback"`
	Concluidas int                              `json:"concluidas"`
	Total      int                              `json:"total"`
}

// latestFeedbackByAction reduz o histórico append-only ao status mais
// recente por ação — mesmo critério de ordenação que analyze.AvaliarFeedback
// usa (RecordedAt), pra "concluída" não divergir entre as duas contas.
func latestFeedbackByAction(entries []domain.DecisionFeedback) map[string]domain.FeedbackStatus {
	out := make(map[string]domain.FeedbackStatus, len(entries))
	seenAt := make(map[string]time.Time, len(entries))
	for _, e := range entries {
		if last, ok := seenAt[e.ActionID]; ok && !e.RecordedAt.After(last) {
			continue
		}
		out[e.ActionID] = e.Status
		seenAt[e.ActionID] = e.RecordedAt
	}
	return out
}

func (s *Server) handleAgenda(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	agenda, err := s.buildAgenda(r.Context(), snap)
	if err != nil {
		http.Error(w, "montando agenda: "+err.Error(), http.StatusInternalServerError)
		return
	}
	feedbackEntries, err := s.Store.ListFeedback(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, "lendo feedback local: "+err.Error(), http.StatusInternalServerError)
		return
	}
	feedback := latestFeedbackByAction(feedbackEntries)

	total := len(agenda.Agora) + len(agenda.EstaSemana) + len(agenda.Observando)
	concluidas := 0
	for _, faixa := range [][]analyze.AcaoAgenda{agenda.Agora, agenda.EstaSemana, agenda.Observando} {
		for _, acao := range faixa {
			if feedback[acao.ID] == domain.FeedbackAceita {
				concluidas++
			}
		}
	}

	writeJSON(w, AgendaResponse{Agenda: agenda, Feedback: feedback, Concluidas: concluidas, Total: total})
}
