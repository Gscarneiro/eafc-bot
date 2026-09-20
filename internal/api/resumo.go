package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/report"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// ResumoResponse é o envelope leve que a topbar e o rail (App.tsx) leem em
// TODA navegação — nunca o snapshot inteiro nem os feeds pesados de
// StatusResponse (NewCards/News/SBCs/Objectives/History). Os deltas usam só
// os dois últimos pontos de SnapshotHistory, não a série de 30 dias.
type ResumoResponse struct {
	GeneratedAt time.Time `json:"generated_at"`
	Cycle       string    `json:"cycle"`

	Coins           int            `json:"coins"`
	CoinsDelta      int            `json:"coins_delta,omitempty"`
	Capital         domain.Capital `json:"capital"`
	SquadScore      float64        `json:"squad_score"`
	SquadScoreDelta float64        `json:"squad_score_delta,omitempty"`

	WeakestSlot     domain.Position `json:"weakest_slot"`
	WeakestName     string          `json:"weakest_name"`
	WeakestGGRating float64         `json:"weakest_gg_rating"`

	Quimica *chemistry.Resultado `json:"chemistry,omitempty"`

	// TrocasViaveis é quantos upgrades de mercado cabem no bolso AGORA
	// (Affordable && !Unpriced) — o selo "Oportunidades · N" do rail e o KPI
	// "trocas viáveis" do modo terminal de Hoje usam o mesmo número.
	TrocasViaveis int `json:"trocas_viaveis"`

	// AnaliseEntraNoXI/CatalogoElegiveis/Salvos são os selos de Evoluções no
	// rail — mesma fonte que as próprias telas usam (evolution_paths.go /
	// evolution_catalog.go), só resumida a um inteiro.
	AnaliseEntraNoXI     int `json:"analise_entra_no_xi"`
	CatalogoElegiveis    int `json:"catalogo_elegiveis"`
	Salvos               int `json:"salvos"`
	GalleryOpportunities int `json:"gallery_opportunities"`

	Avisos []Aviso `json:"avisos"`

	// Ticker é o "Mercado" corrido do modo terminal de Hoje — embrulha
	// report.MarketRows (que já existe pro briefing HTML) com tag JSON,
	// já que report.MarketRow não tem tag de propósito (struct de
	// html/template, ver CLAUDE.md).
	Ticker []TickerRow `json:"ticker"`
}

type TickerRow struct {
	Name  string           `json:"name"`
	Role  string           `json:"role"`
	Trend store.PriceTrend `json:"trend"`
}

func (s *Server) handleResumo(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}

	capital := snap.Capital
	if capital == (domain.Capital{}) {
		capital = snap.Club.Capital(s.EvolutionExtraBudget, s.MarketReserve, 0)
	}
	avg, weakSlot, weakName, weakGG := report.SquadSummaryWithEvaluator(snap.Club, s.resolveEvaluator(), snap.Avaliacao, contextosAtuaisDasVagas(snap))

	var scoreDelta float64
	var coinsDelta int
	if hist, err := s.snapshotHistory(r.Context(), 2); err == nil && len(hist) >= 2 {
		scoreDelta = hist[len(hist)-1].SquadScore - hist[len(hist)-2].SquadScore
		coinsDelta = hist[len(hist)-1].Coins - hist[len(hist)-2].Coins
	}

	chem := s.currentChemistry(snap)

	trocas := 0
	for _, u := range snap.Upgrades {
		if u.Affordable && !u.Unpriced {
			trocas++
		}
	}

	analiseEntraNoXI := evolutionPathsSummary(s.buildEvolutionPlayerAnalysesAtuais(snap)).EntraNoXI
	catalogoElegiveis := summarizeEvolutionCatalog(buildEvolutionCatalog(snap)).Eligible
	salvos := 0
	if backend, isSaved := s.Store.(store.SavedEvolutionPathStore); isSaved {
		if saved, err := backend.ListSavedEvolutionPaths(r.Context(), s.Cycle); err == nil {
			salvos = len(saved)
		}
	}

	galleryOpportunities := 0
	if gs, ok := s.Store.(store.GaleriaStore); ok {
		if rows, err := gs.ListGallery(r.Context(), s.Cycle, snap.Club.GamerTag, snap.Club.Platform); err == nil {
			for _, row := range rows {
				if row.Notify() {
					galleryOpportunities++
				}
			}
		}
	}

	agenda, err := s.buildAgenda(r.Context(), snap)
	if err != nil {
		agenda = analyze.Agenda{}
	}

	marketRows := report.MarketRows(snap.Club, snap.Upgrades, snap.Trends)
	ticker := make([]TickerRow, len(marketRows))
	for i, row := range marketRows {
		ticker[i] = TickerRow{Name: row.Name, Role: row.Role, Trend: row.Trend}
	}
	avisos := buildAvisos(snap, chem, agenda)
	if gs, ok := s.Store.(store.GaleriaStore); ok {
		if rows, err := gs.ListGallery(r.Context(), s.Cycle, snap.Club.GamerTag, snap.Club.Platform); err == nil {
			for _, row := range rows {
				if !row.Notify() {
					continue
				}
				avisos = append(avisos, Aviso{Kind: "galeria", Severity: "alerta", Headline: fmt.Sprintf("Gallery: %s pode chegar a %s", row.Set.Name, row.Evaluation.Grade), Detail: fmt.Sprintf("%d pontos · %d/%d cartas", row.Evaluation.Score, row.Evaluation.Filled, row.Evaluation.Required), Link: "/galeria/" + row.Set.ID})
			}
		}
	}

	writeJSON(w, ResumoResponse{
		GeneratedAt:          snap.GeneratedAt,
		Cycle:                snap.Cycle,
		Coins:                snap.Club.Coins,
		CoinsDelta:           coinsDelta,
		Capital:              capital,
		SquadScore:           avg,
		SquadScoreDelta:      scoreDelta,
		WeakestSlot:          weakSlot,
		WeakestName:          weakName,
		WeakestGGRating:      weakGG,
		Quimica:              chem,
		TrocasViaveis:        trocas,
		AnaliseEntraNoXI:     analiseEntraNoXI,
		CatalogoElegiveis:    catalogoElegiveis,
		Salvos:               salvos,
		GalleryOpportunities: galleryOpportunities,
		Avisos:               avisos,
		Ticker:               ticker,
	})
}
