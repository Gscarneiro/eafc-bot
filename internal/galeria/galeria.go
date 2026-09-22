// Package galeria calcula previsoes da FUT Gallery a partir da colecao
// observada. A nota Gallery e deliberadamente separada de GG Rating e das
// notas de mercado: e uma soma de Item Scores com bonus de tags.
package galeria

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Grade e a letra oficial de um conjunto.
type Grade string

const (
	GradeNone Grade = ""
	GradeD    Grade = "D"
	GradeC    Grade = "C"
	GradeB    Grade = "B"
	GradeA    Grade = "A"
	GradeS    Grade = "S"
)

func (g Grade) Rank() int {
	switch g {
	case GradeD:
		return 1
	case GradeC:
		return 2
	case GradeB:
		return 3
	case GradeA:
		return 4
	case GradeS:
		return 5
	default:
		return 0
	}
}
func gradeFor(score int, thresholds map[Grade]int) Grade {
	best := GradeNone
	if len(thresholds) == 0 {
		return best
	}
	for _, g := range []Grade{GradeD, GradeC, GradeB, GradeA, GradeS} {
		threshold, ok := thresholds[g]
		if ok && threshold > 0 && score >= threshold && g.Rank() > best.Rank() {
			best = g
		}
	}
	return best
}

// Card e uma carta que passou pelo clube. Historical e true quando ela nao
// esta no retrato atual, mas continua valida para a Gallery segundo a EA.
type Card struct {
	ID               int64     `json:"id"`
	ClubItemID       string    `json:"club_item_id,omitempty"`
	PlayerID         int64     `json:"player_id,omitempty"`
	OriginalPlayerID int64     `json:"original_player_id,omitempty"`
	Name             string    `json:"name"`
	Version          string    `json:"version,omitempty"`
	Rating           int       `json:"rating,omitempty"`
	Club             string    `json:"club,omitempty"`
	League           string    `json:"league,omitempty"`
	Nation           string    `json:"nation,omitempty"`
	Rarity           string    `json:"rarity,omitempty"`
	NationID         int64     `json:"nation_id,omitempty"`
	ClubID           int64     `json:"club_id,omitempty"`
	LeagueID         int64     `json:"league_id,omitempty"`
	RarityID         int64     `json:"rarity_id,omitempty"`
	Positions        []string  `json:"positions,omitempty"`
	Position         string    `json:"position,omitempty"`
	WeakFoot         int       `json:"weak_foot,omitempty"`
	SkillMoves       int       `json:"skill_moves,omitempty"`
	ItemScore        int       `json:"item_score"`
	Holographic      bool      `json:"holographic,omitempty"`
	FirstOwner       *bool     `json:"first_owner,omitempty"`
	Loan             *bool     `json:"loan,omitempty"`
	Eligible         *bool     `json:"eligible,omitempty"`
	Historical       bool      `json:"historical,omitempty"`
	Source           string    `json:"source,omitempty"`
	ObservedAt       time.Time `json:"observed_at"`
}

// TagRule e a forma normalizada das regras publicadas pelo FUT.GG.
type TagRule struct {
	ID            string   `json:"id,omitempty"`
	Name          string   `json:"name"`
	Target        string   `json:"target,omitempty"`
	Attribute     string   `json:"attribute,omitempty"`
	Operator      string   `json:"operator,omitempty"`
	Values        []string `json:"values,omitempty"`
	Tiers         []Tier   `json:"tiers,omitempty"`
	BonusType     string   `json:"bonus_type,omitempty"`
	ThresholdType string   `json:"threshold_type,omitempty"`
}
type Tier struct {
	MinItems     int `json:"min_items"`
	BonusPercent int `json:"bonus_percent"`
}
type TagBreakdown struct {
	Name             string  `json:"name"`
	Operator         string  `json:"operator,omitempty"`
	Attribute        string  `json:"attribute,omitempty"`
	MatchedIDs       []int64 `json:"matched_ids,omitempty"`
	MatchedCards     []Pick  `json:"matched_cards,omitempty"`
	MatchedCount     int     `json:"matched_count"`
	MatchedScore     int     `json:"matched_score"`
	UnknownCount     int     `json:"unknown_count,omitempty"`
	UnknownCards     []Pick  `json:"unknown_cards,omitempty"`
	BonusPercent     int     `json:"bonus_percent"`
	BonusPoints      int     `json:"bonus_points"`
	NextMinItems     int     `json:"next_min_items,omitempty"`
	NextBonusPercent int     `json:"next_bonus_percent,omitempty"`
	Pending          bool    `json:"pending,omitempty"`
	Reason           string  `json:"reason,omitempty"`
}
type Set struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Category      string             `json:"category,omitempty"`
	CategoryID    int                `json:"category_id,omitempty"`
	BadgeURL      string             `json:"badge_url,omitempty"`
	TeamID        int64              `json:"team_id,omitempty"`
	URL           string             `json:"url,omitempty"`
	RequiredCards int                `json:"required_cards"`
	Thresholds    map[Grade]int      `json:"thresholds"`
	Rewards       map[Grade][]Reward `json:"rewards,omitempty"`
	Rules         []TagRule          `json:"rules,omitempty"`
	PoolSize      int                `json:"pool_size,omitempty"`
	PoolTruncated bool               `json:"pool_truncated,omitempty"`
	UpdatedAt     time.Time          `json:"updated_at,omitempty"`
	EligibleIDs   []int64            `json:"eligible_ids,omitempty"`
}

// Reward preserva a recompensa publicada, inclusive quantidade e imagem. A
// Gallery registra a nota informada pela pessoa, mas n\u00e3o presume que o item
// tenha sido resgatado dentro do jogo.
type Reward struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Label    string `json:"label,omitempty"`
	Value    int    `json:"value,omitempty"`
	Count    int    `json:"count,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// UnmarshalJSON aceita o formato antigo, que guardava somente o rótulo da
// recompensa. Isso preserva o catálogo já salvo até a próxima sincronização
// enriquecer os mesmos registros com o contrato completo.
func (r *Reward) UnmarshalJSON(data []byte) error {
	var label string
	if err := json.Unmarshal(data, &label); err == nil {
		r.Label = label
		return nil
	}
	type rawReward Reward
	var raw rawReward
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = Reward(raw)
	return nil
}

type Completion struct {
	SetID       string    `json:"set_id"`
	Grade       Grade     `json:"grade"`
	Score       int       `json:"score,omitempty"`
	CompletedAt time.Time `json:"completed_at"`
	Notes       string    `json:"notes,omitempty"`
}

type CollectionOverride struct {
	CardID           int64     `json:"card_id"`
	FirstOwner       *bool     `json:"first_owner,omitempty"`
	Loan             *bool     `json:"loan,omitempty"`
	Eligible         *bool     `json:"eligible,omitempty"`
	ItemScore        *int      `json:"item_score,omitempty"`
	OriginalPlayerID *int64    `json:"original_player_id,omitempty"`
	Source           string    `json:"source,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type EvaluationStatus string

const (
	StatusUnavailable EvaluationStatus = "unavailable"
	StatusMissing     EvaluationStatus = "missing"
	StatusAvailable   EvaluationStatus = "available"
	StatusIncomplete  EvaluationStatus = "incomplete"
	StatusFinal       EvaluationStatus = "final"
)

type Pick struct {
	CardID    int64  `json:"card_id"`
	Name      string `json:"name"`
	ItemScore int    `json:"item_score"`
	Bonus     int    `json:"bonus,omitempty"`
	Reason    string `json:"reason,omitempty"`
}
type Evaluation struct {
	SetID         string           `json:"set_id"`
	Status        EvaluationStatus `json:"status"`
	Required      int              `json:"required"`
	Filled        int              `json:"filled"`
	Score         int              `json:"score"`
	BaseScore     int              `json:"base_score"`
	BonusScore    int              `json:"bonus_score"`
	Tags          []TagBreakdown   `json:"tags,omitempty"`
	Grade         Grade            `json:"grade"`
	NextGrade     Grade            `json:"next_grade,omitempty"`
	NextThreshold int              `json:"next_threshold,omitempty"`
	Picks         []Pick           `json:"picks,omitempty"`
	Missing       []string         `json:"missing,omitempty"`
	Warnings      []string         `json:"warnings,omitempty"`
	Coverage      string           `json:"coverage,omitempty"`
	States        int              `json:"states,omitempty"`
	MaxProven     bool             `json:"max_proven,omitempty"`
	ComputedAt    time.Time        `json:"computed_at"`
	InputHash     string           `json:"input_hash,omitempty"`
	EngineVersion string           `json:"engine_version,omitempty"`
}

type Record struct {
	Set        Set         `json:"set"`
	Evaluation Evaluation  `json:"evaluation"`
	Completion *Completion `json:"completion,omitempty"`
}

type Input struct {
	Sets        []Set
	Cards       []Card
	Overrides   []CollectionOverride
	Completions map[string]Completion
	Now         time.Time
	MaxStates   int
}

// ApplyOverrides devolve os dados efetivamente usados pelo motor. A API usa
// esta mesma função para não exibir "desconhecido" após uma correção manual.
func ApplyOverrides(cards []Card, overrides []CollectionOverride) []Card {
	by := map[int64]CollectionOverride{}
	for _, o := range overrides {
		by[o.CardID] = o
	}
	out := make([]Card, len(cards))
	copy(out, cards)
	for i := range out {
		if o, ok := by[out[i].ID]; ok {
			if o.FirstOwner != nil {
				out[i].FirstOwner = o.FirstOwner
			}
			if o.Loan != nil {
				out[i].Loan = o.Loan
			}
			if o.Eligible != nil {
				out[i].Eligible = o.Eligible
			}
			if o.ItemScore != nil {
				out[i].ItemScore = *o.ItemScore
			}
			if o.OriginalPlayerID != nil {
				out[i].OriginalPlayerID = *o.OriginalPlayerID
			}
			if o.Source != "" {
				out[i].Source = o.Source
			}
		}
	}
	return out
}

const engineVersion = "futgg-game-floor-v2"

func attr(c Card, a string) string {
	switch strings.ToUpper(a) {
	case "NATION":
		if c.NationID != 0 {
			return strconv.FormatInt(c.NationID, 10)
		}
		return c.Nation
	case "CLUB":
		if c.ClubID != 0 {
			return strconv.FormatInt(c.ClubID, 10)
		}
		return c.Club
	case "LEAGUEID", "LEAGUE":
		if c.LeagueID != 0 {
			return strconv.FormatInt(c.LeagueID, 10)
		}
		return c.League
	case "LEVEL":
		switch {
		case c.Rating >= 75:
			return "gold"
		case c.Rating >= 65:
			return "silver"
		case c.Rating > 0:
			return "bronze"
		}
		return ""
	case "RARE":
		if c.RarityID != 0 {
			return strconv.FormatInt(c.RarityID, 10)
		}
		return c.Rarity
	case "FIRST_OWNED":
		if c.FirstOwner != nil && *c.FirstOwner {
			return "true"
		}
		return "false"
	case "HYPER_COSMETIC_TYPE":
		if c.Holographic {
			return "HOLOGRAPHIC"
		}
		return ""
	case "POSSIBLE_POSITIONS":
		if len(c.Positions) > 0 {
			return strings.Join(c.Positions, ",")
		}
		return c.Position
	case "BASE_DEF_ID", "PLAYER_EA_ID":
		if c.OriginalPlayerID != 0 {
			return fmt.Sprint(c.OriginalPlayerID)
		}
		return fmt.Sprint(c.PlayerID)
	case "WEAK_FOOT":
		return fmt.Sprint(c.WeakFoot)
	case "SKILL_MOVES":
		return fmt.Sprint(c.SkillMoves)
	default:
		return ""
	}
}
func attrMatches(c Card, attribute string, values []string) bool {
	attribute = strings.ToUpper(attribute)
	if attribute == "FIRST_OWNED" {
		// O cat\u00e1logo usa values:["1"], mas o dado \u00e9 booleano. Apenas a
		// confirma\u00e7\u00e3o expl\u00edcita concede o b\u00f4nus.
		return c.FirstOwner != nil && *c.FirstOwner
	}
	if attribute == "POSSIBLE_POSITIONS" {
		positions := c.Positions
		if len(positions) == 0 && c.Position != "" {
			positions = []string{c.Position}
		}
		for _, p := range positions {
			for _, v := range values {
				if strings.EqualFold(p, v) {
					return true
				}
			}
		}
		return false
	}
	if attribute == "HYPER_COSMETIC_TYPE" {
		return c.Holographic
	}
	if attribute == "WEAK_FOOT" {
		if len(values) == 0 {
			return false
		}
		n, _ := strconv.Atoi(values[0])
		return c.WeakFoot >= n
	}
	if attribute == "SKILL_MOVES" {
		if len(values) == 0 {
			return false
		}
		n, _ := strconv.Atoi(values[0])
		return c.SkillMoves >= n+1
	}
	v := attr(c, attribute)
	for _, x := range values {
		if strings.EqualFold(v, x) {
			return true
		}
	}
	return len(values) == 0 && v != ""
}
func matches(c Card, rule TagRule) bool {
	if c.Loan != nil && *c.Loan {
		return false
	}
	return attrMatches(c, rule.Attribute, rule.Values)
}

func groupKey(c Card, attribute string) string {
	key := attr(c, attribute)
	if key == "0" {
		return ""
	}
	return key
}

func scoreCards(cards []Card) int {
	total := 0
	for _, c := range cards {
		total += c.ItemScore
	}
	return total
}

func bestCard(cards []Card) Card {
	best := cards[0]
	for _, c := range cards[1:] {
		if c.ItemScore > best.ItemScore || (c.ItemScore == best.ItemScore && c.ID < best.ID) {
			best = c
		}
	}
	return best
}

func sortedGroupKeys(groups map[string][]Card) []string {
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func bonus(picks []Card, rules []TagRule) (int, []TagBreakdown, []string, bool) {
	total := 0
	breakdown := make([]TagBreakdown, 0, len(rules))
	var notes []string
	pending := false
	for _, r := range rules {
		if r.BonusType != "" && !strings.EqualFold(r.BonusType, "ITEM_SCORE_PERCENTAGE") {
			pending = true
			breakdown = append(breakdown, TagBreakdown{Name: r.Name, Operator: r.Operator, Attribute: r.Attribute, Pending: true, Reason: "tipo de bônus não suportado"})
			continue
		}
		matched := make([]Card, 0, len(picks))
		unknown := make([]Card, 0)
		groups := map[string][]Card{}
		isGrouping := strings.EqualFold(r.Operator, "COUNT_DIFF") || strings.EqualFold(r.Operator, "MAX_COUNT_ALL_SAME")
		for _, c := range picks {
			if c.Loan != nil && *c.Loan {
				continue
			}
			if tagAttributeUnknown(c, r.Attribute) {
				unknown = append(unknown, c)
			}
			if isGrouping {
				// Nas regras agrupadas values:["0"] \u00e9 apenas um marcador do
				// contrato, n\u00e3o um filtro para liga, clube ou na\u00e7\u00e3o.
				if key := groupKey(c, r.Attribute); key != "" {
					groups[key] = append(groups[key], c)
				}
				continue
			}
			if matches(c, r) {
				matched = append(matched, c)
			}
		}
		count := len(matched)
		sum := scoreCards(matched)
		switch strings.ToUpper(r.Operator) {
		case "COUNT", "COUNT_ANY", "MIN_COUNT":
		case "COUNT_DIFF":
			matched = matched[:0]
			for _, key := range sortedGroupKeys(groups) {
				matched = append(matched, bestCard(groups[key]))
			}
			count, sum = len(matched), scoreCards(matched)
		case "MAX_COUNT_ALL_SAME":
			matched = matched[:0]
			bestBonus, bestCount := -1, 0
			for _, key := range sortedGroupKeys(groups) {
				group := groups[key]
				groupSum := scoreCards(group)
				candidate := floorBonus(tierFor(len(group), r.Tiers), groupSum)
				if candidate > bestBonus || (candidate == bestBonus && len(group) > bestCount) {
					bestBonus, bestCount = candidate, len(group)
					matched = append(matched[:0], group...)
				}
			}
			count, sum = len(matched), scoreCards(matched)
		case "":
			// Registros antigos gravados antes de o catálogo expor `type`
			// eram regras simples de contagem. Mantê-los como COUNT preserva
			// esses dados; regras novas desconhecidas continuam pendentes.
		default:
			pending = true
			breakdown = append(breakdown, TagBreakdown{Name: r.Name, Operator: r.Operator, Attribute: r.Attribute, Pending: true, Reason: "operador não suportado"})
			continue
		}
		tier := tierFor(count, r.Tiers)
		bd := TagBreakdown{Name: r.Name, Operator: r.Operator, Attribute: r.Attribute, MatchedCount: count, MatchedScore: sum, BonusPercent: tier, UnknownCount: len(unknown)}
		for _, c := range matched {
			bd.MatchedIDs = append(bd.MatchedIDs, c.ID)
			bd.MatchedCards = append(bd.MatchedCards, Pick{CardID: c.ID, Name: c.Name, ItemScore: c.ItemScore})
		}
		for _, c := range unknown {
			bd.UnknownCards = append(bd.UnknownCards, Pick{CardID: c.ID, Name: c.Name, ItemScore: c.ItemScore})
		}
		if len(unknown) > 0 {
			pending = true
			bd.Pending = true
			bd.Reason = fmt.Sprintf("%d carta(s) sem %s confirmado", len(unknown), strings.ToLower(strings.ReplaceAll(r.Attribute, "_", " ")))
		}
		for _, t := range r.Tiers {
			if t.MinItems > count && (bd.NextMinItems == 0 || t.MinItems < bd.NextMinItems) {
				bd.NextMinItems = t.MinItems
				bd.NextBonusPercent = t.BonusPercent
			}
		}
		if tier > 0 {
			// A interface do jogo confirma a divisão inteira direta em valores
			// exatos (por exemplo 500% de 12.910 = 64.550). O bundle público
			// contém um -1 divergente; a previsão reproduz o jogo, não esse artefato.
			b := floorBonus(tier, sum)
			bd.BonusPoints = b
			total += b
			notes = append(notes, fmt.Sprintf("%s: %d itens +%d%% (%d)", r.Name, count, tier, b))
		}
		breakdown = append(breakdown, bd)
	}
	return total, breakdown, notes, pending
}

func tierFor(count int, tiers []Tier) int {
	best := 0
	for _, t := range tiers {
		if count >= t.MinItems {
			best = t.BonusPercent
		}
	}
	return best
}
func floorBonus(percent, subtotal int) int {
	if percent <= 0 || subtotal <= 0 {
		return 0
	}
	return int(int64(percent) * int64(subtotal) / 100)
}

func tagAttributeUnknown(c Card, attribute string) bool {
	switch strings.ToUpper(attribute) {
	case "FIRST_OWNED":
		return c.FirstOwner == nil
	case "WEAK_FOOT":
		return c.WeakFoot == 0
	case "SKILL_MOVES":
		return c.SkillMoves == 0
	default:
		return false
	}
}

// Evaluate encontra uma combinacao valida por busca exaustiva limitada. O
// primeiro resultado usa as cartas de maior score; para conjuntos pequenos a
// busca completa prova o melhor resultado encontrado.
func Evaluate(in Input) []Record {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	cards := ApplyOverrides(in.Cards, in.Overrides)
	if in.MaxStates <= 0 {
		in.MaxStates = 100000
	}
	out := make([]Record, 0, len(in.Sets))
	for _, set := range in.Sets {
		rec := Record{Set: set}
		if c, ok := in.Completions[set.ID]; ok && c.Grade == GradeS {
			rec.Completion = &c
			rec.Evaluation = Evaluation{SetID: set.ID, Status: StatusFinal, Grade: GradeS, Score: c.Score, ComputedAt: now, InputHash: evaluationHash(set, nil, c), EngineVersion: engineVersion}
			out = append(out, rec)
			continue
		}
		if set.RequiredCards <= 0 || !validThresholds(set.Thresholds) {
			rec.Evaluation = Evaluation{SetID: set.ID, Status: StatusUnavailable, Required: set.RequiredCards, Warnings: []string{"catálogo sem vagas ou limites D–S completos"}, Coverage: coverage(set), ComputedAt: now, InputHash: evaluationHash(set, nil, in.Completions[set.ID]), EngineVersion: engineVersion}
			if c, ok := in.Completions[set.ID]; ok {
				rec.Completion = &c
			}
			out = append(out, rec)
			continue
		}
		candidates := cardsForSet(cards, set)
		if set.RequiredCards <= 0 {
			set.RequiredCards = len(candidates)
			rec.Set = set
		}
		if len(candidates) < set.RequiredCards {
			rec.Evaluation = Evaluation{SetID: set.ID, Status: StatusMissing, Required: set.RequiredCards, Filled: len(candidates), Missing: []string{fmt.Sprintf("%d cartas", set.RequiredCards-len(candidates))}, Coverage: coverage(set), ComputedAt: now, InputHash: evaluationHash(set, candidates, in.Completions[set.ID]), EngineVersion: engineVersion}
			if c, ok := in.Completions[set.ID]; ok {
				rec.Completion = &c
			}
			out = append(out, rec)
			continue
		}
		best, states, limited := search(candidates, set, in.MaxStates)
		baseScore, bonusScore, tags, pending := scoreBreakdown(best, set)
		score := baseScore + bonusScore
		grade := gradeFor(score, set.Thresholds)
		next, threshold := nextGrade(score, set.Thresholds)
		// Uma combina\u00e7\u00e3o formada apenas por cartas confirmadas continua v\u00e1lida
		// mesmo quando n\u00e3o \u00e9 poss\u00edvel provar o m\u00e1ximo global do pool.
		ev := Evaluation{SetID: set.ID, Status: StatusAvailable, Required: set.RequiredCards, Filled: set.RequiredCards, Score: score, BaseScore: baseScore, BonusScore: bonusScore, Tags: tags, Grade: grade, NextGrade: next, NextThreshold: threshold, States: states, MaxProven: !set.PoolTruncated && !pending && !limited, Coverage: coverage(set), ComputedAt: now, InputHash: evaluationHash(set, candidates, in.Completions[set.ID]), EngineVersion: engineVersion}
		if pending {
			ev.Warnings = append(ev.Warnings, "há dados ou regras de bônus pendentes; veja o detalhamento das tags")
		}
		if limited {
			ev.Warnings = append(ev.Warnings, "limite de busca atingido; resultado parcial")
		}
		for _, c := range best {
			ev.Picks = append(ev.Picks, Pick{CardID: c.ID, Name: c.Name, ItemScore: c.ItemScore})
		}
		if set.PoolTruncated {
			ev.Warnings = append(ev.Warnings, "pool de cartas truncado; a ausência não prova inelegibilidade")
		}
		if c, ok := in.Completions[set.ID]; ok {
			rec.Completion = &c
			if grade.Rank() <= c.Grade.Rank() {
				ev.Status = StatusAvailable
			}
		}
		rec.Evaluation = ev
		out = append(out, rec)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Set.Name < out[j].Set.Name })
	return out
}
func validThresholds(t map[Grade]int) bool {
	for _, g := range []Grade{GradeD, GradeC, GradeB, GradeA, GradeS} {
		if t[g] <= 0 {
			return false
		}
	}
	return true
}
func coverage(s Set) string {
	if s.PoolTruncated {
		return "incompleta"
	}
	return "confirmada"
}
func cardsForSet(cards []Card, s Set) []Card {
	out := make([]Card, 0)
	allowed := make(map[int64]bool, len(s.EligibleIDs))
	for _, id := range s.EligibleIDs {
		allowed[id] = true
	}
	for _, c := range cards {
		if c.ItemScore <= 0 {
			continue
		}
		if len(allowed) > 0 && !allowed[c.ID] {
			continue
		}
		if len(allowed) == 0 && s.PoolTruncated && (c.Eligible == nil || !*c.Eligible) {
			continue
		}
		if c.Loan != nil && *c.Loan {
			continue
		}
		if c.Eligible != nil && !*c.Eligible {
			continue
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ItemScore > out[j].ItemScore })
	return out
}
func search(c []Card, s Set, max int) ([]Card, int, bool) {
	best := initialCombination(c, s.RequiredCards)
	bestVal := -1
	if len(best) == s.RequiredCards {
		bestVal = bestScore(best, s)
	}
	states := 0
	limited := false
	var walk func(int, []Card)
	walk = func(start int, p []Card) {
		if states >= max {
			limited = true
			return
		}
		states++
		remaining := s.RequiredCards - len(p)
		if len(c)-start < remaining {
			return
		}
		if len(p) == s.RequiredCards {
			v := bestScore(p, s)
			if v > bestVal {
				bestVal = v
				best = append([]Card(nil), p...)
			}
			return
		}
		if bestVal >= 0 && upperBound(c[start:], remaining, p, s.Rules) <= bestVal {
			return
		}
		for i := start; i < len(c); i++ {
			if containsIdentity(p, c[i]) {
				continue
			}
			walk(i+1, append(p, c[i]))
		}
	}
	walk(0, nil)
	return best, states, limited
}

func cardIdentity(c Card) string {
	if c.ClubItemID != "" {
		return "club:" + c.ClubItemID
	}
	return fmt.Sprintf("card:%d", c.ID)
}

func containsIdentity(picks []Card, candidate Card) bool {
	identity := cardIdentity(candidate)
	for _, pick := range picks {
		if cardIdentity(pick) == identity {
			return true
		}
	}
	return false
}

func initialCombination(cards []Card, required int) []Card {
	result := make([]Card, 0, required)
	for _, card := range cards {
		if !containsIdentity(result, card) {
			result = append(result, card)
		}
		if len(result) == required {
			return result
		}
	}
	return nil
}

// upperBound é deliberadamente conservador: soma as maiores cartas restantes
// e aplica, para cada regra, a maior porcentagem sobre todo esse subtotal. Ele
// poda ramos que não podem superar a solução atual sem transformar a busca em
// uma afirmação de máximo quando os dados estão incompletos.
func upperBound(remaining []Card, slots int, picked []Card, rules []TagRule) int {
	base := 0
	for _, c := range picked {
		base += c.ItemScore
	}
	scores := make([]int, len(remaining))
	for i, c := range remaining {
		scores[i] = c.ItemScore
	}
	sort.Sort(sort.Reverse(sort.IntSlice(scores)))
	for i := 0; i < slots && i < len(scores); i++ {
		base += scores[i]
	}
	all := base
	for _, c := range remaining {
		all += c.ItemScore
	}
	bonus := 0
	for _, r := range rules {
		maxPct := 0
		for _, tier := range r.Tiers {
			if tier.BonusPercent > maxPct {
				maxPct = tier.BonusPercent
			}
		}
		bonus += floorBonus(maxPct, all)
	}
	return base + bonus
}
func bestScore(p []Card, s Set) int {
	base, extra, _, _ := scoreBreakdown(p, s)
	return base + extra
}
func scoreBreakdown(p []Card, s Set) (int, int, []TagBreakdown, bool) {
	base := 0
	for _, c := range p {
		base += c.ItemScore
	}
	extra, tags, _, pending := bonus(p, s.Rules)
	return base, extra, tags, pending
}
func nextGrade(score int, t map[Grade]int) (Grade, int) {
	for _, g := range []Grade{GradeD, GradeC, GradeB, GradeA, GradeS} {
		threshold, ok := t[g]
		if ok && threshold > 0 && score < threshold {
			return g, t[g]
		}
	}
	return GradeNone, 0
}

func evaluationHash(set Set, cards []Card, completion Completion) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%d|%v|", engineVersion, set.ID, set.RequiredCards, set.Thresholds)
	for _, r := range set.Rules {
		fmt.Fprintf(h, "%s|%s|%s|%v|%v|", r.Name, r.Attribute, r.Operator, r.Values, r.Tiers)
	}
	for _, c := range cards {
		fmt.Fprintf(h, "%d:%d:%d:%t:%v:%v:%v|", c.ID, c.ItemScore, c.PlayerID, c.Holographic, c.FirstOwner, c.Loan, c.Eligible)
	}
	fmt.Fprintf(h, "completion:%s:%s:%d", completion.SetID, completion.Grade, completion.Score)
	return hex.EncodeToString(h.Sum(nil))
}

func (r Record) Notify() bool {
	if r.Completion != nil && r.Completion.Grade == GradeS {
		return false
	}
	if r.Evaluation.Status != StatusAvailable {
		return false
	}
	if r.Completion == nil {
		return r.Evaluation.Grade.Rank() > 0
	}
	return r.Evaluation.Grade.Rank() > r.Completion.Grade.Rank()
}
