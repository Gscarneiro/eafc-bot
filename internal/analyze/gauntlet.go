package analyze

import (
	"fmt"
	"math"
	"sort"
	"strings"

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
	seen := map[int64]bool{}
	for _, round := range p.Rounds {
		for _, a := range round.Starters {
			// Duas CÓPIAS da mesma carta podem ser titulares em rodadas
			// diferentes (é normal ter duas no clube) — aqui o que importa é o
			// conjunto de cartas a relatar, não quantas vezes cada uma jogou.
			if seen[a.Player.ID] {
				continue
			}
			seen[a.Player.ID] = true
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
//
// É um atalho para BuildGauntletPlanFromRequest com DefaultGauntletRequest
// (4 rodadas, estratégia crescente, sem locks/exclusões, química só
// informativa) — só ChemistryModel vem de opt. Quem já chama esta função ou
// BuildGauntletPlan não precisa mudar nada; o motor geral (regras
// versionadas, estratégias, locks, exclusões) mora em gauntlet_rules.go.
func BuildGauntletPlanWithOptions(club domain.Club, opt GauntletOptions) GauntletPlan {
	req := DefaultGauntletRequest()
	if opt.ChemistryModel.Nome != "" {
		req.ChemistryModel = opt.ChemistryModel
	}
	return BuildGauntletPlanFromRequest(club, req)
}

// gauntletWarnings avisa sobre as cartas em que a trava de "um jogador por
// elenco" não tem como agir: sem basePlayerEaId nem basePlayerSlug, o bot não
// sabe se duas cartas são o mesmo atleta e trata cada uma como jogador
// próprio (ver domain.Player.PlayerKey).
func gauntletWarnings(pool []gauntletCard) []string {
	cegas := 0
	for _, c := range pool {
		if strings.HasPrefix(c.key, "card:") {
			cegas++
		}
	}
	if cegas == 0 {
		return nil
	}
	return []string{fmt.Sprintf(
		"%d cartas do elenco não trazem o id do jogador-base, então o bot não consegue "+
			"saber se são outra versão de alguém já escalado — rode `./eafcbot run` para recoletar o clube",
		cegas)}
}

// gauntletPool são as cartas do clube com GG Rating conhecido — a mesma
// régua de FindSquadSwaps para comparar DENTRO do elenco (ver CLAUDE.md,
// "duas notas, dois domínios de uso"). Sem essa nota não dá para comparar a
// carta com as demais, nem como titular nem como reserva. Atalho para
// gauntletPoolExcluding sem exclusão nenhuma (ver gauntlet_rules.go).
func gauntletPool(club domain.Club) []gauntletCard {
	return gauntletPoolExcluding(club, nil)
}

// gauntletCard é uma carta do pool com a identidade já resolvida. O índice é
// a chave de "já usei esta carta": Player.ID não serve, porque ter duas
// CÓPIAS da mesma carta no clube é normal no FUT — usá-lo faria a segunda
// cópia desaparecer junto com a primeira, mesmo podendo servir noutra
// rodada.
type gauntletCard struct {
	idx int
	p   domain.ClubPlayer
	key string // domain.Player.PlayerKey(): o JOGADOR, não a carta
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

// matchGauntletRound escolhe os 11 titulares de UMA rodada entre as cartas
// disponíveis, reusando o mesmo fluxo de custo mínimo de OptimizeSquad
// (flowEdge/minCostAugment, squad_optimizer.go) — matching global evita a
// heurística de escolher posição por posição isoladamente, que erraria um
// jogador elegível em mais de uma posição (ex.: CB e CDM) para a posição
// errada.
//
// O grafo tem uma camada a mais que o de OptimizeSquad:
//
//	src -cap1-> JOGADOR -cap1-> carta -custo -GGRatingAt(slot)-> slot -cap1-> sink
//
// O nó de jogador é o que proíbe duas versões do mesmo atleta no mesmo
// elenco (regra do jogo, e o bug que esta camada existe para matar); o nó de
// carta continua garantindo uma escalação por carta. Não dá para expressar
// isso num matching único das 4 rodadas: o custo depende do par
// (carta, slot), mas a capacidade a limitar é por (jogador, rodada) — que
// fica ENTRE os dois e apagaria qual carta passou. Por isso a rodada é
// fixada antes, e o matching roda uma vez por rodada.
//
// Uma carta só entra numa aresta quando GGRatingAt confirma a nota NAQUELA
// posição — sem isso o jogador ainda poderia "jogar lá" (PlaysAt) mas sem
// nota conhecida, o que viraria uma aresta de custo zero e um GG Rating de
// exibição enganoso (CLAUDE.md: "na dúvida, não afirma").
func matchGauntletRound(pool []gauntletCard, formation []domain.SquadSlot) ([]GauntletAssignment, []int, bool) {
	// Carta sem nota em nenhum slot da formação não pode ser titular; deixá-la
	// fora do grafo mantém a busca de caminho barata (o elenco real passa de
	// 800 cartas e só uma fração serve a cada formação).
	cards := make([]gauntletCard, 0, len(pool))
	keys := map[string]int{} // PlayerKey -> índice do nó de jogador
	for _, c := range pool {
		eligible := false
		for _, s := range formation {
			if _, ok := c.p.GGRatingAt(s.Position); ok {
				eligible = true
				break
			}
		}
		if !eligible {
			continue
		}
		if _, seen := keys[c.key]; !seen {
			keys[c.key] = len(keys)
		}
		cards = append(cards, c)
	}
	if len(cards) < len(formation) || len(keys) < len(formation) {
		return nil, nil, false
	}

	playerNode := func(i int) int { return 1 + i }
	cardNode := func(i int) int { return 1 + len(keys) + i }
	slotNode := func(j int) int { return 1 + len(keys) + len(cards) + j }
	n := 2 + len(keys) + len(cards) + len(formation)
	src, sink := 0, n-1

	g := make([][]flowEdge, n)
	add := func(a, b, cap, cost int) {
		g[a] = append(g[a], flowEdge{to: b, rev: len(g[b]), cap: cap, cost: cost})
		g[b] = append(g[b], flowEdge{to: a, rev: len(g[a]) - 1, cap: 0, cost: -cost})
	}
	// Índice, não range sobre o mapa: ordem de aresta muda o desempate do
	// Bellman-Ford, e um plano do Gauntlet que mudasse a cada execução com o
	// mesmo elenco seria impossível de conferir.
	for k := 0; k < len(keys); k++ {
		add(src, playerNode(k), 1, 0)
	}
	for i, c := range cards {
		add(playerNode(keys[c.key]), cardNode(i), 1, 0)
		for j, s := range formation {
			r, ok := c.p.GGRatingAt(s.Position)
			if !ok {
				continue
			}
			add(cardNode(i), slotNode(j), 1, -int(math.Round(r*1000)))
		}
	}
	for j := range formation {
		add(slotNode(j), sink, 1, 0)
	}
	for f := 0; f < len(formation); f++ {
		if !minCostAugment(g, src, sink) {
			return nil, nil, false
		}
	}

	out := make([]GauntletAssignment, 0, len(formation))
	picked := make([]int, 0, len(formation))
	for i, c := range cards {
		for _, e := range g[cardNode(i)] {
			if e.to >= slotNode(0) && e.to < sink && e.cap == 0 {
				s := formation[e.to-slotNode(0)]
				rating, _ := c.p.GGRatingAt(s.Position)
				out = append(out, GauntletAssignment{
					Index: s.Index, Position: s.Position, Player: c.p, Rating: rating,
				})
				picked = append(picked, c.idx)
			}
		}
	}
	return out, picked, true
}

// gauntletBench pega, entre as cartas do pool que não viraram titular, as
// mais fracas primeiro: reserva é só cobertura, não deveria consumir uma
// carta que sobrou sem função melhor (ver CLAUDE.md, decisão fechada).
func gauntletBench(pool []gauntletCard, used map[int]bool) []gauntletCard {
	bench := make([]gauntletCard, 0, len(pool))
	for _, c := range pool {
		if !used[c.idx] {
			bench = append(bench, c)
		}
	}
	sort.Slice(bench, func(i, j int) bool {
		vi, vj := gauntletValue(bench[i].p), gauntletValue(bench[j].p)
		if vi != vj {
			return vi < vj
		}
		return bench[i].p.ID < bench[j].p.ID
	})
	return bench
}

// assignBench reparte as reservas mais fracas em blocos de reservasPerRound,
// na mesma ordem crescente usada para os titulares — sem exigência de força
// mínima por rodada, já que o banco é só cobertura (ver o comentário de
// gauntletBench).
//
// A trava de "um jogador por elenco" vale para o banco também: quem já está
// naquela rodada, titular ou reserva, é pulado e cai na rodada seguinte.
// Devolve o número da rodada que não fechou o banco (0 quando todas
// fecharam).
func assignBench(rounds []GauntletSquad, bench []gauntletCard, reservasPerRound int) int {
	used := make(map[int]bool, len(rounds)*reservasPerRound)
	for i := range rounds {
		keys := make(map[string]bool, len(rounds[i].Starters)+reservasPerRound)
		for _, a := range rounds[i].Starters {
			keys[a.Player.PlayerKey()] = true
		}
		for _, c := range bench {
			if len(rounds[i].Bench) == reservasPerRound {
				break
			}
			if used[c.idx] || keys[c.key] {
				continue
			}
			used[c.idx], keys[c.key] = true, true
			rounds[i].Bench = append(rounds[i].Bench, c.p)
		}
		if len(rounds[i].Bench) < reservasPerRound {
			return rounds[i].Round
		}
	}
	return 0
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
