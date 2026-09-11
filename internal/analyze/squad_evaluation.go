package analyze

import (
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// SquadSlotEvaluation é a leitura de uma vaga física na régua ativa. A
// avaliação completa acompanha a nota para a UI explicar cobertura e limites.
type SquadSlotEvaluation struct {
	Index      int                   `json:"index"`
	Position   domain.Position       `json:"position"`
	Player     domain.ClubPlayer     `json:"player"`
	Evaluation domain.AvaliacaoCarta `json:"evaluation"`
}

// EvaluateSquadSlots avalia o XI sem gravar notas contextuais dentro das
// cartas. Isso preserva duas vagas de mesma posição com funções distintas.
func EvaluateSquadSlots(club domain.Club, evaluator Avaliador, base domain.ContextoAvaliacao, porVaga map[int]domain.ContextoAvaliacao) []SquadSlotEvaluation {
	out := make([]SquadSlotEvaluation, 0, len(club.Squad.Starters))
	for _, slot := range club.Squad.Starters {
		player, ok := club.PlayerForSlot(slot)
		if !ok {
			continue
		}
		evaluation := avaliarNaReguaDoElenco(player.Player, slot.Position, evaluator, contextoDaVaga(base, porVaga, slot))
		out = append(out, SquadSlotEvaluation{Index: slot.Index, Position: slot.Position, Player: player, Evaluation: evaluation})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

type WeakLink struct {
	Index    int               `json:"index"`
	Slot     domain.Position   `json:"slot"`
	Player   domain.ClubPlayer `json:"player"`
	Score    float64           `json:"score"`
	GapToAvg float64           `json:"gap_to_avg"`
}

// WeakestLinksFromEvaluations calcula média e elo fraco sobre as mesmas
// avaliações já usadas para desenhar o campo.
func WeakestLinksFromEvaluations(evaluations []SquadSlotEvaluation, n int) []WeakLink {
	all := make([]WeakLink, 0, len(evaluations))
	var sum float64
	for _, item := range evaluations {
		if !item.Evaluation.Disponivel {
			continue
		}
		all = append(all, WeakLink{Index: item.Index, Slot: item.Position, Player: item.Player, Score: item.Evaluation.Nota})
		sum += item.Evaluation.Nota
	}
	if len(all) == 0 {
		return nil
	}
	average := sum / float64(len(all))
	for i := range all {
		all[i].GapToAvg = all[i].Score - average
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Score != all[j].Score {
			return all[i].Score < all[j].Score
		}
		return all[i].Index < all[j].Index
	})
	if n >= 0 && len(all) > n {
		all = all[:n]
	}
	return all
}
