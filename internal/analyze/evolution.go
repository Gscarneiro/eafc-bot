package analyze

import (
	"sort"
	"strings"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// EvoMatch é uma evolução que cabe num jogador seu, já com a projeção
// de como a carta fica no fim.
type EvoMatch struct {
	Evolution    domain.Evolution  `json:"evolution"`
	Player       domain.ClubPlayer `json:"player"`
	Slot         domain.Position   `json:"slot"` // função onde o ganho foi medido
	Before       float64           `json:"before"`
	After        float64           `json:"after"`
	Gain         float64           `json:"gain"`
	Result       domain.Player     `json:"result"` // a carta projetada
	Cost         int               `json:"cost"`
	BeatsStarter bool              `json:"beats_starter"` // vira titular na posição?
	Highlights   []string          `json:"highlights"`
}

// FindEvolutions cruza as evoluções ativas com o seu elenco e devolve só
// as combinações que melhoram alguma coisa de verdade.
func FindEvolutions(club domain.Club, evos []domain.Evolution, budget int) []EvoMatch {
	var out []EvoMatch

	for _, evo := range evos {
		if evo.CoinCost > budget {
			continue
		}
		for _, cp := range club.Players {
			if !cp.Evolvable() {
				continue
			}
			if !Eligible(cp.Player, evo) {
				continue
			}

			result := evo.Apply(cp.Player)

			// Mede o ganho na melhor função possível para o jogador,
			// já considerando posições que a evolução destrava.
			bestSlot, before, after := bestSlotGain(cp.Player, result)
			gain := after - before
			if gain <= 0.5 {
				continue
			}

			m := EvoMatch{
				Evolution:  evo,
				Player:     cp,
				Slot:       bestSlot,
				Before:     before,
				After:      after,
				Gain:       gain,
				Result:     result,
				Cost:       evo.CoinCost,
				Highlights: evoHighlights(cp.Player, result, evo),
			}
			// A evolução vale muito mais se a carta final passa o titular atual.
			if starter, ok := club.Starter(bestSlot); ok {
				m.BeatsStarter = after > Score(starter.Player, bestSlot) && starter.ID != cp.ID
			}
			out = append(out, m)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].BeatsStarter != out[j].BeatsStarter {
			return out[i].BeatsStarter
		}
		return out[i].Gain > out[j].Gain
	})
	return out
}

// bestSlotGain procura em que posição a carta evoluída rende mais.
func bestSlotGain(before, after domain.Player) (domain.Position, float64, float64) {
	var bestSlot domain.Position
	var bestBefore, bestAfter float64
	first := true

	// Considera a posição natural, as alternativas atuais e as que a
	// evolução acrescenta.
	slots := map[domain.Position]bool{after.Position: true}
	for _, p := range after.AltPositions {
		slots[p] = true
	}

	for slot := range slots {
		a := Score(after, slot)
		b := Score(before, slot)
		if first || a > bestAfter {
			bestSlot, bestBefore, bestAfter, first = slot, b, a, false
		}
	}
	return bestSlot, bestBefore, bestAfter
}

// evoHighlights resume em texto o que a evolução muda na carta.
func evoHighlights(before, after domain.Player, evo domain.Evolution) []string {
	var out []string
	if d := after.Rating - before.Rating; d > 0 {
		out = append(out, "overall "+itoa(before.Rating)+" -> "+itoa(after.Rating))
	}
	type ad struct {
		label string
		d     int
	}
	pairs := []ad{
		{"Ritmo", after.Attributes.Pace - before.Attributes.Pace},
		{"Chute", after.Attributes.Shooting - before.Attributes.Shooting},
		{"Passe", after.Attributes.Passing - before.Attributes.Passing},
		{"Drible", after.Attributes.Dribbling - before.Attributes.Dribbling},
		{"Defesa", after.Attributes.Defending - before.Attributes.Defending},
		{"Físico", after.Attributes.Physical - before.Attributes.Physical},
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].d > pairs[j].d })
	for _, p := range pairs {
		if p.d > 0 {
			out = append(out, p.label+" +"+itoa(p.d))
		}
	}

	// PlayStyles novos.
	had := map[string]bool{}
	for _, ps := range before.PlayStyles {
		had[ps.String()] = true
	}
	for _, ps := range after.PlayStyles {
		if !had[ps.String()] {
			out = append(out, "novo: "+ps.String())
		}
	}
	// Posições destravadas.
	hadPos := map[domain.Position]bool{before.Position: true}
	for _, p := range before.AltPositions {
		hadPos[p] = true
	}
	for _, p := range after.AltPositions {
		if !hadPos[p] {
			out = append(out, "destrava "+string(p))
		}
	}
	if len(out) > 6 {
		out = out[:6]
	}
	return out
}

// Eligible avalia se um jogador satisfaz TODOS os requisitos da evolução.
// Requisito que o parser não entendeu é tratado como bloqueante — é melhor
// o bot deixar de sugerir do que sugerir uma evolução impossível.
func Eligible(p domain.Player, evo domain.Evolution) bool {
	for _, req := range evo.Requirements {
		if !meets(p, req) {
			return false
		}
	}
	return true
}

func meets(p domain.Player, req domain.EvoRequirement) bool {
	switch req.Kind {
	case "max_overall":
		return p.Rating <= req.IntValue
	case "min_overall":
		return p.Rating >= req.IntValue
	case "max_pace":
		return p.Attributes.Pace <= req.IntValue
	case "max_shooting":
		return p.Attributes.Shooting <= req.IntValue
	case "max_passing":
		return p.Attributes.Passing <= req.IntValue
	case "max_dribbling":
		return p.Attributes.Dribbling <= req.IntValue
	case "max_defending":
		return p.Attributes.Defending <= req.IntValue
	case "max_physical":
		return p.Attributes.Physical <= req.IntValue
	case "max_skill_moves":
		return p.SkillMoves <= req.IntValue
	case "max_weak_foot":
		return p.WeakFoot <= req.IntValue
	case "min_skill_moves":
		return p.SkillMoves >= req.IntValue
	case "max_playstyles":
		return len(p.PlayStyles) <= req.IntValue
	case "max_playstyles_plus":
		n := 0
		for _, ps := range p.PlayStyles {
			if ps.Plus {
				n++
			}
		}
		return n <= req.IntValue
	case "min_playstyles_plus":
		n := 0
		for _, ps := range p.PlayStyles {
			if ps.Plus {
				n++
			}
		}
		return n >= req.IntValue
	case "position":
		for _, want := range req.Strings {
			if p.PlaysAt(domain.Position(strings.ToUpper(want))) {
				return true
			}
		}
		return false
	case "excluded_position":
		// Ao contrário de "position": aqui a carta precisa NÃO jogar em
		// nenhuma das posições listadas — um único casamento já reprova.
		for _, avoid := range req.Strings {
			if p.PlaysAt(domain.Position(strings.ToUpper(avoid))) {
				return false
			}
		}
		return true
	case "rarity":
		for _, want := range req.Strings {
			if strings.EqualFold(p.Version, want) {
				return true
			}
		}
		return false
	case "league":
		for _, want := range req.Strings {
			if strings.EqualFold(p.League, want) {
				return true
			}
		}
		return false
	case "nation":
		for _, want := range req.Strings {
			if strings.EqualFold(p.Nation, want) {
				return true
			}
		}
		return false
	case "club":
		for _, want := range req.Strings {
			if strings.EqualFold(p.Club, want) {
				return true
			}
		}
		return false
	}
	// Requisito desconhecido: não arrisca.
	return false
}
