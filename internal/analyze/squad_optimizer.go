package analyze

import (
	"math"
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// SquadPlan é a recomendação consultiva de escalação do clube inteiro.
type SquadPlan struct {
	Status           string              `json:"status"`
	Reason           string              `json:"reason,omitempty"`
	CurrentAverage   float64             `json:"current_average"`
	SuggestedAverage float64             `json:"suggested_average"`
	Gain             float64             `json:"gain"`
	CurrentTotal     float64             `json:"current_total"`
	SuggestedTotal   float64             `json:"suggested_total"`
	Starters         []SquadAssignment   `json:"starters"`
	Moves            []SquadMove         `json:"moves"`
	Alternatives     []SquadAlternatives `json:"alternatives"`
}

type SquadAssignment struct {
	Index    int               `json:"index"`
	Position domain.Position   `json:"position"`
	Player   domain.ClubPlayer `json:"player"`
	Rating   float64           `json:"rating"`
}
type SquadMove struct {
	Index           int               `json:"index"`
	Position        domain.Position   `json:"position"`
	Current         domain.ClubPlayer `json:"current"`
	Suggested       domain.ClubPlayer `json:"suggested"`
	CurrentRating   float64           `json:"current_rating"`
	SuggestedRating float64           `json:"suggested_rating"`
	Gain            float64           `json:"gain"`
}
type SquadAlternatives struct {
	Index    int               `json:"index"`
	Position domain.Position   `json:"position"`
	Players  []SquadAssignment `json:"players"`
}

// OptimizeSquad encontra o melhor matching global entre jogadores e slots.
// O fluxo de custo mínimo evita a heurística de escolher cada posição isoladamente.
func OptimizeSquad(club domain.Club) SquadPlan {
	plan := SquadPlan{Status: "unavailable"}
	slots := club.Squad.Starters
	if len(slots) == 0 {
		plan.Reason = "escalação titular não sincronizada"
		return plan
	}
	for _, s := range slots {
		if p, ok := club.PlayerByID(s.PlayerID); !ok {
			plan.Reason = "titular ausente do retrato do clube"
			return plan
		} else if _, ok := p.GGRatingAt(s.Position); !ok {
			plan.Reason = "faltam notas GG Rating por posição; nova coleta necessária"
			return plan
		}
	}
	// Grafo bipartido: fonte -> carta -> slot -> destino.
	n := 2 + len(club.Players) + len(slots)
	src, sink := 0, n-1
	g := make([][]flowEdge, n)
	add := func(a, b, cap, cost int) {
		g[a] = append(g[a], flowEdge{to: b, rev: len(g[b]), cap: cap, cost: cost})
		g[b] = append(g[b], flowEdge{to: a, rev: len(g[a]) - 1, cap: 0, cost: -cost})
	}
	for i, p := range club.Players {
		add(src, 1+i, 1, 0)
		for j, s := range slots {
			if p.PlaysAt(s.Position) {
				r, _ := p.GGRatingAt(s.Position)
				bonus := 0
				if p.ID == s.PlayerID {
					// Um centésimo não evita micro-trocas: aqui o bônus equivale
					// a 0,1 GG, a tolerância mínima da recomendação.
					bonus = 100
				}
				add(1+i, 1+len(club.Players)+j, 1, -int(math.Round(r*1000))-bonus)
			}
		}
	}
	for j := range slots {
		add(1+len(club.Players)+j, sink, 1, 0)
	}
	for f := 0; f < len(slots); f++ {
		if !minCostAugment(g, src, sink) {
			plan.Reason = "não há jogadores elegíveis para todos os slots"
			return plan
		}
	}
	chosen := make([]domain.ClubPlayer, len(slots))
	for i := range club.Players {
		for _, e := range g[1+i] {
			if e.to >= 1+len(club.Players) && e.to < sink && e.cap == 0 {
				chosen[e.to-(1+len(club.Players))] = club.Players[i]
			}
		}
	}
	plan.Starters = make([]SquadAssignment, 0, len(slots))
	plan.Alternatives = make([]SquadAlternatives, 0, len(slots))
	for j, s := range slots {
		r, _ := chosen[j].GGRatingAt(s.Position)
		cr, _ := club.PlayerByID(s.PlayerID)
		cur, _ := cr.GGRatingAt(s.Position)
		plan.CurrentTotal += cur
		plan.SuggestedTotal += r
		plan.Starters = append(plan.Starters, SquadAssignment{s.Index, s.Position, chosen[j], r})
		if chosen[j].ID != s.PlayerID {
			plan.Moves = append(plan.Moves, SquadMove{s.Index, s.Position, cr, chosen[j], cur, r, r - cur})
		}
	}
	plan.CurrentAverage = plan.CurrentTotal / float64(len(slots))
	plan.SuggestedAverage = plan.SuggestedTotal / float64(len(slots))
	plan.Gain = plan.SuggestedAverage - plan.CurrentAverage
	if plan.Gain >= 0.1 {
		plan.Status = "improved"
	} else {
		plan.Status = "optimal"
	}
	used := map[int64]bool{}
	for _, a := range plan.Starters {
		used[a.Player.ID] = true
	}
	for _, s := range slots {
		var cs []SquadAssignment
		for _, p := range club.Players {
			if used[p.ID] || !p.PlaysAt(s.Position) {
				continue
			}
			if r, ok := p.GGRatingAt(s.Position); ok {
				cs = append(cs, SquadAssignment{s.Index, s.Position, p, r})
			}
		}
		sort.Slice(cs, func(a, b int) bool {
			if cs[a].Rating != cs[b].Rating {
				return cs[a].Rating > cs[b].Rating
			}
			return cs[a].Player.ID < cs[b].Player.ID
		})
		if len(cs) > 3 {
			cs = cs[:3]
		}
		plan.Alternatives = append(plan.Alternatives, SquadAlternatives{s.Index, s.Position, cs})
	}
	return plan
}

type flowEdge struct{ to, rev, cap, cost int }

func minCostAugment(g [][]flowEdge, s, t int) bool {
	n := len(g)
	dist := make([]int, n)
	prevV := make([]int, n)
	prevE := make([]int, n)
	for i := range dist {
		dist[i] = int(^uint(0) >> 1)
	}
	dist[s] = 0
	for k := 0; k < n; k++ {
		changed := false
		for v := 0; v < n; v++ {
			if dist[v] == int(^uint(0)>>1) {
				continue
			}
			for ei, e := range g[v] {
				if e.cap > 0 && dist[e.to] > dist[v]+e.cost {
					dist[e.to] = dist[v] + e.cost
					prevV[e.to] = v
					prevE[e.to] = ei
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	if dist[t] == int(^uint(0)>>1) {
		return false
	}
	for v := t; v != s; v = prevV[v] {
		e := &g[prevV[v]][prevE[v]]
		e.cap--
		g[v][e.rev].cap++
	}
	return true
}
