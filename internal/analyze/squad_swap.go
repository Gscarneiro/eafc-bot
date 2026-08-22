package analyze

import (
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// SquadSwap é uma troca sem custo: um titular sai, entra alguém que você JÁ
// TEM no elenco, na mesma posição, com GG Rating maior. Não é opinião deste
// bot — é o número que o próprio fut.gg calculou pra cada carta.
type SquadSwap struct {
	Slot        domain.Position   `json:"slot"`
	Current     domain.ClubPlayer `json:"current"`
	Candidate   domain.ClubPlayer `json:"candidate"`
	GGRatingGap float64           `json:"gg_rating_gap"`
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
	starterIDs := make(map[int64]bool, len(club.Squad.Starters))
	for _, s := range club.Squad.Starters {
		starterIDs[s.PlayerID] = true
	}

	var out []SquadSwap
	for _, slot := range club.Squad.Starters {
		current, ok := club.PlayerByID(slot.PlayerID)
		if !ok || current.GGRating <= 0 {
			continue // sem GG Rating do titular, não há com o que comparar
		}

		var best domain.ClubPlayer
		bestGap := 0.0
		for _, cand := range club.Players {
			if starterIDs[cand.ID] || cand.GGRating <= 0 {
				continue
			}
			if !cand.PlaysAt(slot.Position) {
				continue
			}
			if gap := cand.GGRating - current.GGRating; gap > bestGap {
				best, bestGap = cand, gap
			}
		}
		if bestGap > 0 {
			out = append(out, SquadSwap{
				Slot: slot.Position, Current: current, Candidate: best, GGRatingGap: bestGap,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].GGRatingGap > out[j].GGRatingGap })
	return out
}
