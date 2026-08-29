package analyze

import (
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestPlanMarketRespeitaCapitalGlobalEPrecoAntigo(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	upgrades := []Upgrade{
		{Slot: domain.CM, Candidate: domain.Player{ID: 1, Name: "Primeira"}, GrossCost: 7_000, NetCost: 7_000},
		{Slot: domain.CB, Candidate: domain.Player{ID: 2, Name: "Segunda"}, GrossCost: 7_000, NetCost: 7_000},
		{Slot: domain.ST, Candidate: domain.Player{ID: 3, Name: "Antiga"}, GrossCost: 1_000, NetCost: 1_000},
	}
	plan := PlanMarket(MarketPlanInput{
		Capital: domain.Capital{Available: 10_000}, Upgrades: upgrades, GeneratedAt: now,
		Prices: map[int64]PriceAssessment{
			1: {EAID: 1, ObservedAt: now, Quality: "confirmada"},
			2: {EAID: 2, ObservedAt: now, Quality: "confirmada"},
			3: {EAID: 3, ObservedAt: now.Add(-48 * time.Hour), Quality: "confirmada", Stale: true},
		},
	})
	byName := map[string]MarketAction{}
	for _, action := range plan.Actions {
		byName[action.Name] = action
	}
	if byName["Primeira"].Kind != MarketBuy {
		t.Fatalf("Primeira = %+v, esperava compra", byName["Primeira"])
	}
	if byName["Segunda"].Kind != MarketWait {
		t.Fatalf("Segunda = %+v, esperava espera por capital", byName["Segunda"])
	}
	if byName["Antiga"].Kind != MarketWait || byName["Antiga"].Confidence != "baixa" {
		t.Fatalf("Antiga = %+v, esperava cotação stale", byName["Antiga"])
	}
}

func TestPlanMarketNaoVendeCartaProtegidaParaEvolucao(t *testing.T) {
	player := domain.ClubPlayer{Player: domain.Player{ID: 4, Name: "Protegida", Price: domain.Price{Coins: 10_000}}}
	plan := PlanMarket(MarketPlanInput{
		Capital:    domain.Capital{Available: 20_000},
		Sells:      []SellCandidate{{Player: player, Recommendation: "vender", NetSellValue: 9_500}},
		Evolutions: []EvoMatch{{Player: player, Slot: domain.CM}},
		Watchlist:  []domain.WatchlistEntry{{ID: "w", EAID: 4, Protected: true}},
	})
	if len(plan.Conflicts) == 0 || len(plan.Actions) == 0 || plan.Actions[0].Kind != MarketWait {
		t.Fatalf("esperava venda bloqueada e conflito, veio %+v", plan)
	}
}

func TestPlanMarketResumeEvolucoesPorCartaEIgnoraVendaSemCotacao(t *testing.T) {
	semCotacao := domain.ClubPlayer{Player: domain.Player{ID: 1, Name: "Sem cotação"}}
	candidata := domain.ClubPlayer{Player: domain.Player{ID: 2, Name: "Candidata", Price: domain.Price{Coins: 10_000}}}
	plan := PlanMarket(MarketPlanInput{
		Sells: []SellCandidate{
			{Player: semCotacao, Recommendation: "vender"},
			{Player: candidata, Recommendation: "vender", NetSellValue: 9_500},
		},
		Evolutions: []EvoMatch{
			{Evolution: domain.Evolution{ID: "evo-menor", Name: "Evo menor"}, Player: candidata, Slot: domain.CM, Gain: 2},
			{Evolution: domain.Evolution{ID: "evo-melhor", Name: "Evo melhor"}, Player: candidata, Slot: domain.CM, Gain: 5},
		},
	})

	if len(plan.Actions) != 1 {
		t.Fatalf("ações=%+v; esperava somente a melhor evolução acionável", plan.Actions)
	}
	if got := plan.Actions[0]; got.Name != "Evo melhor · Candidata" || got.Kind != MarketObserve {
		t.Fatalf("ação=%+v; esperava a melhor evolução da carta", got)
	}
}
