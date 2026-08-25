package analyze

import (
	"fmt"
	"sort"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// gauntletMinRounds/gauntletMaxRounds: a EA já descreveu eventos com três
// squads únicos e outros com quatro no mesmo ciclo (EA FC 26 Launch Update,
// Cornerstones) — "Gauntlet sempre usa quatro" deixou de ser verdade, e o
// intervalo aceito reflete isso em vez de travar num número só.
const (
	gauntletMinRounds = 3
	gauntletMaxRounds = 5
)

// GauntletRules descreve o formato de UM evento como DADO VERSIONADO, não
// como constante de código — GauntletRounds/gauntletStartersCount/
// gauntletBenchPerRound continuam existindo como o formato padrão histórico
// (ver DefaultGauntletRules), para quem já chama BuildGauntletPlan não
// precisar mudar nada.
type GauntletRules struct {
	Nome         string    `json:"nome"`
	Rodadas      int       `json:"rodadas"`
	Titulares    int       `json:"titulares"`
	Reservas     int       `json:"reservas"`
	Fonte        string    `json:"fonte"`
	VerificadoEm time.Time `json:"verificado_em,omitempty"`
}

// DefaultGauntletRules reproduz o formato histórico documentado (EA FC 26
// FUT Deep Dive): 4 rodadas, 11 titulares, 7 reservas.
func DefaultGauntletRules() GauntletRules {
	return GauntletRules{
		Nome: "Gauntlet", Rodadas: GauntletRounds,
		Titulares: gauntletStartersCount, Reservas: gauntletBenchPerRound,
		Fonte: "padrão",
	}
}

// Validate recusa um formato fora do que já foi observado em algum evento —
// na dúvida sobre um número fora da faixa, o plano falha com motivo em vez
// de tentar montar rodadas que a regra do evento não pediu.
func (r GauntletRules) Validate() error {
	if r.Rodadas < gauntletMinRounds || r.Rodadas > gauntletMaxRounds {
		return fmt.Errorf("regras do evento: rodadas deve estar entre %d e %d, veio %d", gauntletMinRounds, gauntletMaxRounds, r.Rodadas)
	}
	if r.Titulares <= 0 {
		return fmt.Errorf("regras do evento: titulares por rodada deve ser positivo, veio %d", r.Titulares)
	}
	if r.Reservas < 0 {
		return fmt.Errorf("regras do evento: reservas por rodada não pode ser negativo, veio %d", r.Reservas)
	}
	return nil
}

// GauntletStrategy decide COMO distribuir força entre as rodadas. As quatro
// vêm do que builders do gênero e a própria EA já descreveram publicamente —
// ver docs/pesquisa-futgenie.md, seção 6 ("Como funciona o Gauntlet
// Builder"). Nenhuma reescreve matchGauntletRound: a diferença entre elas é
// só QUAL RODADA escolhe primeiro no pool que ainda sobra.
type GauntletStrategy string

const (
	// GauntletCrescente guarda os melhores para a última rodada — o
	// comportamento histórico deste bot (BuildGauntletPlan sem opções):
	// processa a rodada mais numerosa primeiro (a última), a mais fraca por
	// último, então a força cresce da 1 para a N por construção.
	GauntletCrescente GauntletStrategy = "crescente"
	// GauntletMaisForteFirst concentra os melhores na PRIMEIRA rodada,
	// deixando os mais fracos para o final — a opção do builder legado do
	// FutGenie (Gauntlet Genie).
	GauntletMaisForteFirst GauntletStrategy = "mais_forte_primeiro"
	// GauntletEquilibrada, a cada passo, dá a próxima escolha à rodada que
	// SAIRIA MAIS FRACA se ficasse para o final — uma heurística gulosa de
	// nivelamento, não uma prova de minimax exato (o bot não afirma
	// otimalidade que não consegue provar; ver CLAUDE.md).
	GauntletEquilibrada GauntletStrategy = "equilibrada"
	// GauntletValorTotal, a cada passo, dá a próxima escolha à rodada que
	// SAIRIA MAIS FORTE se escolhesse agora — maximiza a soma total entre as
	// rodadas, sem se importar com qual delas fica mais forte. Também uma
	// heurística gulosa, não uma prova de ótimo global.
	GauntletValorTotal GauntletStrategy = "valor_total"
)

func (s GauntletStrategy) valid() bool {
	switch s {
	case GauntletCrescente, GauntletMaisForteFirst, GauntletEquilibrada, GauntletValorTotal:
		return true
	}
	return false
}

// FormationSource diz de onde vem a formação usada no plano. Só
// "observada" (a escalação sincronizada do clube) e "manual" (uma lista de
// slots informada por quem chama) existem hoje — um catálogo de formações
// preset do FC 27 nunca foi confirmado, e inventar um violaria "na dúvida,
// não afirma" (CLAUDE.md).
type FormationSource string

const (
	FormationObservada FormationSource = "observada"
	FormationManual    FormationSource = "manual"
)

// GauntletLock pede que um jogador específico jogue como titular numa
// rodada. Sem ClubItemID confirmado, o bot não sabe QUAL cópia física
// prender quando o elenco tem mais de uma da mesma carta — travar "a carta
// X" nesse caso seria um palpite. O lock degrada então de "prende esta cópia
// física neste slot" (ClubItemID + Position, honrados os dois) para "garante
// que este JOGADOR entra como titular nesta rodada, na melhor cópia e no
// melhor slot disponíveis" (só PlayerID: quantidade preservada, posição não
// prometida).
type GauntletLock struct {
	Round      int             `json:"round"`
	PlayerID   int64           `json:"player_id"`
	ClubItemID string          `json:"club_item_id,omitempty"`
	Position   domain.Position `json:"position,omitempty"` // só respeitado junto de ClubItemID
}

// GauntletRequest generaliza a entrada de BuildGauntletPlan. Use
// DefaultGauntletRequest e ajuste só o que precisar — os zero-valores dos
// outros campos (Excluded nil, Locks nil, ChemistryWeight 0) já são o
// comportamento neutro.
type GauntletRequest struct {
	Rules           GauntletRules
	FormationFrom   FormationSource
	ManualSlots     []domain.SquadSlot // usado quando FormationFrom == FormationManual
	Strategy        GauntletStrategy
	Locks           []GauntletLock
	Excluded        map[int64]bool // Player.ID excluído do pool inteiro
	ChemistryWeight float64        // 0 preserva o comportamento histórico (nenhuma troca por química)
	ChemistryModel  chemistry.Modelo
}

// DefaultGauntletRequest reproduz exatamente o comportamento histórico de
// BuildGauntletPlan: 4 rodadas, formação observada, estratégia crescente,
// sem locks/exclusões, química só informativa (peso 0, nenhuma troca).
func DefaultGauntletRequest() GauntletRequest {
	return GauntletRequest{
		Rules: DefaultGauntletRules(), FormationFrom: FormationObservada,
		Strategy: GauntletCrescente, ChemistryModel: chemistry.ModeloPadrao(),
	}
}

func normalizeGauntletRequest(req GauntletRequest) GauntletRequest {
	if req.Rules.Rodadas == 0 {
		req.Rules = DefaultGauntletRules()
	}
	if req.Strategy == "" {
		req.Strategy = GauntletCrescente
	}
	if req.FormationFrom == "" {
		req.FormationFrom = FormationObservada
	}
	if req.ChemistryModel.Nome == "" {
		req.ChemistryModel = chemistry.ModeloPadrao()
	}
	return req
}

// BuildGauntletPlanFromRequest é o motor geral por trás de BuildGauntletPlan:
// regras versionadas (3-5 rodadas), quatro estratégias de distribuição,
// locks, exclusões e química ponderada. BuildGauntletPlan/
// BuildGauntletPlanWithOptions continuam existindo como atalhos que montam
// DefaultGauntletRequest — nada que já chama essas duas funções muda de
// comportamento.
func BuildGauntletPlanFromRequest(club domain.Club, req GauntletRequest) GauntletPlan {
	req = normalizeGauntletRequest(req)
	plan := GauntletPlan{Status: "unavailable"}

	if err := req.Rules.Validate(); err != nil {
		plan.Reason = err.Error()
		return plan
	}
	if !req.Strategy.valid() {
		plan.Reason = fmt.Sprintf("estratégia %q desconhecida", req.Strategy)
		return plan
	}

	slots, formationLabel, err := resolveGauntletFormation(club, req)
	if err != nil {
		plan.Reason = err.Error()
		return plan
	}
	plan.Formation = formationLabel
	if len(slots) != req.Rules.Titulares {
		plan.Reason = fmt.Sprintf("formação tem %d slots, a regra do evento pede %d titulares por rodada",
			len(slots), req.Rules.Titulares)
		return plan
	}

	pool := gauntletPoolExcluding(club, req.Excluded)
	totalCards := req.Rules.Rodadas * (req.Rules.Titulares + req.Rules.Reservas)
	if len(pool) < totalCards {
		plan.Reason = fmt.Sprintf(
			"elenco tem %d cartas elegíveis com GG Rating conhecido (após exclusões), precisa de %d "+
				"(%d titulares + %d reservas em %d rodadas) para montar o plano",
			len(pool), totalCards, req.Rules.Rodadas*req.Rules.Titulares, req.Rules.Rodadas*req.Rules.Reservas, req.Rules.Rodadas)
		return plan
	}
	plan.Warnings = gauntletWarnings(pool)

	locksByRound := map[int][]GauntletLock{}
	for _, l := range req.Locks {
		if l.Round < 1 || l.Round > req.Rules.Rodadas {
			plan.Reason = fmt.Sprintf("lock pede a rodada %d, fora do intervalo 1..%d deste plano", l.Round, req.Rules.Rodadas)
			return plan
		}
		locksByRound[l.Round] = append(locksByRound[l.Round], l)
	}

	rounds, roundCards, leftover, reason := gauntletBuildRounds(pool, slots, req.Rules.Rodadas, locksByRound, req.Strategy, club)
	if reason != "" {
		plan.Reason = reason
		return plan
	}

	if req.ChemistryWeight > 0 {
		leftover = applyGauntletChemistryWeighting(rounds, roundCards, leftover, req.ChemistryModel, req.ChemistryWeight)
	}

	bench := gauntletBench(pool, gauntletUsedIndices(pool, leftover))
	if faltou := assignBench(rounds, bench, req.Rules.Reservas); faltou != 0 {
		plan.Reason = fmt.Sprintf(
			"rodada %d: não sobraram %d cartas elegíveis para o banco sem repetir jogador já escalado nela",
			faltou, req.Rules.Reservas)
		return plan
	}

	for i := range rounds {
		rounds[i].Quimica = quimicaDaRodada(req.ChemistryModel, rounds[i].Starters)
	}

	plan.Status = "ok"
	plan.Rounds = rounds
	plan.Strategy = gauntletStrategyDescription(req.Strategy)
	return plan
}

// resolveGauntletFormation traduz FormationSource num slice de slots
// concreto para uma requisição de Gauntlet. Atalho para resolveFormationSource
// (compartilhada com o Squad Planner, ver squad_planner.go).
func resolveGauntletFormation(club domain.Club, req GauntletRequest) ([]domain.SquadSlot, string, error) {
	return resolveFormationSource(club, req.FormationFrom, req.ManualSlots)
}

// resolveFormationSource traduz FormationSource num slice de slots concreto
// — sem inventar formação nenhuma que o clube ou o chamador não tenham
// fornecido explicitamente. Compartilhada entre BuildGauntletPlanFromRequest
// e BuildSquadPlan: as duas aceitam a mesma noção de "de onde vem a
// formação" (ver o comentário de FormationSource).
func resolveFormationSource(club domain.Club, source FormationSource, manualSlots []domain.SquadSlot) ([]domain.SquadSlot, string, error) {
	switch source {
	case FormationObservada:
		slots := club.Squad.Starters
		if len(slots) == 0 {
			return nil, "", fmt.Errorf("escalação titular não sincronizada")
		}
		for _, s := range slots {
			if _, ok := club.PlayerByID(s.PlayerID); !ok {
				return nil, "", fmt.Errorf("titular ausente do retrato do clube")
			}
		}
		return slots, club.Squad.Formation, nil
	case FormationManual:
		if len(manualSlots) == 0 {
			return nil, "", fmt.Errorf("formação manual pedida sem slots (ManualSlots vazio)")
		}
		return manualSlots, "manual", nil
	default:
		return nil, "", fmt.Errorf(
			"fonte de formação %q desconhecida — só \"observada\" e \"manual\" existem hoje; um catálogo de "+
				"formações preset do FC 27 ainda não foi confirmado em lugar nenhum, então este bot não inventa um",
			source)
	}
}

// gauntletPoolExcluding é gauntletPool com exclusões aplicadas antes de
// indexar — a carta excluída não entra no pool, então não pode ser titular
// nem reserva em rodada nenhuma.
func gauntletPoolExcluding(club domain.Club, excluded map[int64]bool) []gauntletCard {
	pool := make([]gauntletCard, 0, len(club.Players))
	for _, p := range club.Players {
		if excluded[p.ID] {
			continue
		}
		if gauntletValue(p) > 0 {
			pool = append(pool, gauntletCard{idx: len(pool), p: p, key: p.PlayerKey()})
		}
	}
	return pool
}

// gauntletUsedIndices devolve o conjunto de índices do pool que NÃO estão
// mais na sobra — ou seja, que algum round já usou (titular, depois de
// locks e da troca por química).
func gauntletUsedIndices(pool, leftover []gauntletCard) map[int]bool {
	free := make(map[int]bool, len(leftover))
	for _, c := range leftover {
		free[c.idx] = true
	}
	used := make(map[int]bool, len(pool)-len(leftover))
	for _, c := range pool {
		if !free[c.idx] {
			used[c.idx] = true
		}
	}
	return used
}

// gauntletLockedPick é um lock já resolvido contra o pool ORIGINAL — carta
// concreta, slot concreto, mais o índice físico dela no pool (necessário
// para roundCards/a fase de química conseguirem devolvê-la à sobra numa
// troca, do mesmo jeito que qualquer outra carta escolhida pelo matching).
type gauntletLockedPick struct {
	assignment GauntletAssignment
	idx        int
}

// gauntletBuildRounds monta as N rodadas na ordem de PROCESSAMENTO que a
// estratégia decide — "ordem de processamento" é só uma decisão de qual
// rodada escolhe primeiro no pool que ainda sobra; matchGauntletRound (o
// motor de escalação em si) nunca muda entre estratégias. Locks são
// resolvidos TODOS de uma vez, antes de qualquer rodada ser processada —
// sem isso, uma rodada sem lock nenhum processada primeiro (a ordem depende
// da estratégia) podia "roubar" sem querer a cópia que uma rodada
// processada depois precisava por lock. Devolve também roundCards, paralelo
// a rounds[i].Starters, para a fase de química, e o leftover final (o que
// nenhuma rodada usou).
func gauntletBuildRounds(pool []gauntletCard, slots []domain.SquadSlot, rodadas int,
	locksByRound map[int][]GauntletLock, strategy GauntletStrategy, club domain.Club,
) (rounds []GauntletSquad, roundCards [][]gauntletCard, leftover []gauntletCard, reason string) {
	var allLocks []GauntletLock
	for _, ls := range locksByRound {
		allLocks = append(allLocks, ls...)
	}
	lockedByRound, slotsByRound, available, err := resolveAllGauntletLocks(pool, slots, allLocks, club)
	if err != nil {
		return nil, nil, nil, err.Error()
	}

	rounds = make([]GauntletSquad, rodadas)
	roundCards = make([][]gauntletCard, rodadas)

	remaining := make([]int, 0, rodadas)
	for r := 1; r <= rodadas; r++ {
		remaining = append(remaining, r)
	}

	slotsFor := func(round int) []domain.SquadSlot {
		if s, ok := slotsByRound[round]; ok {
			return s
		}
		return slots
	}
	buildXI := func(round int) ([]GauntletAssignment, []int, bool) {
		remainingSlots := slotsFor(round)
		var xi []GauntletAssignment
		var picked []int
		if len(remainingSlots) > 0 {
			var ok bool
			xi, picked, ok = matchGauntletRound(available, remainingSlots)
			if !ok {
				return nil, nil, false
			}
		}
		for _, lp := range lockedByRound[round] {
			xi = append(xi, lp.assignment)
			picked = append(picked, lp.idx)
		}
		return xi, picked, true
	}

	for len(remaining) > 0 {
		var round int
		var xi []GauntletAssignment
		var picked []int

		switch strategy {
		case GauntletCrescente, GauntletMaisForteFirst:
			pos := 0
			if strategy == GauntletCrescente {
				pos = len(remaining) - 1 // processa a última rodada primeiro: ela escolhe com o pool cheio
			}
			round = remaining[pos]
			remaining = append(remaining[:pos], remaining[pos+1:]...)

			var ok bool
			xi, picked, ok = buildXI(round)
			if !ok {
				return nil, nil, nil, fmt.Sprintf("rodada %d: não sobraram jogadores DIFERENTES elegíveis, por "+
					"posição, para os slots restantes (duas versões do mesmo jogador não podem dividir o mesmo elenco)", round)
			}

		default: // GauntletEquilibrada, GauntletValorTotal: decidido por simulação
			bestPos := -1
			var bestTotal float64
			for i, r := range remaining {
				candXI, candPicked, ok := buildXI(r)
				if !ok {
					return nil, nil, nil, fmt.Sprintf("rodada %d: não sobraram jogadores DIFERENTES elegíveis, por "+
						"posição, para os slots restantes (duas versões do mesmo jogador não podem dividir o mesmo elenco)", r)
				}
				total := 0.0
				for _, a := range candXI {
					total += a.Rating
				}
				win := bestPos == -1
				if strategy == GauntletValorTotal {
					win = win || total > bestTotal
				} else { // equilibrada: prioriza quem sairia mais fraco
					win = win || total < bestTotal
				}
				if win {
					bestPos, bestTotal, round, xi, picked = i, total, r, candXI, candPicked
				}
			}
			remaining = append(remaining[:bestPos], remaining[bestPos+1:]...)
		}

		sort.Slice(xi, func(a, b int) bool { return xi[a].Index < xi[b].Index })
		// picked está na mesma ordem que xi ANTES do sort acima (ambos vêm do
		// mesmo laço em buildXI) — reordena picked junto para roundCards
		// continuar paralelo a Starters depois do sort.
		type pair struct {
			a   GauntletAssignment
			idx int
		}
		pairs := make([]pair, len(xi))
		for i := range xi {
			pairs[i] = pair{xi[i], picked[i]}
		}
		sort.Slice(pairs, func(a, b int) bool { return pairs[a].a.Index < pairs[b].a.Index })

		squad := GauntletSquad{Round: round}
		cards := make([]gauntletCard, len(pairs))
		for i, p := range pairs {
			a := p.a
			a.Round = round
			squad.Starters = append(squad.Starters, a)
			squad.TotalRating += a.Rating
			cards[i] = gauntletCard{idx: p.idx, p: a.Player, key: a.Player.PlayerKey()}
		}
		squad.AverageRating = squad.TotalRating / float64(len(squad.Starters))
		rounds[round-1] = squad
		roundCards[round-1] = cards

		usedIdx := make(map[int]bool, len(picked))
		for _, idx := range picked {
			usedIdx[idx] = true
		}
		rest := make([]gauntletCard, 0, len(available)-len(picked))
		for _, c := range available {
			if !usedIdx[c.idx] {
				rest = append(rest, c)
			}
		}
		available = rest
	}
	return rounds, roundCards, available, ""
}

// resolveAllGauntletLocks resolve TODOS os locks de uma vez, contra o pool
// ORIGINAL — antes de qualquer rodada ser processada. Com ClubItemID
// confirmado, prende exatamente aquela cópia física no slot da Position
// pedida; sem ClubItemID, garante que uma cópia do JOGADOR (PlayerID) entra
// como titular na melhor combinação carta×slot livre que sobrar naquela
// rodada — sem prometer qual cópia nem qual slot exato, porque não há como
// saber (ver o comentário de GauntletLock). Devolve o pool JÁ SEM as cartas
// travadas — nenhuma rodada sem lock, processada antes ou depois, pode
// "roubar" uma cópia que outra rodada reservou.
func resolveAllGauntletLocks(pool []gauntletCard, slots []domain.SquadSlot, locks []GauntletLock, club domain.Club) (
	lockedByRound map[int][]gauntletLockedPick, slotsByRound map[int][]domain.SquadSlot, remainingPool []gauntletCard, err error,
) {
	lockedByRound = map[int][]gauntletLockedPick{}
	usedCard := map[int]bool{}
	usedSlotByRound := map[int]map[int]bool{}

	for _, lock := range locks {
		var candidates []gauntletCard
		who := fmt.Sprintf("jogador %d", lock.PlayerID)
		if lock.ClubItemID != "" {
			who = fmt.Sprintf("cópia %q", lock.ClubItemID)
			for _, c := range pool {
				if c.p.ClubItemID == lock.ClubItemID && !usedCard[c.idx] {
					candidates = append(candidates, c)
					break
				}
			}
		} else {
			p, ok := club.PlayerByID(lock.PlayerID)
			if !ok {
				return nil, nil, nil, fmt.Errorf("lock: %s não está no elenco", who)
			}
			key := p.PlayerKey()
			for _, c := range pool {
				if c.key == key && !usedCard[c.idx] {
					candidates = append(candidates, c)
				}
			}
		}
		if len(candidates) == 0 {
			return nil, nil, nil, fmt.Errorf("lock: nenhuma cópia disponível de %s na rodada %d "+
				"(já usada em outro lock, excluída, ou o elenco não tem)", who, lock.Round)
		}

		if usedSlotByRound[lock.Round] == nil {
			usedSlotByRound[lock.Round] = map[int]bool{}
		}
		usedSlot := usedSlotByRound[lock.Round]

		bestCand, bestSlot, bestRating, found := -1, -1, 0.0, false
		for ci, c := range candidates {
			for si, s := range slots {
				if usedSlot[si] {
					continue
				}
				if lock.ClubItemID != "" && lock.Position != "" && s.Position != lock.Position {
					continue // posição só é honrada junto de ClubItemID confirmado
				}
				r, ok := c.p.GGRatingAt(s.Position)
				if !ok {
					continue
				}
				if !found || r > bestRating {
					bestCand, bestSlot, bestRating, found = ci, si, r, true
				}
			}
		}
		if !found {
			return nil, nil, nil, fmt.Errorf("lock: %s não tem posição elegível entre os slots livres da rodada %d", who, lock.Round)
		}
		chosen := candidates[bestCand]
		s := slots[bestSlot]
		lockedByRound[lock.Round] = append(lockedByRound[lock.Round], gauntletLockedPick{
			assignment: GauntletAssignment{Index: s.Index, Position: s.Position, Player: chosen.p, Rating: bestRating},
			idx:        chosen.idx,
		})
		usedSlot[bestSlot] = true
		usedCard[chosen.idx] = true
	}

	slotsByRound = map[int][]domain.SquadSlot{}
	for round, usedSlot := range usedSlotByRound {
		var rem []domain.SquadSlot
		for si, s := range slots {
			if !usedSlot[si] {
				rem = append(rem, s)
			}
		}
		slotsByRound[round] = rem
	}
	for _, c := range pool {
		if !usedCard[c.idx] {
			remainingPool = append(remainingPool, c)
		}
	}
	return lockedByRound, slotsByRound, remainingPool, nil
}

// applyGauntletChemistryWeighting é a "segunda fase de busca local" que
// internal/chemistry.Contador foi desenhado para viabilizar (ver
// docs/pesquisa-futgenie.md) — nunca reescreve matchGauntletRound; troca,
// DEPOIS do matching de cada rodada, um titular pela carta da sobra que mais
// melhora rating + peso*ganho_de_química. Com peso 0 esta função nunca é
// chamada (ver o guard em BuildGauntletPlanFromRequest), então o
// comportamento histórico fica intacto por construção, não por sorte.
func applyGauntletChemistryWeighting(rounds []GauntletSquad, roundCards [][]gauntletCard, leftover []gauntletCard,
	m chemistry.Modelo, weight float64,
) []gauntletCard {
	for i := range rounds {
		leftover = chemistrySwapRound(&rounds[i], roundCards[i], leftover, m, weight)
	}
	return leftover
}

// chemistrySwapRound considera, uma vez por slot (uma passada só — simples,
// limitada e previsível em vez de buscar até convergência), a melhor troca
// disponível na sobra. Aceita a troca só quando o placar ponderado
// (delta de rating + peso*delta de química) é estritamente positivo.
func chemistrySwapRound(round *GauntletSquad, cards []gauntletCard, leftover []gauntletCard, m chemistry.Modelo, weight float64) []gauntletCard {
	xi := make([]chemistry.Titular, len(round.Starters))
	for i, a := range round.Starters {
		xi[i] = chemistry.Titular{Index: a.Index, Position: a.Position, Player: a.Player.Player}
	}
	contador := chemistry.NovoContador(m, xi)
	baseChem := contador.Total()

	for slotIdx := range round.Starters {
		curAssignment := round.Starters[slotIdx]
		curCard := cards[slotIdx]

		bestCandidate, bestScore := -1, 0.0
		for ci, cand := range leftover {
			r, ok := cand.p.GGRatingAt(curAssignment.Position)
			if !ok {
				continue
			}
			conflict := false
			for j, other := range round.Starters {
				if j != slotIdx && other.Player.PlayerKey() == cand.key {
					conflict = true
					break
				}
			}
			if conflict {
				continue
			}
			deltaRating := r - curAssignment.Rating
			deltaChem := float64(contador.TotalSe(slotIdx, cand.p.Player) - baseChem)
			score := deltaRating + weight*deltaChem
			if bestCandidate < 0 || score > bestScore {
				bestScore, bestCandidate = score, ci
			}
		}
		if bestCandidate < 0 || bestScore <= 1e-9 {
			continue
		}

		chosen := leftover[bestCandidate]
		newRating, _ := chosen.p.GGRatingAt(curAssignment.Position)

		round.TotalRating += newRating - curAssignment.Rating
		round.Starters[slotIdx].Player = chosen.p
		round.Starters[slotIdx].Rating = newRating
		cards[slotIdx] = chosen
		contador.Aplicar(slotIdx, chosen.p.Player)
		baseChem = contador.Total()

		leftover[bestCandidate] = curCard // a carta deslocada volta pra sobra
	}
	round.AverageRating = round.TotalRating / float64(len(round.Starters))
	return leftover
}

func gauntletStrategyDescription(s GauntletStrategy) string {
	switch s {
	case GauntletMaisForteFirst:
		return "Mais forte primeiro: a primeira rodada escolhe primeiro no pool inteiro e concentra os melhores " +
			"titulares; as seguintes usam o que sobrou, então a força cai da 1 para a última. Mesmo motor de " +
			"matching global de GG Rating das outras estratégias — só a ordem de escolha muda."
	case GauntletEquilibrada:
		return "Equilibrada: a cada passo, a rodada que sairia mais fraca se ficasse para o fim escolhe primeiro " +
			"— uma heurística de nivelamento, não uma prova de mínimo garantido. Tende a deixar as rodadas mais " +
			"parecidas em força do que crescente ou mais-forte-primeiro."
	case GauntletValorTotal:
		return "Valor total: a cada passo, a rodada que ganharia mais escolhendo agora tem prioridade — maximiza " +
			"a soma de GG Rating entre todas as rodadas, sem se importar com qual delas fica mais forte."
	default: // GauntletCrescente
		return "Crescente: uma rodada por vez, da última para a primeira — cada rodada leva o melhor XI possível " +
			"entre as cartas que ainda sobraram, por matching global de GG Rating (nunca posição por posição " +
			"isolada). Os melhores ficam guardados para a última partida e a força cresce por construção. " +
			"Nenhum elenco tem duas versões do mesmo jogador, banco incluso, porque o jogo não aceita. Reservas " +
			"usam as cartas elegíveis restantes mais fracas, sem tirar lugar de titular nenhum."
	}
}
