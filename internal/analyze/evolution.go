package analyze

import (
	"sort"
	"strings"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// EvoMatch é uma evolução que cabe num jogador seu, já com a projeção
// de como a carta fica no fim.
type EvoMatch struct {
	Evolution           domain.Evolution      `json:"evolution"`
	Player              domain.ClubPlayer     `json:"player"`
	Slot                domain.Position       `json:"slot"`  // função onde o ganho foi medido
	Index               int                   `json:"index"` // vaga física comparada; use HasComparison para distinguir ausência do índice zero
	Before              float64               `json:"before"`
	After               float64               `json:"after"`
	Gain                float64               `json:"gain"`
	Result              domain.Player         `json:"result"` // a carta projetada
	Cost                int                   `json:"cost"`
	Affordable          bool                  `json:"affordable"`
	Acquisition         string                `json:"acquisition"`
	CardSlug            string                `json:"card_slug,omitempty"`
	BeatsStarter        bool                  `json:"beats_starter"` // vira titular na posição?
	ImprovesStarter     bool                  `json:"improves_starter"`
	HasComparison       bool                  `json:"has_comparison"`
	Current             *domain.ClubPlayer    `json:"current,omitempty"`
	ReplacementGain     float64               `json:"replacement_gain,omitempty"`
	Highlights          []string              `json:"highlights"`
	BeforeEvaluation    domain.AvaliacaoCarta `json:"before_evaluation,omitempty"`
	AfterEvaluation     domain.AvaliacaoCarta `json:"after_evaluation,omitempty"`
	CurrentEvaluation   domain.AvaliacaoCarta `json:"current_evaluation,omitempty"`
	CandidateEvaluation domain.AvaliacaoCarta `json:"candidate_evaluation,omitempty"`
}

// EvolutionOptions governa o cruzamento diário de evoluções com o elenco.
type EvolutionOptions struct {
	Budget              int
	MinRating           int
	IncludeUnaffordable bool
	Evaluator           Avaliador
	Contexto            domain.ContextoAvaliacao
	// ContextosPorVaga impede que dois slots de mesma posição e funções
	// diferentes sejam reduzidos à mesma comparação.
	ContextosPorVaga map[int]domain.ContextoAvaliacao
}

// EvolutionAcquisition classifica apenas o que os dados permitem afirmar.
// Objetivos são desafios declarados para concluir a evolução, não prova de
// que a EA liberou a evolução por recompensa.
func EvolutionAcquisition(evo domain.Evolution) string {
	if evo.CoinCost > 0 {
		return "moedas"
	}
	if evo.PointCost > 0 {
		return "pontos"
	}
	for _, level := range evo.Levels {
		if len(level.Objectives) > 0 {
			return "objetivos"
		}
	}
	return "origem não identificada"
}

// FindEvolutions cruza as evoluções ativas com o seu elenco e devolve só
// as combinações que melhoram alguma coisa de verdade.
func FindEvolutions(club domain.Club, evos []domain.Evolution, budget int) []EvoMatch {
	return FindEvolutionsWithOptions(club, evos, EvolutionOptions{Budget: budget})
}

// FindEvolutionsWithOptions é a versão usada pelo job: mantém metas fora do
// bolso visíveis no painel e concentra a análise nas cartas relevantes.
func FindEvolutionsWithOptions(club domain.Club, evos []domain.Evolution, options EvolutionOptions) []EvoMatch {
	var out []EvoMatch

	for _, evo := range evos {
		if evo.CoinCost > options.Budget && !options.IncludeUnaffordable {
			continue
		}
		for _, cp := range club.Players {
			if options.MinRating > 0 && cp.Rating < options.MinRating {
				continue
			}
			if !cp.Evolvable() {
				continue
			}
			if !Eligible(cp.Player, evo) {
				continue
			}

			result := evo.Apply(cp.Player)

			// Mede o ganho na melhor função possível para o jogador,
			// já considerando posições que a evolução destrava.
			bestSlot, before, after, beforeEvaluation, afterEvaluation, available := bestEvolutionGainForOptions(club, cp.Player, result, options)
			if !available {
				continue
			}
			gain := after - before
			if gain <= 0.5 {
				continue
			}

			m := EvoMatch{
				Evolution:        evo,
				Player:           cp,
				Slot:             bestSlot,
				Before:           before,
				After:            after,
				Gain:             gain,
				Result:           result,
				Cost:             evo.CoinCost,
				Affordable:       evo.CoinCost <= options.Budget,
				Acquisition:      EvolutionAcquisition(evo),
				Highlights:       evoHighlights(cp.Player, result, evo),
				BeforeEvaluation: beforeEvaluation,
				AfterEvaluation:  afterEvaluation,
			}
			compararEvolucaoComXI(&m, club, cp, result, options)
			out = append(out, m)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].BeatsStarter != out[j].BeatsStarter {
			return out[i].BeatsStarter
		}
		if out[i].ImprovesStarter != out[j].ImprovesStarter {
			return out[i].ImprovesStarter
		}
		if out[i].ReplacementGain != out[j].ReplacementGain {
			return out[i].ReplacementGain > out[j].ReplacementGain
		}
		return out[i].Gain > out[j].Gain
	})
	return out
}

func bestEvolutionGainForOptions(club domain.Club, before, after domain.Player, options EvolutionOptions) (domain.Position, float64, float64, domain.AvaliacaoCarta, domain.AvaliacaoCarta, bool) {
	var bestSlot domain.Position
	var bestBefore, bestAfter float64
	var bestBeforeEvaluation, bestAfterEvaluation domain.AvaliacaoCarta
	found := false
	for _, slot := range club.Squad.Starters {
		if _, specific := options.ContextosPorVaga[slot.Index]; !specific || !after.PlaysAt(slot.Position) {
			continue
		}
		ctx := contextoDaVaga(options.Contexto, options.ContextosPorVaga, slot)
		beforeEvaluation := avaliarOpcional(before, slot.Position, options.Evaluator, ctx)
		afterEvaluation := avaliarOpcional(after, slot.Position, options.Evaluator, ctx)
		if !beforeEvaluation.Disponivel || !afterEvaluation.Disponivel {
			continue
		}
		if !found || afterEvaluation.Nota > bestAfter {
			bestSlot, bestBefore, bestAfter = slot.Position, beforeEvaluation.Nota, afterEvaluation.Nota
			bestBeforeEvaluation, bestAfterEvaluation, found = beforeEvaluation, afterEvaluation, true
		}
	}
	if found {
		return bestSlot, bestBefore, bestAfter, bestBeforeEvaluation, bestAfterEvaluation, true
	}
	return bestSlotGainWithEvaluator(before, after, options.Evaluator, options.Contexto)
}

// compararEvolucaoComXI identifica o ocupante exato que a carta final
// enfrentaria. Se a própria cópia já é titular, descreve uma melhora daquele
// titular; uma versão do mesmo atleta só pode disputar a vaga dele para não
// sugerir duas versões ilegais no XI.
func compararEvolucaoComXI(match *EvoMatch, club domain.Club, original domain.ClubPlayer, result domain.Player, options EvolutionOptions) {
	type comparison struct {
		slot      domain.SquadSlot
		current   domain.ClubPlayer
		currentEv domain.AvaliacaoCarta
		finalEv   domain.AvaliacaoCarta
		gain      float64
		sameCopy  bool
	}
	var restricted *domain.SquadSlot
	// A cópia física tem precedência sobre outra versão do mesmo atleta.
	for pass := 0; pass < 2 && restricted == nil; pass++ {
		for i := range club.Squad.Starters {
			slot := &club.Squad.Starters[i]
			if !result.PlaysAt(slot.Position) {
				continue
			}
			current, ok := club.PlayerForSlot(*slot)
			if !ok {
				continue
			}
			if (pass == 0 && current.IdentityKey() == original.IdentityKey()) ||
				(pass == 1 && current.PlayerKey() == original.PlayerKey()) {
				restricted = slot
				break
			}
		}
	}
	var comparable []comparison
	for _, slot := range club.Squad.Starters {
		if restricted != nil && slot.Index != restricted.Index {
			continue
		}
		if !result.PlaysAt(slot.Position) {
			continue
		}
		current, ok := club.PlayerForSlot(slot)
		if !ok {
			continue
		}
		sameCopy := current.IdentityKey() == original.IdentityKey()
		ctx := contextoDaVaga(options.Contexto, options.ContextosPorVaga, slot)
		currentEv := avaliarOpcional(current.Player, slot.Position, options.Evaluator, ctx)
		finalEv := avaliarOpcional(result, slot.Position, options.Evaluator, ctx)
		if !currentEv.Disponivel || !finalEv.Disponivel {
			continue
		}
		entry := comparison{slot: slot, current: current, currentEv: currentEv, finalEv: finalEv, gain: finalEv.Nota - currentEv.Nota, sameCopy: sameCopy}
		comparable = append(comparable, entry)
	}
	if len(comparable) == 0 {
		return
	}
	best := comparable[0]
	for _, candidate := range comparable[1:] {
		if candidate.gain > best.gain {
			best = candidate
		}
	}
	current := best.current
	match.Index = best.slot.Index
	match.Slot = best.slot.Position
	match.HasComparison = true
	match.Current = &current
	match.ReplacementGain = best.gain
	match.CurrentEvaluation = best.currentEv
	match.CandidateEvaluation = best.finalEv
	match.ImprovesStarter = best.sameCopy && best.gain > 0
	match.BeatsStarter = !best.sameCopy && best.gain > 0
}

// bestSlotGain procura em que posição a carta evoluída rende mais.
func bestSlotGain(before, after domain.Player) (domain.Position, float64, float64) {
	slot, b, a, _, _, _ := bestSlotGainWithEvaluator(before, after, nil, domain.ContextoAvaliacao{})
	return slot, b, a
}

func bestSlotGainWithEvaluator(before, after domain.Player, evaluator Avaliador, ctx domain.ContextoAvaliacao) (domain.Position, float64, float64, domain.AvaliacaoCarta, domain.AvaliacaoCarta, bool) {
	var bestSlot domain.Position
	var bestBefore, bestAfter float64
	var bestBeforeEvaluation, bestAfterEvaluation domain.AvaliacaoCarta
	first := true

	// Considera a posição natural, as alternativas atuais e as que a
	// evolução acrescenta.
	slots := map[domain.Position]bool{after.Position: true}
	for _, p := range after.AltPositions {
		slots[p] = true
	}

	for slot := range slots {
		aEvaluation := avaliarOpcional(after, slot, evaluator, ctx)
		bEvaluation := avaliarOpcional(before, slot, evaluator, ctx)
		if !aEvaluation.Disponivel || !bEvaluation.Disponivel {
			continue
		}
		a, b := aEvaluation.Nota, bEvaluation.Nota
		if first || a > bestAfter {
			bestSlot, bestBefore, bestAfter, bestBeforeEvaluation, bestAfterEvaluation, first = slot, b, a, bEvaluation, aEvaluation, false
		}
	}
	return bestSlot, bestBefore, bestAfter, bestBeforeEvaluation, bestAfterEvaluation, !first
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
