package analyze

import (
	"math"
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/chemistry"
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

	// CurrentQuimica é o entrosamento do XI que está de pé HOJE (pode ter
	// alguém fora de posição — é a única forma de perder entrosamento sob o
	// modelo padrão, ver internal/chemistry). Quimica é o da escalação
	// SUGERIDA (Starters), sempre 100% em posição por construção (o fluxo só
	// cria aresta pra quem PlaysAt o slot) — nil quando não há XI válido
	// para avaliar (Status == "unavailable").
	CurrentQuimica *chemistry.Resultado `json:"current_chemistry,omitempty"`
	Quimica        *chemistry.Resultado `json:"chemistry,omitempty"`
}

// SquadOptions governa como OptimizeSquad explica a sugestão. Por ora só o
// modelo de química importa: ele decide como CurrentQuimica/Quimica são
// calculados. Pesar o vínculo CONTRA o GG Rating na escolha em si (a
// otimização de verdade) ainda não existe — hoje o resultado do fluxo de
// custo mínimo não muda com estas opções.
type SquadOptions struct {
	ChemistryModel chemistry.Modelo
}

// DefaultSquadOptions usa o modelo de química padrão.
func DefaultSquadOptions() SquadOptions {
	return SquadOptions{ChemistryModel: chemistry.ModeloPadrao()}
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

// OptimizeSquad encontra o melhor matching global entre jogadores e slots,
// com o modelo de química padrão. Ver OptimizeSquadWithOptions.
func OptimizeSquad(club domain.Club) SquadPlan {
	return OptimizeSquadWithOptions(club, DefaultSquadOptions())
}

// OptimizeSquadWithOptions encontra o melhor matching global entre jogadores
// e slots. O fluxo de custo mínimo evita a heurística de escolher cada
// posição isoladamente.
func OptimizeSquadWithOptions(club domain.Club, opt SquadOptions) SquadPlan {
	plan := SquadPlan{Status: "unavailable"}
	slots := club.Squad.Starters
	if len(slots) == 0 {
		plan.Reason = "escalação titular não sincronizada"
		return plan
	}
	// A química do XI ATUAL não depende de GG Rating nenhum — só de
	// posição/clube/liga/nação, que o retrato do clube sempre traz quando
	// há slots. Calculado aqui, antes das checagens abaixo (que são só
	// pré-requisito da SUGESTÃO), para não desaparecer só porque o fluxo de
	// GG Rating não pôde rodar (ex.: dado sem GGRatingAt preenchido).
	plan.CurrentQuimica = chemistry.Avaliar(opt.ChemistryModel, club)
	for _, s := range slots {
		if p, ok := club.PlayerByID(s.PlayerID); !ok {
			plan.Reason = "titular ausente do retrato do clube"
			return plan
		} else if _, ok := p.GGRatingAt(s.Position); !ok {
			plan.Reason = "faltam notas GG Rating por posição; nova coleta necessária"
			return plan
		}
	}
	chosen, ok := squadMatch(club.Players, slots)
	if !ok {
		plan.Reason = "não há jogadores elegíveis para todos os slots"
		return plan
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
	// Por JOGADOR, não por carta: sugerir "troque por Mbappé TOTS" quando o
	// Mbappé ouro já é titular noutro slot seria uma troca ilegal — o jogo não
	// aceita duas versões do mesmo atleta no mesmo elenco.
	used := map[string]bool{}
	for _, a := range plan.Starters {
		used[a.Player.PlayerKey()] = true
	}
	for _, s := range slots {
		var cs []SquadAssignment
		for _, p := range club.Players {
			if used[p.PlayerKey()] || !p.PlaysAt(s.Position) {
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

	plan.Quimica = quimicaDaSugestao(opt.ChemistryModel, plan.Starters)
	return plan
}

// squadMatch roda o fluxo de custo mínimo de sempre sobre um pool de cartas
// e uma lista de slots, e devolve a carta escolhida para cada slot (na mesma
// ordem de `slots`) — ou false quando não dá pra cobrir todos. É o motor de
// escalação em si, compartilhado por OptimizeSquad (players = club.Players
// inteiro, slots = club.Squad.Starters) e pelo Squad Planner
// (squad_planner.go), que pode filtrar o pool por exclusão e usar uma
// formação diferente — a mesma regra de "não reescrever o matching" que
// vale para o Gauntlet (ver gauntlet_rules.go).
//
// Grafo: fonte -> JOGADOR -> carta -> slot -> destino. O nó de jogador é o
// que impede a sugestão de escalar duas versões do mesmo atleta (o jogo não
// aceita duas cartas do mesmo jogador no mesmo elenco); o nó de carta
// continua garantindo um slot por carta. Ver domain.Player.PlayerKey.
func squadMatch(players []domain.ClubPlayer, slots []domain.SquadSlot) ([]domain.ClubPlayer, bool) {
	keys := map[string]int{}
	for _, p := range players {
		if _, seen := keys[p.PlayerKey()]; !seen {
			keys[p.PlayerKey()] = len(keys)
		}
	}
	playerNode := func(i int) int { return 1 + i }
	cardNode := func(i int) int { return 1 + len(keys) + i }
	slotNode := func(j int) int { return 1 + len(keys) + len(players) + j }
	n := 2 + len(keys) + len(players) + len(slots)
	src, sink := 0, n-1
	g := make([][]flowEdge, n)
	add := func(a, b, cap, cost int) {
		g[a] = append(g[a], flowEdge{to: b, rev: len(g[b]), cap: cap, cost: cost})
		g[b] = append(g[b], flowEdge{to: a, rev: len(g[a]) - 1, cap: 0, cost: -cost})
	}
	// Por índice, não por range no mapa: ordem de aresta muda o desempate do
	// Bellman-Ford, e uma sugestão que mudasse a cada execução com o mesmo
	// elenco seria impossível de conferir.
	for k := 0; k < len(keys); k++ {
		add(src, playerNode(k), 1, 0)
	}
	for i, p := range players {
		add(playerNode(keys[p.PlayerKey()]), cardNode(i), 1, 0)
		for j, s := range slots {
			if p.PlaysAt(s.Position) {
				r, _ := p.GGRatingAt(s.Position)
				bonus := 0
				if p.ID == s.PlayerID {
					// Um centésimo não evita micro-trocas: aqui o bônus equivale
					// a 0,1 GG, a tolerância mínima da recomendação.
					bonus = 100
				}
				add(cardNode(i), slotNode(j), 1, -int(math.Round(r*1000))-bonus)
			}
		}
	}
	for j := range slots {
		add(slotNode(j), sink, 1, 0)
	}
	for f := 0; f < len(slots); f++ {
		if !minCostAugment(g, src, sink) {
			return nil, false
		}
	}
	chosen := make([]domain.ClubPlayer, len(slots))
	for i := range players {
		for _, e := range g[cardNode(i)] {
			if e.to >= slotNode(0) && e.to < sink && e.cap == 0 {
				chosen[e.to-slotNode(0)] = players[i]
			}
		}
	}
	return chosen, true
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

// quimicaDaSugestao converte a escalação sugerida para o formato que
// internal/chemistry entende. nil quando não há sugestão (a chamadora só
// preenche Starters nos caminhos de sucesso).
func quimicaDaSugestao(m chemistry.Modelo, starters []SquadAssignment) *chemistry.Resultado {
	if len(starters) == 0 {
		return nil
	}
	xi := make([]chemistry.Titular, len(starters))
	for i, a := range starters {
		xi[i] = chemistry.Titular{Index: a.Index, Position: a.Position, Player: a.Player.Player}
	}
	res := chemistry.Calcular(m, xi)
	return &res
}
