package analyze

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// SquadPlanGoal decide qual ponta da fronteira nota×química o primeiro
// cenário da resposta representa — os cenários em si continuam sendo a
// mesma fronteira Pareto para qualquer objetivo, só a ORDEM muda.
type SquadPlanGoal string

const (
	SquadPlanGoalEquilibrado  SquadPlanGoal = "equilibrado"
	SquadPlanGoalMaiorNota    SquadPlanGoal = "maior_nota"
	SquadPlanGoalMaiorQuimica SquadPlanGoal = "maior_quimica"
)

func (g SquadPlanGoal) valid() bool {
	switch g {
	case SquadPlanGoalEquilibrado, SquadPlanGoalMaiorNota, SquadPlanGoalMaiorQuimica:
		return true
	}
	return false
}

// SquadPlanLock pede que um jogador específico jogue como titular no plano.
// Mesma distinção de GauntletLock (ver o comentário lá): com ClubItemID
// confirmado, prende a cópia física exata num slot; sem ele, degrada para
// "quantidade preservada" — garante o jogador no XI, sem prometer slot.
type SquadPlanLock struct {
	PlayerID   int64           `json:"player_id"`
	ClubItemID string          `json:"club_item_id,omitempty"`
	Position   domain.Position `json:"position,omitempty"` // só respeitado junto de ClubItemID
}

// SquadPlanRequest generaliza a entrada do Squad Planner. Use
// DefaultSquadPlanRequest e ajuste só o que precisar. Orçamento não mora
// aqui de propósito: o Squad Planner nunca escolhe compra nenhuma (só
// "necessidades", ver SquadPlanNeed), então dinheiro não influencia o
// algoritmo — quem quiser mostrar domain.Capital junto da resposta computa
// direto de domain.Club, como toda outra rota da API já faz.
type SquadPlanRequest struct {
	Goal           SquadPlanGoal
	FormationFrom  FormationSource
	ManualSlots    []domain.SquadSlot // usado quando FormationFrom == FormationManual
	Locks          []SquadPlanLock
	Excluded       map[int64]bool // Player.ID excluído do pool inteiro
	MaxScenarios   int            // 3-5; fora da faixa é ajustado, não recusado
	ChemistryModel chemistry.Modelo
}

const (
	defaultSquadPlanMaxScenarios = 5
	squadPlanMinScenarios        = 3
	squadPlanMaxScenariosCap     = 5
)

// DefaultSquadPlanRequest usa a formação observada, objetivo equilibrado,
// sem locks/exclusões, até 5 cenários.
func DefaultSquadPlanRequest() SquadPlanRequest {
	return SquadPlanRequest{
		Goal: SquadPlanGoalEquilibrado, FormationFrom: FormationObservada,
		MaxScenarios: defaultSquadPlanMaxScenarios, ChemistryModel: chemistry.ModeloPadrao(),
	}
}

func normalizeSquadPlanRequest(req SquadPlanRequest) SquadPlanRequest {
	if req.Goal == "" {
		req.Goal = SquadPlanGoalEquilibrado
	}
	if req.FormationFrom == "" {
		req.FormationFrom = FormationObservada
	}
	if req.MaxScenarios <= 0 {
		req.MaxScenarios = defaultSquadPlanMaxScenarios
	}
	if req.MaxScenarios > squadPlanMaxScenariosCap {
		req.MaxScenarios = squadPlanMaxScenariosCap
	}
	if req.ChemistryModel.Nome == "" {
		req.ChemistryModel = chemistry.ModeloPadrao()
	}
	return req
}

// SquadPlanScenario é UM ponto da fronteira nota×química: o XI completo,
// nota, química e os movimentos (em relação ao XI ATUAL, quando a formação é
// observada) para chegar lá.
type SquadPlanScenario struct {
	Label           string               `json:"label"`
	ChemistryWeight float64              `json:"chemistry_weight"`
	Starters        []SquadAssignment    `json:"starters"`
	TotalRating     float64              `json:"total_rating"`
	AverageRating   float64              `json:"average_rating"`
	Quimica         *chemistry.Resultado `json:"chemistry,omitempty"`
	Moves           []SquadMove          `json:"moves"`
}

// SquadPlanNeed é uma posição que precisa de reforço — o Squad Planner só
// APONTA a necessidade; ele nunca escolhe qual carta comprar (isso é
// analyze.FindUpgrades, do lado do mercado — ver CLAUDE.md).
type SquadPlanNeed struct {
	Index    int             `json:"index"`
	Position domain.Position `json:"position"`
	Reason   string          `json:"reason"`
}

// SquadPlannerPlan é a saída do Squad Planner: de squadPlanMinScenarios a 5
// cenários Pareto (nota × química), necessidades de mercado, e diagnóstico
// de inviabilidade quando não dá pra montar XI nenhum.
type SquadPlannerPlan struct {
	Status    string              `json:"status"` // "ok" | "unavailable"
	Reason    string              `json:"reason,omitempty"`
	Formation string              `json:"formation"`
	Scenarios []SquadPlanScenario `json:"scenarios"`
	Needs     []SquadPlanNeed     `json:"needs"`
	Warnings  []string            `json:"warnings,omitempty"`
}

// BuildSquadPlan monta o Squad Planner: resolve formação, aplica exclusões e
// locks, gera a fronteira nota×química e aponta necessidades de mercado.
// Nunca reescreve squadMatch (squad_optimizer.go) — cada cenário é o mesmo
// fluxo de custo mínimo de sempre, só que sobre o pool/slots restantes após
// locks, com uma segunda fase opcional de busca local por química (ver
// chemistrySwapSquad) exatamente como o Gauntlet generalizado faz
// (gauntlet_rules.go).
func BuildSquadPlan(club domain.Club, req SquadPlanRequest) SquadPlannerPlan {
	req = normalizeSquadPlanRequest(req)
	plan := SquadPlannerPlan{Status: "unavailable"}

	if !req.Goal.valid() {
		plan.Reason = fmt.Sprintf("objetivo %q desconhecido", req.Goal)
		return plan
	}

	slots, formationLabel, err := resolveFormationSource(club, req.FormationFrom, req.ManualSlots)
	if err != nil {
		plan.Reason = err.Error()
		return plan
	}
	plan.Formation = formationLabel

	// A checagem de "titular presente e com nota conhecida" só faz sentido
	// para a formação OBSERVADA — ela é o que dá a cada slot um "titular
	// atual" pra comparar (ver squadPlanMoves). Formação manual não tem
	// titular atual nenhum: todo mundo escolhido é uma "sugestão" nova.
	if req.FormationFrom == FormationObservada {
		for _, s := range slots {
			p, ok := club.PlayerByID(s.PlayerID)
			if !ok {
				plan.Reason = "titular ausente do retrato do clube"
				return plan
			}
			if _, ok := p.GGRatingAt(s.Position); !ok {
				plan.Reason = "faltam notas GG Rating por posição; nova coleta necessária"
				return plan
			}
		}
	}

	players := squadPlanPoolExcluding(club.Players, req.Excluded)

	locked, remainingSlots, remainingPlayers, err := resolveSquadPlanLocks(players, slots, req.Locks, club)
	if err != nil {
		plan.Reason = err.Error()
		return plan
	}

	baseline, ok := buildSquadScenario(remainingPlayers, remainingSlots, locked, slots, club, req.ChemistryModel, 0, "")
	if !ok {
		plan.Reason = "não há jogadores elegíveis para todos os slots"
		return plan
	}

	scenarios := paretoSquadScenarios(remainingPlayers, remainingSlots, locked, slots, club, req)
	if len(scenarios) == 0 {
		scenarios = []SquadPlanScenario{baseline}
	}
	labelSquadScenarios(scenarios, req.Goal)

	plan.Status = "ok"
	plan.Scenarios = scenarios
	plan.Needs = squadPlanNeeds(baseline, players)
	return plan
}

// squadPlanPoolExcluding filtra o Player.ID excluído — mesma régua de
// gauntletPoolExcluding, sem a checagem de GG Rating (squadMatch já ignora
// carta sem nota via o custo zero implícito, mesmo comportamento que
// OptimizeSquad sempre teve).
func squadPlanPoolExcluding(players []domain.ClubPlayer, excluded map[int64]bool) []domain.ClubPlayer {
	if len(excluded) == 0 {
		return players
	}
	out := make([]domain.ClubPlayer, 0, len(players))
	for _, p := range players {
		if !excluded[p.ID] {
			out = append(out, p)
		}
	}
	return out
}

// resolveSquadPlanLocks resolve cada SquadPlanLock contra o pool: com
// ClubItemID confirmado, prende exatamente aquela cópia física no slot da
// Position pedida; sem ele, garante que uma cópia do JOGADOR entra como
// titular na melhor combinação carta×slot livre que sobrar — sem prometer
// qual cópia nem qual slot exato (ver o comentário de SquadPlanLock).
func resolveSquadPlanLocks(players []domain.ClubPlayer, slots []domain.SquadSlot, locks []SquadPlanLock, club domain.Club) (
	locked []SquadAssignment, remainingSlots []domain.SquadSlot, remainingPlayers []domain.ClubPlayer, err error,
) {
	usedSlot := make(map[int]bool, len(locks))
	usedPlayer := make(map[int]bool, len(locks))

	for _, lock := range locks {
		var candidates []int
		who := fmt.Sprintf("jogador %d", lock.PlayerID)
		if lock.ClubItemID != "" {
			who = fmt.Sprintf("cópia %q", lock.ClubItemID)
			for i, p := range players {
				if p.ClubItemID == lock.ClubItemID && !usedPlayer[i] {
					candidates = append(candidates, i)
					break
				}
			}
		} else {
			p, ok := club.PlayerByID(lock.PlayerID)
			if !ok {
				return nil, nil, nil, fmt.Errorf("lock: %s não está no elenco", who)
			}
			key := p.PlayerKey()
			for i, cand := range players {
				if cand.PlayerKey() == key && !usedPlayer[i] {
					candidates = append(candidates, i)
				}
			}
		}
		if len(candidates) == 0 {
			return nil, nil, nil, fmt.Errorf(
				"lock: nenhuma cópia disponível de %s (já usada em outro lock, excluída, ou o elenco não tem)", who)
		}

		bestIdx, bestSlot, bestRating, found := -1, -1, 0.0, false
		for _, ci := range candidates {
			for si, s := range slots {
				if usedSlot[si] {
					continue
				}
				if lock.ClubItemID != "" && lock.Position != "" && s.Position != lock.Position {
					continue // posição só é honrada junto de ClubItemID confirmado
				}
				r, ok := players[ci].GGRatingAt(s.Position)
				if !ok {
					continue
				}
				if !found || r > bestRating {
					bestIdx, bestSlot, bestRating, found = ci, si, r, true
				}
			}
		}
		if !found {
			return nil, nil, nil, fmt.Errorf("lock: %s não tem posição elegível entre os slots livres", who)
		}
		s := slots[bestSlot]
		locked = append(locked, SquadAssignment{Index: s.Index, Position: s.Position, Player: players[bestIdx], Rating: bestRating})
		usedSlot[bestSlot] = true
		usedPlayer[bestIdx] = true
	}

	for si, s := range slots {
		if !usedSlot[si] {
			remainingSlots = append(remainingSlots, s)
		}
	}
	for i, p := range players {
		if !usedPlayer[i] {
			remainingPlayers = append(remainingPlayers, p)
		}
	}
	return locked, remainingSlots, remainingPlayers, nil
}

// buildSquadScenario roda squadMatch sobre os slots/pool restantes (após
// locks), mescla com os assignments travados e — quando weight > 0 —
// aplica chemistrySwapSquad. Devolve false quando nem o matching básico
// (sem química nenhuma) cobre todos os slots.
func buildSquadScenario(players []domain.ClubPlayer, remainingSlots []domain.SquadSlot, locked []SquadAssignment,
	allSlots []domain.SquadSlot, club domain.Club, m chemistry.Modelo, weight float64, label string,
) (SquadPlanScenario, bool) {
	var starters []SquadAssignment
	if len(remainingSlots) > 0 {
		chosen, ok := squadMatch(players, remainingSlots)
		if !ok {
			return SquadPlanScenario{}, false
		}
		for j, s := range remainingSlots {
			r, _ := chosen[j].GGRatingAt(s.Position)
			starters = append(starters, SquadAssignment{s.Index, s.Position, chosen[j], r})
		}
	}
	starters = append(starters, locked...)
	sort.Slice(starters, func(a, b int) bool { return starters[a].Index < starters[b].Index })

	if weight > 0 {
		// Slots travados por lock não podem ser tocados pela busca de
		// química — senão um lock "sem ClubItemID" (garantia de presença)
		// virava garantia nenhuma assim que ChemistryWeight > 0 achasse uma
		// troca melhor. squadPlanLeftover usa `players` (já filtrado por
		// exclusão e sem as cartas travadas), não club.Players inteiro —
		// senão uma carta excluída podia reentrar pela busca de química.
		lockedIdx := make(map[int]bool, len(locked))
		for _, l := range locked {
			lockedIdx[l.Index] = true
		}
		starters = chemistrySwapSquad(starters, squadPlanLeftover(players, starters), m, weight, lockedIdx)
	}

	sc := SquadPlanScenario{Label: label, ChemistryWeight: weight, Starters: starters}
	for _, a := range starters {
		sc.TotalRating += a.Rating
	}
	if len(starters) > 0 {
		sc.AverageRating = sc.TotalRating / float64(len(starters))
	}
	sc.Moves = squadPlanMoves(starters, allSlots, club)
	sc.Quimica = quimicaDaSugestao(m, starters)
	return sc, true
}

// squadPlanLeftover é o pool inteiro do clube menos quem já está no XI —
// candidatos possíveis para a busca local de química.
func squadPlanLeftover(allPlayers []domain.ClubPlayer, starters []SquadAssignment) []domain.ClubPlayer {
	used := make(map[string]bool, len(starters))
	for _, a := range starters {
		used[a.Player.PlayerKey()] = true
	}
	out := make([]domain.ClubPlayer, 0, len(allPlayers))
	for _, p := range allPlayers {
		if !used[p.PlayerKey()] {
			out = append(out, p)
		}
	}
	return out
}

// squadPlanMoves compara o XI escolhido contra o titular ATUAL de cada slot
// (allSlots, que inclui os slots travados por lock) — formação manual não
// tem "atual" nenhum (PlayerID zero), então não gera movimento.
func squadPlanMoves(starters []SquadAssignment, allSlots []domain.SquadSlot, club domain.Club) []SquadMove {
	byIndex := make(map[int]domain.SquadSlot, len(allSlots))
	for _, s := range allSlots {
		byIndex[s.Index] = s
	}
	var moves []SquadMove
	for _, a := range starters {
		s, ok := byIndex[a.Index]
		if !ok || s.PlayerID == 0 || a.Player.ID == s.PlayerID {
			continue
		}
		cr, ok := club.PlayerByID(s.PlayerID)
		if !ok {
			continue
		}
		cur, _ := cr.GGRatingAt(s.Position)
		moves = append(moves, SquadMove{a.Index, a.Position, cr, a.Player, cur, a.Rating, a.Rating - cur})
	}
	return moves
}

// chemistrySwapSquad é o equivalente, para um XI só, de chemistrySwapRound
// (gauntlet_rules.go) — a mesma busca local de UMA passada por slot, sobre
// chemistry.Contador, DEPOIS do matching por nota. Peso 0 nunca é chamado
// (ver o guard em buildSquadScenario), preservando por construção o
// comportamento de OptimizeSquad para quem não pediu química ponderada.
func chemistrySwapSquad(starters []SquadAssignment, leftover []domain.ClubPlayer, m chemistry.Modelo, weight float64, lockedIdx map[int]bool) []SquadAssignment {
	xi := make([]chemistry.Titular, len(starters))
	for i, a := range starters {
		xi[i] = chemistry.Titular{Index: a.Index, Position: a.Position, Player: a.Player.Player}
	}
	contador := chemistry.NovoContador(m, xi)
	baseChem := contador.Total()

	out := append([]SquadAssignment(nil), starters...)
	pool := append([]domain.ClubPlayer(nil), leftover...)

	for slotIdx := range out {
		if lockedIdx[out[slotIdx].Index] {
			continue // travado por lock: a busca de química não pode desfazer a garantia
		}
		cur := out[slotIdx]
		bestCandidate, bestScore := -1, 0.0
		for pi, cand := range pool {
			r, ok := cand.GGRatingAt(cur.Position)
			if !ok {
				continue
			}
			conflict := false
			for j, other := range out {
				if j != slotIdx && other.Player.PlayerKey() == cand.PlayerKey() {
					conflict = true
					break
				}
			}
			if conflict {
				continue
			}
			deltaRating := r - cur.Rating
			deltaChem := float64(contador.TotalSe(slotIdx, cand.Player) - baseChem)
			score := deltaRating + weight*deltaChem
			if bestCandidate < 0 || score > bestScore {
				bestScore, bestCandidate = score, pi
			}
		}
		if bestCandidate < 0 || bestScore <= 1e-9 {
			continue
		}
		chosen := pool[bestCandidate]
		newRating, _ := chosen.GGRatingAt(cur.Position)
		pool[bestCandidate] = out[slotIdx].Player
		out[slotIdx].Player = chosen
		out[slotIdx].Rating = newRating
		contador.Aplicar(slotIdx, chosen.Player)
		baseChem = contador.Total()
	}
	return out
}

// squadPlanWeightSweep é a faixa de pesos usada para aproximar a fronteira
// Pareto nota×química por soma ponderada — técnica conhecida, mas que só
// alcança a parte CONVEXA da fronteira real; pontos não-convexos podem
// ficar de fora (CLAUDE.md: não afirmar o que não é provado).
var squadPlanWeightSweep = []float64{0, 0.5, 1, 2, 5, 10, 25, 60}

// paretoSquadScenarios varre squadPlanWeightSweep, descarta cenários
// duplicados (o mesmo XI pode sair de pesos diferentes) e devolve só os NÃO
// DOMINADOS — nenhum outro cenário tem nota E química maiores ou iguais,
// com pelo menos uma estritamente maior —, cortado em req.MaxScenarios.
func paretoSquadScenarios(players []domain.ClubPlayer, remainingSlots []domain.SquadSlot, locked []SquadAssignment,
	allSlots []domain.SquadSlot, club domain.Club, req SquadPlanRequest,
) []SquadPlanScenario {
	var candidates []SquadPlanScenario
	seen := map[string]bool{}
	for _, w := range squadPlanWeightSweep {
		sc, ok := buildSquadScenario(players, remainingSlots, locked, allSlots, club, req.ChemistryModel, w, "")
		if !ok {
			continue
		}
		key := squadScenarioKey(sc)
		if seen[key] {
			continue
		}
		seen[key] = true
		candidates = append(candidates, sc)
	}

	frontier := dominantSquadScenarios(candidates)
	if len(frontier) > req.MaxScenarios {
		frontier = spreadSquadScenarios(frontier, req.MaxScenarios)
	}
	return frontier
}

func squadScenarioKey(sc SquadPlanScenario) string {
	var sb strings.Builder
	for _, a := range sc.Starters {
		sb.WriteString(strconv.FormatInt(a.Player.ID, 10))
		sb.WriteByte(',')
	}
	return sb.String()
}

func chemTotal(sc SquadPlanScenario) int {
	if sc.Quimica == nil {
		return 0
	}
	return sc.Quimica.Total
}

// dominantSquadScenarios devolve os cenários não dominados, ordenados por
// nota decrescente.
func dominantSquadScenarios(all []SquadPlanScenario) []SquadPlanScenario {
	var out []SquadPlanScenario
	for i, a := range all {
		dominated := false
		for j, b := range all {
			if i == j {
				continue
			}
			aChem, bChem := chemTotal(a), chemTotal(b)
			if b.AverageRating >= a.AverageRating && bChem >= aChem && (b.AverageRating > a.AverageRating || bChem > aChem) {
				dominated = true
				break
			}
		}
		if !dominated {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AverageRating > out[j].AverageRating })
	return out
}

// spreadSquadScenarios escolhe `max` pontos espalhados ao longo da fronteira
// já ordenada por nota — preserva as duas pontas (maior nota, maior
// química) em vez de cortar arbitrariamente pelo início da lista.
func spreadSquadScenarios(sorted []SquadPlanScenario, max int) []SquadPlanScenario {
	if max <= 1 || len(sorted) <= max {
		return sorted
	}
	out := make([]SquadPlanScenario, 0, max)
	step := float64(len(sorted)-1) / float64(max-1)
	for i := 0; i < max; i++ {
		idx := int(math.Round(float64(i) * step))
		out = append(out, sorted[idx])
	}
	return out
}

// labelSquadScenarios rotula cada cenário (maior nota / maior química /
// equilíbrio) e reordena a lista pelo objetivo pedido — o mais alinhado ao
// objetivo aparece primeiro, sem descartar os outros pontos da fronteira.
func labelSquadScenarios(scenarios []SquadPlanScenario, goal SquadPlanGoal) {
	if len(scenarios) == 0 {
		return
	}
	bestRatingIdx, bestChemIdx := 0, 0
	for i, sc := range scenarios {
		if sc.AverageRating > scenarios[bestRatingIdx].AverageRating {
			bestRatingIdx = i
		}
		if chemTotal(sc) > chemTotal(scenarios[bestChemIdx]) {
			bestChemIdx = i
		}
	}
	for i := range scenarios {
		switch {
		case i == bestRatingIdx && i == bestChemIdx:
			scenarios[i].Label = "melhor nota e melhor química juntas"
		case i == bestRatingIdx:
			scenarios[i].Label = "maior nota"
		case i == bestChemIdx:
			scenarios[i].Label = "maior química"
		default:
			scenarios[i].Label = "equilíbrio"
		}
	}
	sort.SliceStable(scenarios, func(i, j int) bool {
		switch goal {
		case SquadPlanGoalMaiorQuimica:
			return chemTotal(scenarios[i]) > chemTotal(scenarios[j])
		case SquadPlanGoalMaiorNota:
			return scenarios[i].AverageRating > scenarios[j].AverageRating
		default: // equilibrado: mantém a ordem por nota que dominantSquadScenarios já deu
			return false
		}
	})
}

// squadPlanNeedGapThreshold é quantos pontos de GG Rating abaixo da média do
// time já conta como "abaixo da média" o bastante pra virar necessidade de
// mercado — limiar fixo, mesmo espírito do "elo mais fraco" que
// WeakestLinks usa em outro contexto (upgrade.go), sem reusar o código dele
// (WeakestLinks compara por Score()/GG Rating do mercado; aqui é só sobre o
// elenco atual).
const squadPlanNeedGapThreshold = 3.0

// squadPlanNeeds aponta, a partir do cenário BASE (peso de química 0 — a
// visão mais objetiva do elenco), as posições sem alternativa no banco ou
// notavelmente abaixo da média do time. Nunca escolhe uma compra — isso é
// FindUpgrades, do lado do mercado.
func squadPlanNeeds(baseline SquadPlanScenario, players []domain.ClubPlayer) []SquadPlanNeed {
	altCount := squadPlanAlternativeCounts(baseline.Starters, players)
	var needs []SquadPlanNeed
	for _, a := range baseline.Starters {
		switch {
		case altCount[a.Index] == 0:
			needs = append(needs, SquadPlanNeed{Index: a.Index, Position: a.Position, Reason: "sem alternativa no elenco além do titular escalado"})
		case a.Rating < baseline.AverageRating-squadPlanNeedGapThreshold:
			needs = append(needs, SquadPlanNeed{Index: a.Index, Position: a.Position, Reason: "abaixo da média do time"})
		}
	}
	return needs
}

func squadPlanAlternativeCounts(starters []SquadAssignment, players []domain.ClubPlayer) map[int]int {
	used := make(map[string]bool, len(starters))
	for _, a := range starters {
		used[a.Player.PlayerKey()] = true
	}
	counts := make(map[int]int, len(starters))
	for _, a := range starters {
		count := 0
		for _, p := range players {
			if used[p.PlayerKey()] || !p.PlaysAt(a.Position) {
				continue
			}
			if _, ok := p.GGRatingAt(a.Position); ok {
				count++
			}
		}
		counts[a.Index] = count
	}
	return counts
}
