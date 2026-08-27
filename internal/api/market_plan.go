package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

const priceStaleAfter = 24 * time.Hour

type MarketPlanResponse struct {
	Plan            analyze.MarketPlan        `json:"plan"`
	Watchlist       []domain.WatchlistEntry   `json:"watchlist"`
	Ledger          []domain.LedgerEntry      `json:"ledger"`
	LedgerSummary   domain.LedgerSummary      `json:"ledger_summary"`
	PriceAssessment []analyze.PriceAssessment `json:"price_assessment"`
}

// handleMarketPlan junta dados já persistidos. Nem a consulta nem as rotas
// locais abaixo tocam no mercado ou na conta EA; a cotação é a do snapshot.
func (s *Server) handleMarketPlan(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	watchlist, err := s.Store.ListWatchlist(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, "lendo watchlist local: "+err.Error(), http.StatusInternalServerError)
		return
	}
	ledger, err := s.Store.ListLedger(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, "lendo ledger local: "+err.Error(), http.StatusInternalServerError)
		return
	}
	summary := domain.SummarizeLedger(ledger)
	capital := snap.Club.Capital(s.EvolutionExtraBudget, s.MarketReserve, summary.Committed)
	prices := marketPriceAssessments(snap, time.Now())
	priceByID := make(map[int64]analyze.PriceAssessment, len(prices))
	for _, price := range prices {
		priceByID[price.EAID] = price
	}
	squadRequest := analyze.DefaultSquadPlanRequest()
	squadRequest.ChemistryModel = s.resolveChemistryModel()
	squadPlan := analyze.BuildSquadPlan(snap.Club, squadRequest)
	needs := make([]analyze.MarketNeed, 0, len(squadPlan.Needs))
	for _, need := range squadPlan.Needs {
		needs = append(needs, analyze.MarketNeed{Position: need.Position, Reason: need.Reason})
	}
	sells, _ := analyze.FindSellCandidates(snap.Club, snap.Cards, snap.SquadSwaps, analyze.DefaultSellOptions())
	plan := analyze.PlanMarket(analyze.MarketPlanInput{
		Capital: capital, Needs: needs, Upgrades: snap.Upgrades, Evolutions: snap.EvoMatches,
		Sells: sells, Watchlist: watchlist, Prices: priceByID, GeneratedAt: time.Now(),
	})
	writeJSON(w, MarketPlanResponse{Plan: plan, Watchlist: watchlist, Ledger: ledger, LedgerSummary: summary, PriceAssessment: prices})
}

func marketPriceAssessments(snap store.Snapshot, now time.Time) []analyze.PriceAssessment {
	byID := make(map[int64]domain.Player, len(snap.Market)+len(snap.Club.Players))
	for _, player := range snap.Market {
		byID[player.ID] = player
	}
	for _, player := range snap.Club.Players {
		if _, exists := byID[player.ID]; !exists {
			byID[player.ID] = player.Player
		}
	}
	observation := snap.Capabilities["mercado"]
	prices := make([]analyze.PriceAssessment, 0, len(byID))
	for _, player := range byID {
		observedAt := player.Price.UpdatedAt
		if observedAt.IsZero() {
			observedAt = observation.ObservedAt
		}
		source := player.Price.Source
		if source == "" {
			source = observation.Source
		}
		coverage := player.Price.Coverage
		if coverage == 0 {
			coverage = observation.Coverage
		}
		quality := player.Price.Quality
		if quality == "" {
			if observation.Status == futgg.StatusConfirmado {
				quality = "confirmada"
			} else if source == "" {
				quality = "incompleta"
			} else {
				quality = "estimada"
			}
		}
		prices = append(prices, analyze.PriceAssessment{EAID: player.ID, Platform: firstNonEmpty(player.Price.Platform, snap.Club.Platform), Source: source, ObservedAt: observedAt, Coverage: coverage, Quality: quality, Stale: observedAt.IsZero() || now.Sub(observedAt) > priceStaleAfter})
	}
	sort.Slice(prices, func(i, j int) bool { return prices[i].EAID < prices[j].EAID })
	return prices
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (s *Server) handleWatchlistCreate(w http.ResponseWriter, r *http.Request) {
	var entry domain.WatchlistEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "corpo de watchlist inválido: "+err.Error(), http.StatusBadRequest)
		return
	}
	if entry.ID == "" {
		entry.ID = localID("watch")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	entry.UpdatedAt = time.Now()
	if err := s.Store.UpsertWatchlist(r.Context(), s.Cycle, entry); err != nil {
		http.Error(w, "gravando watchlist: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, entry)
}

func (s *Server) handleWatchlistUpdate(w http.ResponseWriter, r *http.Request) {
	var entry domain.WatchlistEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "corpo de watchlist inválido: "+err.Error(), http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if entry.ID != "" && entry.ID != id {
		http.Error(w, "id do corpo difere da rota", http.StatusBadRequest)
		return
	}
	entry.ID = id
	entry.UpdatedAt = time.Now()
	if err := s.Store.UpsertWatchlist(r.Context(), s.Cycle, entry); err != nil {
		http.Error(w, "gravando watchlist: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, entry)
}

func (s *Server) handleWatchlistDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteWatchlist(r.Context(), s.Cycle, r.PathValue("id")); err != nil {
		http.Error(w, "apagando watchlist: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLedgerAppend(w http.ResponseWriter, r *http.Request) {
	var entry domain.LedgerEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "corpo de ledger inválido: "+err.Error(), http.StatusBadRequest)
		return
	}
	if entry.ID == "" {
		entry.ID = localID("ledger")
	}
	if entry.Status == "" {
		entry.Status = domain.LedgerConfirmado
	}
	if entry.RecordedAt.IsZero() {
		entry.RecordedAt = time.Now()
	}
	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = entry.RecordedAt
	}
	if err := s.Store.AppendLedger(r.Context(), s.Cycle, entry); err != nil {
		http.Error(w, "gravando ledger: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, entry)
}

func localID(prefix string) string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + strings.ToLower(hex.EncodeToString(buf))
}
