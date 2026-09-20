package api

import (
	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// LeituraDoBot é a "leitura" de uma linha do banco (Meu time) — o Kind
// escolhe qual modelo de frase a tela monta; os números ficam crus (sem
// separador de milhar, sem casa decimal fixa) porque formatar é trabalho de
// web/src/format.ts, não deste pacote — mesmo padrão de TopMove.NetCost e
// de todo o resto da API. A decisão em si (qual Kind) já foi tomada por
// analyze.FindSellCandidates e analyze.FodderMatches; esta função só
// escolhe QUAL delas mostrar quando mais de uma se aplica, por prioridade
// de ação: dinheiro parado perdendo valor primeiro, depois evolução,
// depois o resto.
type LeituraDoBot struct {
	Kind string `json:"kind"` // "evoluir" | "vender_caindo" | "vender" | "promover" | "fodder" | "nao_vendavel" | "aguardar_verificacao" | ""
	// FinalGGRating/CoinsCost só valem para Kind=="evoluir".
	FinalGGRating float64 `json:"final_gg_rating,omitempty"`
	CoinsCost     int     `json:"coins_cost,omitempty"`
	// ChangePct30d só vale para Kind=="vender_caindo".
	ChangePct30d float64 `json:"change_pct_30d,omitempty"`
	// SBCName só vale para Kind=="fodder" ou "nao_vendavel" quando há match
	// de analyze.FodderMatches — nunca para "min_team_rating"/
	// "min_rarity_count" (ver o comentário de analyze.FodderMatch).
	SBCName string `json:"sbc_name,omitempty"`
	// Promocao só vem quando Kind=="promover" e a troca pode ser ligada a
	// uma cópia física do banco. Ela expõe a vaga e as duas notas que o motor
	// comparou; sem isso, "bate um titular" não é uma recomendação auditável.
	Promocao *PromocaoDoBanco `json:"promocao,omitempty"`
}

type PromocaoDoBanco struct {
	SlotIndex     int             `json:"slot_index"`
	Posicao       domain.Position `json:"position"`
	Titular       string          `json:"starter_name"`
	NotaTitular   float64         `json:"starter_rating"`
	NotaCandidato float64         `json:"candidate_rating"`
	Ganho         float64         `json:"gain"`
	Metrica       string          `json:"metric,omitempty"` // "gg_rating_card"; vazio para outro avaliador
}

func leituraDoBot(sell analyze.SellCandidate, trend store.PriceTrend, hasTrend bool, matches []analyze.FodderMatch, swap *analyze.SquadSwap) LeituraDoBot {
	switch sell.Recommendation {
	case "segurar_potencial":
		return LeituraDoBot{Kind: "evoluir", FinalGGRating: sell.Player.GGRating + sell.EvoGGGain, CoinsCost: sell.EvoCost}
	case "vender":
		if hasTrend && trend.ChangePct < 0 {
			return LeituraDoBot{Kind: "vender_caindo", ChangePct30d: trend.ChangePct}
		}
		return LeituraDoBot{Kind: "vender"}
	case "promover":
		// Uma recomendação legada não volta a promover metarank pela leitura
		// de um snapshot anterior à retirada dessa referência.
		if swap != nil && (metricaAvaliacao(swap.CurrentEvaluation) == "metarank" || metricaAvaliacao(swap.CandidateEvaluation) == "metarank") {
			return LeituraDoBot{}
		}
		leitura := LeituraDoBot{Kind: "promover"}
		if swap != nil {
			leitura.Promocao = &PromocaoDoBanco{
				SlotIndex:     swap.Index,
				Posicao:       swap.Slot,
				Titular:       swap.Current.Display(),
				NotaTitular:   swap.CurrentRating,
				NotaCandidato: swap.CandidateRating,
				Ganho:         swap.GGRatingGap,
				Metrica:       metricaPromocao(swap),
			}
		}
		return leitura
	case "nao_vendavel":
		if len(matches) > 0 {
			return LeituraDoBot{Kind: "nao_vendavel", SBCName: matches[0].SBCName}
		}
		return LeituraDoBot{Kind: "nao_vendavel"}
	case "aguardar_verificacao":
		return LeituraDoBot{Kind: "aguardar_verificacao"}
	}
	if len(matches) > 0 {
		return LeituraDoBot{Kind: "fodder", SBCName: matches[0].SBCName}
	}
	return LeituraDoBot{}
}

func metricaPromocao(swap *analyze.SquadSwap) string {
	atual := metricaAvaliacao(swap.CurrentEvaluation)
	if atual == "" || atual != metricaAvaliacao(swap.CandidateEvaluation) {
		return ""
	}
	return atual
}

func metricaAvaliacao(avaliacao domain.AvaliacaoCarta) string {
	for _, componente := range avaliacao.Componentes {
		switch componente.Chave {
		case "gg_rating_carta":
			return "gg_rating_card"
		case "metarank_score":
			return "metarank"
		}
	}
	return ""
}
