package analyze

import (
	"fmt"
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// SellCandidate é uma carta do BANCO (fora do XI titular) com uma
// recomendação do que fazer com ela.
type SellCandidate struct {
	Player domain.ClubPlayer `json:"player"`
	// Recommendation: "vender" | "segurar_potencial" | "aguardar_verificacao" |
	// "promover" | "nao_vendavel".
	Recommendation string `json:"recommendation"`
	// NetSellValue é o líquido de verdade, já descontando os 5% de taxa de
	// venda da EA. O mesmo método é usado por Upgrade.Recoup e Club.Budget.
	// Zero
	// quando Recommendation é "promover" ou "nao_vendavel".
	NetSellValue int `json:"net_sell_value,omitempty"`
	// EvoGGGain/EvoCost só vêm preenchidos quando Recommendation é
	// "segurar_potencial" — o ganho e o custo do MELHOR caminho de
	// evolução disponível (cards.CardReport.Best, calculado pelo fut.gg).
	EvoGGGain float64  `json:"evo_gg_gain,omitempty"`
	EvoCost   int      `json:"evo_cost,omitempty"`
	Rationale []string `json:"rationale"`
}

// SellOptions controla o piso de potencial que justifica segurar em vez
// de vender.
type SellOptions struct {
	// MinEvoGGGain é o ganho mínimo de GG Rating (Best.GGRatingGain) pra
	// valer segurar em vez de vender — abaixo disso, o potencial existe
	// mas é pequeno demais pra travar moeda parada no banco.
	MinEvoGGGain float64
}

// DefaultSellOptions são padrões conservadores.
func DefaultSellOptions() SellOptions {
	return SellOptions{MinEvoGGGain: 2.0}
}

// SellFunnel conta, carta a carta do banco, qual recomendação cada uma
// recebeu — mesmo padrão de UpgradeFunnel/InvestmentFunnel.
type SellFunnel struct {
	Considered          int `json:"considered"` // banco: club.Players menos Squad.Starters
	NotTradeable        int `json:"not_tradeable"`
	Promotable          int `json:"promotable"`
	HeldForPotential    int `json:"held_for_potential"`
	WaitingVerification int `json:"waiting_verification"`
	Suggested           int `json:"suggested"` // "vender"

	MinEvoGGGain float64 `json:"min_evo_gg_gain"`
}

// FindSellCandidates varre o BANCO (fora do XI titular — ver
// domain.Club.Squad.Starters, não InSquad, que é mais amplo) e recomenda
// o que fazer com cada carta:
//
//   - "promover": já bate um titular na mesma posição (ver FindSquadSwaps)
//     — a jogada certa é escalar, não vender.
//   - "nao_vendavel": Untradeable — SellValue() já devolve 0 pra essas.
//     Recomendar a venda do que não pode ser vendido é pior que não dizer
//     nada (pesquisa de mercado); a carta ainda é útil como fodder de SBC
//     sem custo de oportunidade, é só não vendável no mercado.
//   - "segurar_potencial": cards.CardReport.Best (projeção REAL do fut.gg
//     via evolução, não fórmula própria) ganha GG Rating suficiente pra
//     valer esperar.
//   - "vender": nenhum dos motivos acima — líquida, sem uso à vista.
func FindSellCandidates(club domain.Club, cardReports []cards.CardReport, swaps []SquadSwap, opt SellOptions) ([]SellCandidate, SellFunnel) {
	starterIDs := make(map[int64]bool, len(club.Squad.Starters))
	for _, s := range club.Squad.Starters {
		starterIDs[s.PlayerID] = true
	}
	promotable := make(map[int64]bool, len(swaps))
	for _, s := range swaps {
		promotable[s.Candidate.ID] = true
	}
	bestByID := make(map[int64]*cards.EvoPotential, len(cardReports))
	evoStatusByID := make(map[int64]cards.EvolutionStatus, len(cardReports))
	analyzed := make(map[int64]bool, len(cardReports))
	for _, r := range cardReports {
		bestByID[r.Player.ID] = r.Best
		evoStatusByID[r.Player.ID] = r.EvolutionStatus
		analyzed[r.Player.ID] = true
	}

	funnel := SellFunnel{MinEvoGGGain: opt.MinEvoGGGain}

	var out []SellCandidate
	for _, p := range club.Players {
		if starterIDs[p.ID] {
			continue // titular, não é banco
		}
		funnel.Considered++

		if promotable[p.ID] {
			funnel.Promotable++
			out = append(out, SellCandidate{
				Player: p, Recommendation: "promover",
				Rationale: []string{"já bate um titular na mesma posição (GG Rating maior) — considere escalar em vez de vender"},
			})
			continue
		}

		if p.Untradeable {
			funnel.NotTradeable++
			out = append(out, SellCandidate{
				Player: p, Recommendation: "nao_vendavel",
				Rationale: []string{"untradeable — não vendável no mercado, mas serve como fodder de SBC sem custo de oportunidade"},
			})
			continue
		}

		net := p.NetSellValue()
		if status := evoStatusByID[p.ID]; status == cards.EvolutionFetchError || status == cards.EvolutionNotChecked {
			funnel.WaitingVerification++
			out = append(out, SellCandidate{
				Player: p, Recommendation: "aguardar_verificacao",
				Rationale: []string{"a evolução não foi verificada; aguarde uma coleta bem-sucedida antes de vender"},
			})
			continue
		}

		if best := bestByID[p.ID]; best != nil && best.GGRatingGain >= opt.MinEvoGGGain {
			funnel.HeldForPotential++
			out = append(out, SellCandidate{
				Player: p, Recommendation: "segurar_potencial", NetSellValue: net,
				EvoGGGain: best.GGRatingGain, EvoCost: best.CoinsCost,
				Rationale: []string{fmt.Sprintf("evolução disponível ganha +%.1f GG Rating", best.GGRatingGain)},
			})
			continue
		}

		funnel.Suggested++
		rationale := []string{"sem uso no XI titular"}
		if !analyzed[p.ID] {
			rationale = append(rationale, "sem análise de evolução disponível (abaixo do piso de overall analisado — ver serve.cards_min_rating)")
		}
		out = append(out, SellCandidate{
			Player: p, Recommendation: "vender", NetSellValue: net,
			Rationale: rationale,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].NetSellValue > out[j].NetSellValue })
	return out, funnel
}
