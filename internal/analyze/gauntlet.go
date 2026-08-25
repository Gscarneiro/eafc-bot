package analyze

import (
	"fmt"
	"math"
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// GauntletRounds é quantas rodadas consecutivas o modo Gauntlet exige — a
// regra oficial pede elenco inteiramente diferente, banco incluso, em cada
// uma (ver EA FC 26 FUT Deep Dive, pitch notes).
const (
	GauntletRounds        = 4
	gauntletStartersCount = 11
	gauntletBenchPerRound = 7
	gauntletTotalCards    = GauntletRounds * (gauntletStartersCount + gauntletBenchPerRound)
)

// GauntletAssignment é uma carta escalada num slot físico do Gauntlet, numa
// rodada específica. Position é a posição do SLOT — pode divergir da
// natural da carta, mesma pegadinha de domain.SquadSlot — e Rating é o GG
// Rating do fut.gg NESSA posição, não necessariamente o "melhor" da carta.
type GauntletAssignment struct {
	Round    int               `json:"round"` // 1..4
	Index    int               `json:"index"` // slot físico (0..10), ordem do fut.gg
	Position domain.Position   `json:"position"`
	Player   domain.ClubPlayer `json:"player"`
	Rating   float64           `json:"rating"`
}

// GauntletSquad é um dos quatro elencos do Gauntlet: 11 titulares + 7
// reservas, sem repetir carta com nenhuma outra rodada.
type GauntletSquad struct {
	Round int `json:"round"` // 1..4

	Starters []GauntletAssignment `json:"starters"`
	Bench    []domain.ClubPlayer  `json:"bench"`

	TotalRating   float64 `json:"total_rating"`
	AverageRating float64 `json:"average_rating"`

	// Quimica é o entrosamento dos 11 titulares desta rodada. Sempre 100% em
	// posição por construção (matchGauntletStarters só cria aresta pra quem
	// GGRatingAt confirma a nota naquele slot) — sob o modelo padrão isso
	// já é 33/33 (ver internal/chemistry), então este campo é sobretudo
	// verificação/transparência, não algo que hoje varie entre rodadas.
	Quimica *chemistry.Resultado `json:"chemistry,omitempty"`
}

// GauntletPlan é o planejamento das quatro rodadas do Gauntlet: 72 cartas
// únicas (44 titulares + 28 reservas), força crescente da rodada 1 para a
// 4 — cálculo determinístico em Go, sem chamada de rede nem de LLM.
type GauntletPlan struct {
	Status    string          `json:"status"` // "ok" | "unavailable"
	Reason    string          `json:"reason,omitempty"`
	Formation string          `json:"formation"`
	Rounds    []GauntletSquad `json:"rounds"`
	Warnings  []string        `json:"warnings,omitempty"`
	Strategy  string          `json:"strategy,omitempty"`
}

// StarterIDs lista, sem repetição, o id (Player.ID) de todo titular
// escalado em qualquer rodada — usado para forçar cards.BuildReports a
// gerar relatório de evolução pra eles mesmo abaixo do corte normal de
// rating (ver CLAUDE.md, seção do Gauntlet).
func (p GauntletPlan) StarterIDs() []int64 {
	ids := make([]int64, 0, GauntletRounds*gauntletStartersCount)
	for _, round := range p.Rounds {
		for _, a := range round.Starters {
			ids = append(ids, a.Player.ID)
		}
	}
	return ids
}

// GauntletOptions governa como BuildGauntletPlan explica o plano. Por ora só
// o modelo de química importa — ver o comentário equivalente em
// SquadOptions.
type GauntletOptions struct {
	ChemistryModel chemistry.Modelo
}

// DefaultGauntletOptions usa o modelo de química padrão.
func DefaultGauntletOptions() GauntletOptions {
	return GauntletOptions{ChemistryModel: chemistry.ModeloPadrao()}
}

// BuildGauntletPlan monta os quatro elencos do Gauntlet com o modelo de
// química padrão. Ver BuildGauntletPlanWithOptions.
func BuildGauntletPlan(club domain.Club) GauntletPlan {
	return BuildGauntletPlanWithOptions(club, DefaultGauntletOptions())
}

// BuildGauntletPlanWithOptions monta os quatro elencos do Gauntlet a partir
// do clube atual, repetindo a formação titular ativa (club.Squad.Starters).
// A prioridade é maximizar a força do conjunto de titulares; reservas só
// cobrem a exigência de banco cheio, com as cartas elegíveis mais fracas
// que sobraram — nunca uma carta que valeria mais como titular.
func BuildGauntletPlanWithOptions(club domain.Club, opt GauntletOptions) GauntletPlan {
	plan := GauntletPlan{Status: "unavailable", Formation: club.Squad.Formation}

	slots := club.Squad.Starters
	if len(slots) != gauntletStartersCount {
		plan.Reason = fmt.Sprintf("escalação titular não sincronizada (tem %d slots, precisa de %d)",
			len(slots), gauntletStartersCount)
		return plan
	}
	for _, s := range slots {
		if _, ok := club.PlayerByID(s.PlayerID); !ok {
			plan.Reason = "titular ausente do retrato do clube"
			return plan
		}
	}

	pool := gauntletPool(club)
	if len(pool) < gauntletTotalCards {
		plan.Reason = fmt.Sprintf(
			"elenco tem %d cartas com GG Rating conhecido, precisa de %d (%d titulares + %d reservas em %d rodadas) para montar o Gauntlet",
			len(pool), gauntletTotalCards, GauntletRounds*gauntletStartersCount, GauntletRounds*gauntletBenchPerRound, GauntletRounds)
		return plan
	}

	assignments, ok := matchGauntletStarters(pool, slots)
	if !ok {
		plan.Reason = "não há jogadores elegíveis o bastante, por posição, para preencher os titulares das 4 rodadas nessa formação"
		return plan
	}

	rounds := distributeRounds(assignments)

	used := make(map[int64]bool, len(assignments))
	for _, a := range assignments {
		used[a.Player.ID] = true
	}
	bench := gauntletBench(pool, used)
	if len(bench) < GauntletRounds*gauntletBenchPerRound {
		plan.Reason = fmt.Sprintf("sobraram %d cartas elegíveis para o banco, precisa de %d",
			len(bench), GauntletRounds*gauntletBenchPerRound)
		return plan
	}
	assignBench(rounds, bench)

	for i := range rounds {
		rounds[i].Quimica = quimicaDaRodada(opt.ChemistryModel, rounds[i].Starters)
	}

	plan.Status = "ok"
	plan.Rounds = rounds
	plan.Strategy = "Titulares escolhidos de uma vez só, por matching global de GG Rating nas 4 rodadas " +
		"(nunca posição por posição isolada), depois distribuídos da rodada mais fraca para a mais forte " +
		"em cada posição — os melhores ficam guardados para a última partida. Reservas usam as cartas " +
		"elegíveis restantes mais fracas, sem tirar lugar de titular nenhum."
	return plan
}

// gauntletPool são as cartas do clube com GG Rating conhecido — a mesma
// régua de FindSquadSwaps para comparar DENTRO do elenco (ver CLAUDE.md,
// "duas notas, dois domínios de uso"). Sem essa nota não dá para comparar a
// carta com as demais, nem como titular nem como reserva.
func gauntletPool(club domain.Club) []domain.ClubPlayer {
	pool := make([]domain.ClubPlayer, 0, len(club.Players))
	for _, p := range club.Players {
		if gauntletValue(p) > 0 {
			pool = append(pool, p)
		}
	}
	return pool
}

// gauntletValue é a melhor nota conhecida da carta, em qualquer posição —
// usa o campo escalar GGRating quando presente (é o que a maioria dos
// snapshots preenche) e cai para o maior valor do mapa GGRatings quando só
// ele existir. Mesma dualidade de fonte que domain.Player.GGRatingAt já
// trata por posição; aqui é o "melhor em qualquer posição", usado só para
// ordenar elegibilidade e o banco, não para escalar ninguém num slot.
func gauntletValue(p domain.ClubPlayer) float64 {
	best := p.GGRating
	for _, v := range p.GGRatings {
		if v > best {
			best = v
		}
	}
	return best
}

// gauntletSlot é uma cópia de um slot físico da formação, repetida uma vez
// por rodada — 44 no total. O matching escolhe, entre as cópias, os
// melhores 44 titulares sem repetir carta; a rodada de cada um é decidida
// depois, em distributeRounds.
type gauntletSlot struct {
	Index    int
	Position domain.Position
}

// matchGauntletStarters reusa o mesmo fluxo de custo mínimo de
// OptimizeSquad (flowEdge/minCostAugment, squad_optimizer.go), só que
// contra 44 slots (11 x 4 rodadas) em vez de 11 — o mesmo argumento vale:
// matching global evita a heurística de escolher posição por posição
// isoladamente, que erraria um jogador elegível em mais de uma posição
// (ex.: CB e CDM) para a posição errada.
//
// Diferente de OptimizeSquad, uma carta só entra numa aresta quando
// GGRatingAt confirma a nota NAQUELA posição — sem isso o jogador ainda
// poderia "jogar lá" (PlaysAt) mas sem nota conhecida, o que viraria uma
// aresta de custo zero e um GG Rating de exibição enganoso (CLAUDE.md: "na
// dúvida, não afirma").
func matchGauntletStarters(pool []domain.ClubPlayer, formation []domain.SquadSlot) ([]GauntletAssignment, bool) {
	slots := make([]gauntletSlot, 0, len(formation)*GauntletRounds)
	for _, s := range formation {
		for r := 0; r < GauntletRounds; r++ {
			slots = append(slots, gauntletSlot{Index: s.Index, Position: s.Position})
		}
	}

	n := 2 + len(pool) + len(slots)
	src, sink := 0, n-1
	g := make([][]flowEdge, n)
	add := func(a, b, cap, cost int) {
		g[a] = append(g[a], flowEdge{to: b, rev: len(g[b]), cap: cap, cost: cost})
		g[b] = append(g[b], flowEdge{to: a, rev: len(g[a]) - 1, cap: 0, cost: -cost})
	}
	for i, p := range pool {
		add(src, 1+i, 1, 0)
		for j, slot := range slots {
			r, ok := p.GGRatingAt(slot.Position)
			if !ok {
				continue
			}
			add(1+i, 1+len(pool)+j, 1, -int(math.Round(r*1000)))
		}
	}
	for j := range slots {
		add(1+len(pool)+j, sink, 1, 0)
	}
	for f := 0; f < len(slots); f++ {
		if !minCostAugment(g, src, sink) {
			return nil, false
		}
	}

	out := make([]GauntletAssignment, 0, len(slots))
	for i, p := range pool {
		for _, e := range g[1+i] {
			if e.to >= 1+len(pool) && e.to < sink && e.cap == 0 {
				j := e.to - (1 + len(pool))
				rating, _ := p.GGRatingAt(slots[j].Position)
				out = append(out, GauntletAssignment{
					Index: slots[j].Index, Position: slots[j].Position, Player: p, Rating: rating,
				})
			}
		}
	}
	return out, true
}

// distributeRounds agrupa as 44 escolhas por slot físico (0..10) e ordena
// cada grupo de 4 pela nota ascendente, dando a rodada 1 à mais fraca e a
// rodada 4 à mais forte. Como cada posição cresce isoladamente, a soma por
// rodada também cresce — ordenar É a troca local ótima aqui: rearranjar
// duas rodadas da mesma posição só pioraria a monotonicidade, nunca
// melhora, então nenhuma busca de troca adicional é necessária.
func distributeRounds(assignments []GauntletAssignment) []GauntletSquad {
	bySlot := map[int][]GauntletAssignment{}
	var order []int
	for _, a := range assignments {
		if _, seen := bySlot[a.Index]; !seen {
			order = append(order, a.Index)
		}
		bySlot[a.Index] = append(bySlot[a.Index], a)
	}
	sort.Ints(order)

	rounds := make([]GauntletSquad, GauntletRounds)
	for i := range rounds {
		rounds[i].Round = i + 1
	}
	for _, slotIndex := range order {
		group := bySlot[slotIndex]
		sort.Slice(group, func(i, j int) bool {
			if group[i].Rating != group[j].Rating {
				return group[i].Rating < group[j].Rating
			}
			return group[i].Player.ID < group[j].Player.ID
		})
		for r := 0; r < GauntletRounds && r < len(group); r++ {
			a := group[r]
			a.Round = r + 1
			rounds[r].Starters = append(rounds[r].Starters, a)
		}
	}
	for i := range rounds {
		sort.Slice(rounds[i].Starters, func(a, b int) bool {
			return rounds[i].Starters[a].Index < rounds[i].Starters[b].Index
		})
		for _, a := range rounds[i].Starters {
			rounds[i].TotalRating += a.Rating
		}
		if n := len(rounds[i].Starters); n > 0 {
			rounds[i].AverageRating = rounds[i].TotalRating / float64(n)
		}
	}
	return rounds
}

// gauntletBench pega, entre as cartas do pool que não viraram titular, as
// mais fracas primeiro: reserva é só cobertura, não deveria consumir uma
// carta que sobrou sem função melhor (ver CLAUDE.md, decisão fechada).
func gauntletBench(pool []domain.ClubPlayer, used map[int64]bool) []domain.ClubPlayer {
	bench := make([]domain.ClubPlayer, 0, len(pool))
	for _, p := range pool {
		if !used[p.ID] {
			bench = append(bench, p)
		}
	}
	sort.Slice(bench, func(i, j int) bool {
		vi, vj := gauntletValue(bench[i]), gauntletValue(bench[j])
		if vi != vj {
			return vi < vj
		}
		return bench[i].ID < bench[j].ID
	})
	return bench
}

// assignBench reparte as reservas mais fracas em blocos de 7, na mesma
// ordem crescente usada para os titulares — sem exigência de força mínima
// por rodada, já que o banco é só cobertura (ver o comentário de
// gauntletBench).
func assignBench(rounds []GauntletSquad, bench []domain.ClubPlayer) {
	for i := range rounds {
		start := i * gauntletBenchPerRound
		end := start + gauntletBenchPerRound
		rounds[i].Bench = append([]domain.ClubPlayer(nil), bench[start:end]...)
	}
}

// quimicaDaRodada converte os titulares da rodada para o formato que
// internal/chemistry entende.
func quimicaDaRodada(m chemistry.Modelo, starters []GauntletAssignment) *chemistry.Resultado {
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
