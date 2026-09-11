package analyze

import (
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// SquadSwap é uma troca sem custo: um titular sai, entra alguém que você JÁ
// TEM no elenco, na mesma posição, com GG Rating maior. Não é opinião deste
// bot — é o número que o próprio fut.gg calculou pra cada carta.
type SquadSwap struct {
	// Index é o SLOT FÍSICO (Squad.Starters[i].Index), não a posição lógica
	// — uma formação repete posição (dois CB), e "CB" sozinho não diz qual
	// dos dois esta troca é sobre. Zero-valor é o sentinela de snapshot
	// gravado antes deste campo existir; quem lê cai para casar por posição
	// só quando existir exatamente um slot daquela posição (ver
	// internal/api, mapa de posições).
	Index     int               `json:"index"`
	Slot      domain.Position   `json:"slot"`
	Current   domain.ClubPlayer `json:"current"`
	Candidate domain.ClubPlayer `json:"candidate"`
	// CurrentRating e CandidateRating são as notas publicadas PARA Slot.
	// GGRating pode ser a melhor nota da carta em outra posição e por isso
	// nunca deve ser usada para explicar ou ordenar esta troca.
	CurrentRating       float64               `json:"current_rating"`
	CandidateRating     float64               `json:"candidate_rating"`
	GGRatingGap         float64               `json:"gg_rating_gap"`
	CurrentEvaluation   domain.AvaliacaoCarta `json:"current_evaluation,omitempty"`
	CandidateEvaluation domain.AvaliacaoCarta `json:"candidate_evaluation,omitempty"`
}

// FindSquadSwaps varre o BANCO — não o mercado — atrás de reforço. Pra cada
// titular, procura entre os jogadores que sobraram do elenco (quem não está
// nos 11) o de maior GG Rating que joga na mesma posição, e sugere a troca
// se ele for melhor que o titular atual.
//
// Existe porque comparar GG Rating não é uma conta que este bot inventa:
// ranquear cartas pelo Score() próprio (roles.go, pesos por função + bônus
// de PlayStyle) é uma opinião DESTE bot sobre o que importa em campo, e faz
// sentido pra decidir COMPRA no mercado, onde não existe um número oficial
// pra comparar. Mas dentro do seu elenco, o fut.gg já publicou uma nota por
// carta — usar ela é mais direto, e é a nota que você já confere no site.
func FindSquadSwaps(club domain.Club) []SquadSwap {
	return FindSquadSwapsWithOptions(club, SquadSwapOptions{})
}

// FindSquadSwapsWithEvaluator aplica uma única régua antes de comparar banco
// e titulares. O wrapper histórico acima preserva relatórios gravados.
func FindSquadSwapsWithEvaluator(club domain.Club, evaluator Avaliador, ctx domain.ContextoAvaliacao) []SquadSwap {
	return FindSquadSwapsWithOptions(club, SquadSwapOptions{Evaluator: evaluator, Contexto: ctx})
}

// SquadSwapOptions conserva o contexto da vaga física. Duas posições CM na
// mesma formação podem ter funções e estilos de entrosamento diferentes.
type SquadSwapOptions struct {
	Evaluator        Avaliador
	Contexto         domain.ContextoAvaliacao
	ContextosPorVaga map[int]domain.ContextoAvaliacao
}

// FindSquadSwapsWithOptions compara cada reserva com o ocupante exato da
// vaga. A avaliação acontece dentro do laço do slot: projetar o clube inteiro
// antes perderia a diferença entre dois lugares de mesma posição lógica.
func FindSquadSwapsWithOptions(club domain.Club, options SquadSwapOptions) []SquadSwap {
	starterCards := make(map[string]bool, len(club.Squad.Starters))
	for _, s := range club.Squad.Starters {
		if p, ok := club.PlayerForSlot(s); ok {
			starterCards[p.IdentityKey()] = true
		}
	}

	var out []SquadSwap
	for _, slot := range club.Squad.Starters {
		current, ok := club.PlayerForSlot(slot)
		if !ok {
			continue
		}
		contexto := contextoDaVaga(options.Contexto, options.ContextosPorVaga, slot)
		currentEvaluation := avaliarNaReguaDoElenco(current.Player, slot.Position, options.Evaluator, contexto)
		if !currentEvaluation.Disponivel {
			continue // sem nota publicada PARA a vaga, não há comparação segura
		}
		currentRating := currentEvaluation.Nota

		var best domain.ClubPlayer
		var bestEvaluation domain.AvaliacaoCarta
		bestRating := 0.0
		bestGap := 0.0
		for _, cand := range club.Players {
			if starterCards[cand.IdentityKey()] || cand.AlvoPlano {
				continue
			}
			if !cand.PlaysAt(slot.Position) {
				continue
			}
			candidateEvaluation := avaliarNaReguaDoElenco(cand.Player, slot.Position, options.Evaluator, contexto)
			if !candidateEvaluation.Disponivel {
				continue // sem inferir a nota de uma vaga ausente
			}
			candidateRating := candidateEvaluation.Nota
			if gap := candidateRating - currentRating; gap > bestGap {
				best, bestEvaluation, bestRating, bestGap = cand, candidateEvaluation, candidateRating, gap
			}
		}
		if bestGap > 0 {
			out = append(out, SquadSwap{
				Index: slot.Index, Slot: slot.Position, Current: current, Candidate: best,
				CurrentRating: currentRating, CandidateRating: bestRating, GGRatingGap: bestGap,
				CurrentEvaluation: currentEvaluation, CandidateEvaluation: bestEvaluation,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].GGRatingGap > out[j].GGRatingGap })
	return out
}

func contextoDaVaga(base domain.ContextoAvaliacao, porVaga map[int]domain.ContextoAvaliacao, slot domain.SquadSlot) domain.ContextoAvaliacao {
	ctx := base
	if especifico, ok := porVaga[slot.Index]; ok {
		ctx = especifico
	}
	ctx.Posicao = slot.Position
	return ctx
}

// avaliarNaReguaDoElenco mantém o wrapper sem avaliador compatível com o
// contrato histórico do elenco: nil significa GG Rating, e nunca Score().
func avaliarNaReguaDoElenco(card domain.Player, pos domain.Position, evaluator Avaliador, ctx domain.ContextoAvaliacao) domain.AvaliacaoCarta {
	ctx.Posicao = pos
	if evaluator != nil {
		return evaluator.Avaliar(card, pos, ctx)
	}
	nota, ok := card.GGRatingAt(pos)
	if !ok {
		return domain.AvaliacaoCarta{Contexto: ctx, Motivo: "GG Rating ausente para esta vaga"}
	}
	ctx.Fonte = domain.FonteFutGG
	return domain.AvaliacaoCarta{Disponivel: true, Nota: nota, Contexto: ctx,
		Cobertura:   []string{"nota posicional do FUT.GG"},
		Componentes: []domain.ComponenteAvaliacao{{Chave: "gg_rating", Rotulo: "GG Rating posicional", Valor: nota}},
	}
}
