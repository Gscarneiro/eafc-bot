package api

import (
	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"net/http"
	"time"
)

func (s *Server) handleAgenda(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	watch, err := s.Store.ListWatchlist(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, "lendo watchlist local: "+err.Error(), http.StatusInternalServerError)
		return
	}
	ledger, err := s.Store.ListLedger(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, "lendo ledger local: "+err.Error(), http.StatusInternalServerError)
		return
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
	writeJSON(w, analyze.MontarAgenda(analyze.AgendaInput{Mercado: market, Evolucoes: snap.EvoMatches, SBCs: snap.SBCs, Watchlist: watch, Agora: time.Now()}))
}
