package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/query"
	"github.com/gscarneiro/eafc-bot/internal/report"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// serveList concentra a borda HTTP do subconjunto OData. O motor permanece
// agnóstico à rede e a resposta de erro já informa os campos aceitos para a
// próxima tentativa.
func serveList[T any](w http.ResponseWriter, r *http.Request, schema query.Schema[T], items []T) (query.Page[T], bool) {
	options, err := query.Parse(r.URL.Query())
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "details": err})
		return query.Page[T]{}, false
	}
	page, err := query.Apply(schema, items, options)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "details": err})
		return query.Page[T]{}, false
	}
	return page, true
}

type mercadoCollectionResponse struct {
	query.Page[analyze.Upgrade]
	Funnel      analyze.UpgradeFunnel        `json:"@eafc.funnel"`
	PriceSeries map[int64][]store.PricePoint `json:"@eafc.price_series"`
	// Campos antigos ficam por uma versão para clientes locais que ainda não
	// migraram. O contrato novo sempre usa value e @eafc.*.
	Upgrades []analyze.Upgrade `json:"upgrades,omitempty"`
}

type evolucoesCollectionResponse struct {
	query.Page[EvoMatchView]
	Summary EvolucoesSummary `json:"@eafc.summary"`
	// Compatibilidade com a tela anterior, removida depois que todos os
	// clientes locais passarem a usar value.
	Matches    []EvoMatchView `json:"matches,omitempty"`
	Total      int            `json:"total,omitempty"`
	LegacyPage int            `json:"page,omitempty"`
	PageSize   int            `json:"page_size,omitempty"`
	Pages      int            `json:"pages,omitempty"`
}

type collectionResponse[T any] struct {
	query.Page[T]
}

type startersCollectionResponse struct {
	query.Page[StarterCard]
	Formation string `json:"@eafc.formation"`
}

type reservasCollectionResponse struct {
	query.Page[RosterCard]
	MinimumRating int `json:"@eafc.minimum_rating"`
}

type investimentosCollectionResponse struct {
	query.Page[analyze.Investment]
	Funnel analyze.InvestmentFunnel `json:"@eafc.funnel"`
}

type vendasCollectionResponse struct {
	query.Page[analyze.SellCandidate]
	Funnel analyze.SellFunnel `json:"@eafc.funnel"`
}

type sbcsCollectionResponse struct {
	query.Page[analyze.FodderSignal]
}

type movimentoCard struct {
	RosterCard
	Movimento string `json:"movimento"`
}

type movimentoCollectionResponse struct {
	query.Page[movimentoCard]
}

type evolucaoDetailCollectionResponse struct {
	query.Page[EvoMatchView]
}

func (s *Server) handleMercado(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	page, ok := serveList(w, r, mercadoSchema(), snap.Upgrades)
	if !ok {
		return
	}
	ids := make([]int64, 0, len(page.Value))
	for _, upgrade := range page.Value {
		ids = append(ids, upgrade.Candidate.ID)
	}
	series, err := s.Store.PriceSeries(r.Context(), s.Cycle, ids, priceSeriesWindow)
	if err != nil {
		series = nil
	}
	writeJSON(w, mercadoCollectionResponse{Page: page, Funnel: snap.MarketFunnel, PriceSeries: series, Upgrades: page.Value})
}

func (s *Server) handleEvolucoes(w http.ResponseWriter, r *http.Request) {
	// A compatibilidade é deliberadamente estreita: só é ativada quando um
	// cliente antigo ainda manda os parâmetros legados. Uma URL nova, mesmo
	// sem filtro, recebe exclusivamente a coleção OData.
	// A tela atual ainda mantém os nomes legados na URL para preservar links
	// salvos, mas manda também o contrato OData. O envelope OData tem
	// precedência quando qualquer parâmetro com "$" está presente; só uma
	// URL exclusivamente legada cai no adaptador antigo.
	if hasLegacyEvolutionQuery(r) && !hasODataEvolutionQuery(r) {
		s.handleEvolucoesLegacy(w, r)
		return
	}
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	views := confirmedEvoViews(snap, s.EvolutionMinRating, s.EvolutionExtraBudget, s.MarketReserve)
	page, ok := serveList(w, r, evolucoesSchema(), views)
	if !ok {
		return
	}
	writeJSON(w, evolucoesCollectionResponse{
		Page:       page,
		Summary:    evolutionSummary(page.Count, page.Value),
		Matches:    page.Value,
		Total:      page.Count,
		LegacyPage: page.Skip/page.Top + 1,
		PageSize:   page.Top,
		Pages:      pageCount(page.Count, page.Top),
	})
}

func hasLegacyEvolutionQuery(r *http.Request) bool {
	q := r.URL.Query()
	for _, key := range []string{"page", "page_size", "position", "impact", "category", "q", "status", "expiring", "sort"} {
		if q.Get(key) != "" {
			return true
		}
	}
	return false
}

func hasODataEvolutionQuery(r *http.Request) bool {
	q := r.URL.Query()
	for _, key := range []string{"$filter", "$search", "$orderby", "$top", "$skip", "$count"} {
		if _, ok := q[key]; ok {
			return true
		}
	}
	return false
}

func evolutionSummary(total int, views []EvoMatchView) EvolucoesSummary {
	summary := EvolucoesSummary{Matches: total, ByAcquisition: map[string]int{}}
	players := map[int64]bool{}
	for _, view := range views {
		players[view.Player.ID] = true
		if view.BeatsStarter {
			summary.Starters++
		}
		if !view.Affordable {
			summary.Unaffordable++
		}
		if !view.Evolution.ExpiresAt.IsZero() && !view.Evolution.ExpiresAt.Before(time.Now()) && !view.Evolution.ExpiresAt.After(time.Now().Add(7*24*time.Hour)) {
			summary.ExpiringSoon++
		}
		summary.ByAcquisition[view.Acquisition]++
	}
	summary.Players = len(players)
	return summary
}

func pageCount(total, top int) int {
	if total == 0 || top <= 0 {
		return 0
	}
	return (total + top - 1) / top
}

func (s *Server) handleTitulares(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	quimicaPorCarta := chemistryByPlayer(s.currentChemistry(snap))
	starters, formation := buildStarters(snap, quimicaPorCarta)
	page, ok := serveList(w, r, startersSchema(), starters)
	if !ok {
		return
	}
	writeJSON(w, startersCollectionResponse{Page: page, Formation: formation})
}

func (s *Server) handleReservas(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	starters := map[int64]bool{}
	for _, slot := range snap.Club.Squad.Starters {
		starters[slot.PlayerID] = true
	}
	players := filteredBench(snap.Club.Players, starters, httptestRequestWithoutLegacyFilters(r))
	rows := make([]RosterCard, 0, len(players))
	lookup := newCardSlugLookup(snap.Cards)
	for _, player := range players {
		rows = append(rows, RosterCard{Player: player, CardSlug: lookup.slug(player)})
	}
	page, ok := serveList(w, r, reservasSchema(), rows)
	if !ok {
		return
	}
	writeJSON(w, reservasCollectionResponse{Page: page, MinimumRating: benchMinimumRating})
}

// filteredBench recebe a requisição para manter a compatibilidade da função
// antiga, mas as rotas novas deixam a busca, posição e ordenação para Apply.
func httptestRequestWithoutLegacyFilters(r *http.Request) *http.Request {
	clone := r.Clone(r.Context())
	clone.URL.RawQuery = ""
	return clone
}

type cardSlugLookup struct {
	byItem map[string]cards.CardReport
	byID   map[int64][]cards.CardReport
}

func newCardSlugLookup(reports []cards.CardReport) cardSlugLookup {
	lookup := cardSlugLookup{byItem: make(map[string]cards.CardReport), byID: make(map[int64][]cards.CardReport)}
	for _, report := range reports {
		if report.Player.ClubItemID != "" {
			lookup.byItem[report.Player.ClubItemID] = report
		}
		lookup.byID[report.Player.ID] = append(lookup.byID[report.Player.ID], report)
	}
	return lookup
}

// slug resolve primeiro a cópia física; quando a fonte não provou essa
// identidade, só usa EA ID se houver uma única carta. Assim uma cópia
// duplicada nunca abre silenciosamente o detalhe de outra.
func (lookup cardSlugLookup) slug(player domain.ClubPlayer) string {
	if player.ClubItemID != "" {
		if report, ok := lookup.byItem[player.ClubItemID]; ok {
			return report.Slug
		}
	}
	candidates := lookup.byID[player.ID]
	if len(candidates) == 1 {
		return candidates[0].Slug
	}
	return ""
}

func (lookup cardSlugLookup) report(player domain.ClubPlayer) (cards.CardReport, bool) {
	if player.ClubItemID != "" {
		if report, ok := lookup.byItem[player.ClubItemID]; ok {
			return report, true
		}
	}
	candidates := lookup.byID[player.ID]
	if len(candidates) == 1 {
		return candidates[0], true
	}
	return cards.CardReport{}, false
}

func buildStarters(snap store.Snapshot, quimicaPorCarta map[int64]chemistry.Jogador) ([]StarterCard, string) {
	lookup := newCardSlugLookup(snap.Cards)
	main := reportMainSquad(snap.Club)
	starters := make([]StarterCard, 0, len(main))
	for _, card := range main {
		rating, _ := card.Player.GGRatingAt(card.Position)
		sc := StarterCard{RosterCard: RosterCard{Player: card.Player, CardSlug: lookup.slug(card.Player)}, Index: card.Index, Position: card.Position, PositionGGRating: rating}
		if j, ok := quimicaPorCarta[card.Player.ID]; ok {
			sc.Quimica = &j
		}
		starters = append(starters, sc)
	}
	formation := snap.Club.Squad.Formation
	if formation == "" {
		formation = inferFormation(snap.Club.Squad.Starters)
	}
	return starters, formation
}

// reportMainSquad é um alias local que documenta que o endpoint usa o XI
// físico, não Club.Starter(pos), que perderia posições repetidas.
func reportMainSquad(club domain.Club) []reportSquadCard {
	main := report.MainSquad(club)
	result := make([]reportSquadCard, len(main))
	for i, card := range main {
		result[i] = reportSquadCard{Index: card.Index, Position: card.Position, Player: card.Player}
	}
	return result
}

type reportSquadCard struct {
	Index    int
	Position domain.Position
	Player   domain.ClubPlayer
}

func (s *Server) handleCapitalInvestimentos(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	momentum, err := s.Store.LatestMomentum(r.Context(), s.Cycle)
	if err != nil {
		momentum = nil
	}
	items, funnel := analyze.FindInvestments(snap.Club, momentum, snap.NewCards, analyze.DefaultInvestmentOptions())
	page, ok := serveList(w, r, investimentosSchema(), items)
	if !ok {
		return
	}
	writeJSON(w, investimentosCollectionResponse{Page: page, Funnel: funnel})
}

func (s *Server) handleCapitalVendas(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	items, funnel := analyze.FindSellCandidates(snap.Club, snap.Cards, snap.SquadSwaps, analyze.DefaultSellOptions())
	page, ok := serveList(w, r, vendasSchema(), items)
	if !ok {
		return
	}
	writeJSON(w, vendasCollectionResponse{Page: page, Funnel: funnel})
}

func fodderSignals(ctx context.Context, s *Server, snap store.Snapshot) []analyze.FodderSignal {
	keys := make([]string, 0, len(snap.SBCs))
	for _, sbc := range snap.SBCs {
		for index, challenge := range sbc.Challenges {
			keys = append(keys, store.SBCChallengeKey(sbc.ID, index, challenge.Name))
		}
	}
	raw, err := s.Store.SBCCostTrend(ctx, s.Cycle, keys, priceSeriesWindow)
	if err != nil {
		raw = nil
	}
	trends := make(map[string]analyze.CostTrend, len(raw))
	for key, trend := range raw {
		trends[key] = analyze.CostTrend{ChangePct: trend.ChangePct, Samples: trend.Samples}
	}
	return analyze.FindFodderDemand(snap.SBCs, snap.Market, trends, analyze.DefaultFodderDemandOptions())
}

func (s *Server) handleCapitalSBCs(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	page, ok := serveList(w, r, fodderSchema(), fodderSignals(r.Context(), s, snap))
	if !ok {
		return
	}
	writeJSON(w, sbcsCollectionResponse{Page: page})
}

func (s *Server) handleHojeNovidades(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	page, ok := serveList(w, r, newCardsSchema(), snap.NewCards)
	if ok {
		writeJSON(w, collectionResponse[domain.Player]{Page: page})
	}
}

func (s *Server) handleHojeNoticias(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	page, ok := serveList(w, r, newsSchema(), snap.FreshNews)
	if ok {
		writeJSON(w, collectionResponse[domain.NewsItem]{Page: page})
	}
}

func (s *Server) handleHojeSBCs(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	sbcs := append([]domain.SBC(nil), snap.SBCs...)
	objectives := append([]domain.Objective(nil), snap.Objectives...)
	active, _ := report.RankChallenges(sbcs, objectives)
	page, ok := serveList(w, r, activeSBCSchema(), active)
	if ok {
		writeJSON(w, collectionResponse[domain.SBC]{Page: page})
	}
}

func (s *Server) handleHojeObjetivos(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	page, ok := serveList(w, r, objectivesSchema(), snap.Objectives)
	if ok {
		writeJSON(w, collectionResponse[domain.Objective]{Page: page})
	}
}

func (s *Server) handleHojeMovimentacao(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	lookup := newCardSlugLookup(snap.Cards)
	items := make([]movimentoCard, 0, len(snap.Diff.Added)+len(snap.Diff.Removed))
	for _, player := range snap.Diff.Added {
		items = append(items, movimentoCard{RosterCard: RosterCard{Player: player, CardSlug: lookup.slug(player)}, Movimento: "entrou"})
	}
	for _, player := range snap.Diff.Removed {
		items = append(items, movimentoCard{RosterCard: RosterCard{Player: player, CardSlug: lookup.slug(player)}, Movimento: "saiu"})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Movimento != items[j].Movimento {
			return items[i].Movimento < items[j].Movimento
		}
		return strings.ToLower(items[i].Player.Display()) < strings.ToLower(items[j].Player.Display())
	})
	page, ok := serveList(w, r, movimentoSchema(), items)
	if ok {
		writeJSON(w, movimentoCollectionResponse{Page: page})
	}
}

func (s *Server) handleHistorico(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.load(w, r); !ok {
		return
	}
	items, err := s.Store.SnapshotHistory(r.Context(), s.Cycle, s.History)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	page, ok := serveList(w, r, historySchema(), items)
	if ok {
		writeJSON(w, collectionResponse[store.SnapshotSummary]{Page: page})
	}
}
