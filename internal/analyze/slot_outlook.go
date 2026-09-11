package analyze

import "github.com/gscarneiro/eafc-bot/internal/domain"

// SlotOutlookKind é a anotação do "Mapa de posições" (Meu time) — o que
// existe além do titular atual pra aquele slot físico, na MESMA escala de
// GG Rating que a barra usa. Nunca a escala de Score() aqui — ver CLAUDE.md
// "duas notas, dois domínios de uso: não misture".
type SlotOutlookKind string

const (
	// SlotOutlookMelhorDisponivel: tem alguém no BANCO com GG Rating maior
	// nesta posição (o mesmo SquadSwap de custo zero).
	SlotOutlookMelhorDisponivel SlotOutlookKind = "melhor_disponivel"
	// SlotOutlookSemCotacao: o mercado tem alvo pra esta posição, mas sem
	// preço confirmado — não dá pra comparar em GG Rating por custo.
	SlotOutlookSemCotacao SlotOutlookKind = "sem_cotacao"
	// SlotOutlookTeto: nem o banco nem o mercado (com cotação) superam o
	// titular — não é "o melhor jogador do mundo", é "nada bateu o piso
	// desta coleta".
	SlotOutlookTeto SlotOutlookKind = "teto"
	// SlotOutlookSemDado: sem GG Rating do titular nesta posição pra
	// comparar com nada — na dúvida, não afirma.
	SlotOutlookSemDado SlotOutlookKind = "sem_dado"
)

type SlotOutlook struct {
	Index    int             `json:"index"`
	Position domain.Position `json:"position"`
	Kind     SlotOutlookKind `json:"kind"`
	// Delta só é preenchido em SlotOutlookMelhorDisponivel, e é sempre GG
	// Rating (SquadSwap.GGRatingGap) — nunca analyze.Upgrade.Gain, que está
	// na escala de Score().
	Delta float64 `json:"delta,omitempty"`
}

// BuildSlotOutlooks cruza swaps (o banco, já calculado por FindSquadSwaps)
// e upgrades (o mercado, já calculado por FindUpgrades) por SLOT FÍSICO —
// swaps já carrega Index; upgrades só carrega Position, então "sem cotação"
// vale pra todo slot daquela posição, não um slot específico.
func BuildSlotOutlooks(club domain.Club, swaps []SquadSwap, upgrades []Upgrade) []SlotOutlook {
	return BuildSlotOutlooksWithEvaluations(club, swaps, upgrades, EvaluateSquadSlots(club, nil, domain.ContextoAvaliacao{}, nil))
}

// BuildSlotOutlooksWithEvaluations recebe a mesma leitura que alimenta as
// barras do campo; assim, uma lacuna na nota ativa não vira "teto" só porque
// outra fonte tinha GG Rating gravado na carta.
func BuildSlotOutlooksWithEvaluations(club domain.Club, swaps []SquadSwap, upgrades []Upgrade, evaluations []SquadSlotEvaluation) []SlotOutlook {
	bySlot := make(map[int]SquadSwap, len(swaps))
	for _, s := range swaps {
		bySlot[s.Index] = s
	}
	unpricedByPosition := make(map[domain.Position]bool)
	for _, u := range upgrades {
		if u.Unpriced {
			unpricedByPosition[u.Slot] = true
		}
	}
	evaluationBySlot := make(map[int]domain.AvaliacaoCarta, len(evaluations))
	for _, item := range evaluations {
		evaluationBySlot[item.Index] = item.Evaluation
	}

	out := make([]SlotOutlook, 0, len(club.Squad.Starters))
	for _, slot := range club.Squad.Starters {
		_, hasPlayer := club.PlayerForSlot(slot)
		evaluation, hasEvaluation := evaluationBySlot[slot.Index]
		switch {
		case !hasPlayer || !hasEvaluation || !evaluation.Disponivel:
			out = append(out, SlotOutlook{Index: slot.Index, Position: slot.Position, Kind: SlotOutlookSemDado})
		case bySlot[slot.Index].GGRatingGap > 0:
			out = append(out, SlotOutlook{Index: slot.Index, Position: slot.Position, Kind: SlotOutlookMelhorDisponivel, Delta: bySlot[slot.Index].GGRatingGap})
		case unpricedByPosition[slot.Position]:
			out = append(out, SlotOutlook{Index: slot.Index, Position: slot.Position, Kind: SlotOutlookSemCotacao})
		default:
			out = append(out, SlotOutlook{Index: slot.Index, Position: slot.Position, Kind: SlotOutlookTeto})
		}
	}
	return out
}
