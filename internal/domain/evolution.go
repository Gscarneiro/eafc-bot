package domain

import (
	"strings"
	"time"
)

// EvoRequirement é uma condição de entrada da evolução. O bot precisa
// conseguir avaliar cada uma contra uma carta do clube sem intervenção humana.
type EvoRequirement struct {
	Kind     string   `json:"kind"` // "max_overall", "min_overall", "position", "max_playstyles", "rarity", "max_pace"...
	IntValue int      `json:"int_value"`
	Strings  []string `json:"strings"` // valores aceitos, p.ex. posições ou raridades
	Raw      string   `json:"raw"`     // texto original, para o relatório e para debug
}

// EvoUpgrade é um ganho concedido por um nível da evolução.
type EvoUpgrade struct {
	Kind   string `json:"kind"` // "overall", "attribute", "sub_attribute", "playstyle", "position", "skill_moves", "weak_foot"
	Attr   string `json:"attr"` // "pac", "sho", ... quando Kind == "attribute"
	Amount int    `json:"amount"`
	// MaxValue é o teto que o fut.gg declara para o ganho ("+10, até 96").
	// Zero significa "sem teto declarado" — Apply() cai no clamp padrão.
	MaxValue  int       `json:"max_value,omitempty"`
	PlayStyle PlayStyle `json:"play_style"`
	Position  Position  `json:"position"`
	Raw       string    `json:"raw"`
}

// EvolutionClassification preserva a taxonomia que o próprio catálogo do
// fut.gg publica. Category é a seção exclusiva usada pelo jogo; Origin é um
// detalhe secundário de obtenção (objetivo, SBC, passe etc.). Não inferimos
// categoria a partir do upgrade: uma recompensa que concede PlayStyle+ ainda
// continua em Rewards.
type EvolutionClassification struct {
	Category       string   `json:"category"`
	CategoryLabel  string   `json:"category_label"`
	CategorySlug   string   `json:"category_slug,omitempty"`
	CategorySource string   `json:"category_source"`
	Origin         string   `json:"origin,omitempty"`
	OriginLabel    string   `json:"origin_label,omitempty"`
	OriginSource   string   `json:"origin_source,omitempty"`
	Evidence       []string `json:"evidence,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

// EvoLevel é um nível da evolução: custa objetivos e entrega upgrades.
type EvoLevel struct {
	Number     int          `json:"number"`
	Upgrades   []EvoUpgrade `json:"upgrades"`
	Objectives []string     `json:"objectives"` // "Marque 5 gols em Rivals"...
}

// Evolution é uma evolução ativa na loja.
type Evolution struct {
	ID                       string           `json:"id"`
	Slug                     string           `json:"slug"`
	Name                     string           `json:"name"`
	Description              string           `json:"description"`
	CoinCost                 int              `json:"coin_cost"`
	PointCost                int              `json:"point_cost"`
	TokenCost                int              `json:"token_cost,omitempty"`
	EventTokenID             string           `json:"event_token_id,omitempty"`
	RepeatabilityCount       int              `json:"repeatability_count,omitempty"`
	Repeatable               bool             `json:"repeatable,omitempty"`
	AllowedPriorEvolutionIDs []string         `json:"allowed_prior_evolution_ids,omitempty"`
	Requirements             []EvoRequirement `json:"requirements"`
	Levels                   []EvoLevel       `json:"levels"`
	ExpiresAt                time.Time        `json:"expires_at"`
	EndSubmissionAt          time.Time        `json:"end_submission_at,omitempty"`
	Cycle                    string           `json:"cycle"`
	URL                      string           `json:"url"`

	// Metadados publicados no objeto bruto. Eles ficam no snapshot para
	// que a classificação e a origem possam ser auditadas sem rede.
	CategoryID             string                  `json:"category_id,omitempty"`
	CategoryName           string                  `json:"category_name,omitempty"`
	CategorySlug           string                  `json:"category_slug,omitempty"`
	SBCName                string                  `json:"sbc_name,omitempty"`
	SBCURL                 string                  `json:"sbc_url,omitempty"`
	SBCSlug                string                  `json:"sbc_slug,omitempty"`
	ObjectiveGroupName     string                  `json:"objective_group_name,omitempty"`
	ObjectiveGroupURL      string                  `json:"objective_group_url,omitempty"`
	ObjectiveGroupSlug     string                  `json:"objective_group_slug,omitempty"`
	TimedCustom            bool                    `json:"timed_custom,omitempty"`
	CustomUnlockable       bool                    `json:"custom_unlockable,omitempty"`
	IsTimed                bool                    `json:"is_timed,omitempty"`
	TotalTrainingTime      int                     `json:"total_training_time,omitempty"`
	HideFromFilters        bool                    `json:"hide_from_filters,omitempty"`
	ExcludeFromActivePaths bool                    `json:"exclude_from_active_paths,omitempty"`
	IsGKEvolution          bool                    `json:"is_gk_evolution,omitempty"`
	IsNotInEvoLab          bool                    `json:"is_not_in_evo_lab,omitempty"`
	DoesNotUpgradePlayer   bool                    `json:"does_not_upgrade_player,omitempty"`
	IsRewardEvolution      bool                    `json:"is_reward_evolution,omitempty"`
	Classification         EvolutionClassification `json:"classification,omitempty"`
}

const (
	EvolutionCategoryNormal         = "evolutions"
	EvolutionCategoryRewards        = "rewards"
	EvolutionCategoryPlayStyles     = "playstyles"
	EvolutionCategoryPlayStylesPlus = "playstyles_plus"
	EvolutionCategoryRolesPlusPlus  = "roles_plus_plus"
	EvolutionCategoryTrainingCamp   = "training_camp"
	EvolutionCategoryCosmetics      = "cosmetics"
	EvolutionOriginFree             = "free"
	EvolutionOriginPaid             = "paid"
	EvolutionOriginObjective        = "objective"
	EvolutionOriginSBC              = "sbc"
	EvolutionOriginSeasonPass       = "season_pass"
	EvolutionOriginTokenStore       = "token_store"
	EvolutionOriginOtherReward      = "other_reward"
)

// ClassifyEvolution aplica a precedência documentada da taxonomia do jogo.
// Deve ser chamado após o parser carregar os metadados; também serve para
// snapshots antigos que ainda não tinham Classification preenchida.
func (e Evolution) ClassifyEvolution() Evolution {
	if e.Classification.Category != "" {
		// Snapshots gravados antes da taxonomia podem ter só a seção. Completar
		// os campos derivados aqui mantém a leitura compatível sem reclassificar
		// a categoria que já foi confirmada pela fonte.
		if e.Classification.CategoryLabel == "" {
			e.Classification.CategoryLabel = evolutionCategoryLabel(e.Classification.Category)
		}
		if e.Classification.Origin == "" {
			if e.Classification.Category == EvolutionCategoryRewards || e.IsRewardEvolution {
				e.Classification.Origin = evolutionRewardOrigin(e)
				e.Classification.OriginLabel = evolutionRewardOriginLabel(e)
				e.Classification.OriginSource = "futgg:reward_metadata"
			} else if e.CoinCost > 0 || e.PointCost > 0 || e.TokenCost > 0 {
				e.Classification.Origin, e.Classification.OriginLabel = EvolutionOriginPaid, "Comprável"
			} else {
				e.Classification.Origin, e.Classification.OriginLabel = EvolutionOriginFree, "Grátis"
			}
		}
		return e
	}
	classification := EvolutionClassification{}
	categorySlug := strings.ToLower(strings.TrimSpace(e.CategorySlug))
	categoryName := strings.ToLower(strings.TrimSpace(e.CategoryName))
	set := func(key, label, source string) {
		classification.Category = key
		classification.CategoryLabel = label
		classification.CategorySlug = e.CategorySlug
		classification.CategorySource = source
		if source != "" {
			classification.Evidence = append(classification.Evidence, source)
		}
	}

	// Recompensa é uma categoria do jogo e tem precedência sobre o tipo de
	// upgrade (inclusive PlayStyle+). O flag explícito é a evidência mais
	// forte; nunca deixamos um campo de upgrade mover a evolução para Lab.
	if e.IsRewardEvolution {
		set(EvolutionCategoryRewards, "Rewards", "futgg:is_reward_evolution")
		// PlayStyles Lab é uma seção própria. A subdivisão é validada apenas
		// dentro dela pelo upgrade único, nunca usada para mover recompensa.
	} else if categorySlug == "playstyle-lab" || strings.Contains(categoryName, "playstyle lab") {
		plus, normal, other := 0, 0, 0
		for _, up := range e.TotalUpgrades() {
			if up.Kind != "playstyle" {
				continue
			}
			if up.PlayStyle.Plus {
				plus++
			} else {
				normal++
			}
			if up.PlayStyle.Name == "" {
				other++
			}
		}
		switch {
		case plus == 1 && normal == 0 && other == 0:
			set(EvolutionCategoryPlayStylesPlus, "PlayStyles+", "futgg:category/playstyle-lab+upgrade")
		case normal == 1 && plus == 0 && other == 0:
			set(EvolutionCategoryPlayStyles, "PlayStyles", "futgg:category/playstyle-lab+upgrade")
		default:
			set("playstyle_lab", "PlayStyles Lab", "futgg:category")
			classification.Warnings = append(classification.Warnings, "PlayStyles Lab sem upgrade único reconhecido")
		}
	} else if categorySlug != "" {
		switch {
		case strings.Contains(categorySlug, "reward"):
			set(EvolutionCategoryRewards, firstNonEmpty(e.CategoryName, "Rewards"), "futgg:category_slug")
		case strings.Contains(categorySlug, "playstyle") && strings.Contains(categorySlug, "plus"):
			set(EvolutionCategoryPlayStylesPlus, firstNonEmpty(e.CategoryName, "PlayStyles+"), "futgg:category_slug")
		case strings.Contains(categorySlug, "playstyle"):
			set(EvolutionCategoryPlayStyles, firstNonEmpty(e.CategoryName, "PlayStyles"), "futgg:category_slug")
		case strings.Contains(categorySlug, "role"):
			set(EvolutionCategoryRolesPlusPlus, firstNonEmpty(e.CategoryName, "Roles++"), "futgg:category_slug")
		case strings.Contains(categorySlug, "training"):
			set(EvolutionCategoryTrainingCamp, firstNonEmpty(e.CategoryName, "Training Camp"), "futgg:category_slug")
		case strings.Contains(categorySlug, "cosmetic"):
			set(EvolutionCategoryCosmetics, firstNonEmpty(e.CategoryName, "Cosmetics"), "futgg:category_slug")
		default:
			set(categorySlug, firstNonEmpty(e.CategoryName, e.CategorySlug), "futgg:category_slug")
		}
	} else if categoryName != "" {
		switch {
		case strings.Contains(categoryName, "reward"):
			set(EvolutionCategoryRewards, e.CategoryName, "futgg:category_name")
		case strings.Contains(categoryName, "playstyle") && strings.Contains(categoryName, "+"):
			set(EvolutionCategoryPlayStylesPlus, e.CategoryName, "futgg:category_name")
		case strings.Contains(categoryName, "playstyle"):
			set(EvolutionCategoryPlayStyles, e.CategoryName, "futgg:category_name")
		case strings.Contains(categoryName, "role"):
			set(EvolutionCategoryRolesPlusPlus, e.CategoryName, "futgg:category_name")
		case strings.Contains(categoryName, "training"):
			set(EvolutionCategoryTrainingCamp, e.CategoryName, "futgg:category_name")
		case strings.Contains(categoryName, "cosmetic"):
			set(EvolutionCategoryCosmetics, e.CategoryName, "futgg:category_name")
		default:
			set(normalizeCategoryKey(categoryName), e.CategoryName, "futgg:category_name")
		}
	} else if e.IsRewardEvolution || e.ObjectiveGroupName != "" || e.SBCName != "" || e.EventTokenID != "" || strings.Contains(strings.ToLower(e.Name+" "+e.Description), "season pass") {
		// Esses campos são marcadores de origem publicados pelo catálogo. Eles
		// distinguem Rewards mesmo quando uma versão da API omite o flag
		// isRewardEvolution; objetivos internos dos níveis não entram aqui.
		set(EvolutionCategoryRewards, "Rewards", "futgg:reward_metadata")
	} else if e.TotalTrainingTime > 0 {
		set(EvolutionCategoryTrainingCamp, "Training Camp", "futgg:total_training_time")
	} else if e.DoesNotUpgradePlayer {
		set(EvolutionCategoryCosmetics, "Cosmetics", "futgg:does_not_upgrade_player")
	} else if strings.Contains(strings.ToLower(e.Name+" "+e.Description), "training camp") {
		// É uma marca textual explícita da seção; não usamos custo ou
		// tipo de upgrade para adivinhar Training Camp.
		set(EvolutionCategoryTrainingCamp, "Training Camp", "futgg:name_or_description")
	} else {
		set(EvolutionCategoryNormal, "Evoluções", "futgg:default")
	}

	if classification.Category == EvolutionCategoryRewards || e.IsRewardEvolution {
		classification.Origin, classification.OriginLabel, classification.OriginSource = evolutionRewardOrigin(e), evolutionRewardOriginLabel(e), "futgg:reward_metadata"
	} else if e.CoinCost > 0 || e.PointCost > 0 || e.TokenCost > 0 {
		classification.Origin, classification.OriginLabel = EvolutionOriginPaid, "Comprável"
	} else {
		classification.Origin, classification.OriginLabel = EvolutionOriginFree, "Grátis"
	}
	e.Classification = classification
	return e
}

func evolutionCategoryLabel(category string) string {
	switch category {
	case EvolutionCategoryNormal:
		return "Evoluções"
	case EvolutionCategoryRewards:
		return "Rewards"
	case EvolutionCategoryPlayStyles:
		return "PlayStyles"
	case EvolutionCategoryPlayStylesPlus:
		return "PlayStyles+"
	case EvolutionCategoryRolesPlusPlus:
		return "Roles++"
	case EvolutionCategoryTrainingCamp:
		return "Training Camp"
	case EvolutionCategoryCosmetics:
		return "Cosmetics"
	default:
		return category
	}
}

func evolutionRewardOrigin(e Evolution) string {
	switch {
	case e.ObjectiveGroupName != "" || e.ObjectiveGroupSlug != "":
		return EvolutionOriginObjective
	case e.SBCName != "" || e.SBCSlug != "":
		return EvolutionOriginSBC
	case e.EventTokenID != "":
		return EvolutionOriginTokenStore
	case strings.Contains(strings.ToLower(e.Name+" "+e.Description), "season pass"):
		return EvolutionOriginSeasonPass
	default:
		return EvolutionOriginOtherReward
	}
}

func evolutionRewardOriginLabel(e Evolution) string {
	switch evolutionRewardOrigin(e) {
	case EvolutionOriginObjective:
		return "Objetivo"
	case EvolutionOriginSBC:
		return "SBC"
	case EvolutionOriginTokenStore:
		return "Tokens"
	case EvolutionOriginSeasonPass:
		return "Season Pass"
	default:
		return "Outra recompensa"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeCategoryKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	previousUnderscore := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			previousUnderscore = false
		case !previousUnderscore && b.Len() > 0:
			b.WriteByte('_')
			previousUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

// TotalUpgrades achata todos os níveis num único conjunto de ganhos,
// que é o que interessa para estimar a carta final.
func (e Evolution) TotalUpgrades() []EvoUpgrade {
	var all []EvoUpgrade
	for _, lvl := range e.Levels {
		all = append(all, lvl.Upgrades...)
	}
	return all
}

// Apply projeta como a carta fica depois de completar a evolução inteira.
// É uma estimativa: a EA recalcula o overall por fórmula própria, então o
// overall só é confiável quando a evolução declara o ganho explicitamente.
func (e Evolution) Apply(p Player) Player {
	out := p
	out.Attributes = p.Attributes
	out.PlayStyles = append([]PlayStyle(nil), p.PlayStyles...)
	out.AltPositions = append([]Position(nil), p.AltPositions...)
	if p.DetailedAttributes != nil {
		d := *p.DetailedAttributes
		out.DetailedAttributes = &d
	}

	for _, up := range e.TotalUpgrades() {
		switch up.Kind {
		case "overall":
			out.Rating = capAt(out.Rating+up.Amount, up.MaxValue)
		case "attribute":
			switch up.Attr {
			case "pac":
				out.Attributes.Pace = capAt(clamp99(out.Attributes.Pace+up.Amount), up.MaxValue)
			case "sho":
				out.Attributes.Shooting = capAt(clamp99(out.Attributes.Shooting+up.Amount), up.MaxValue)
			case "pas":
				out.Attributes.Passing = capAt(clamp99(out.Attributes.Passing+up.Amount), up.MaxValue)
			case "dri":
				out.Attributes.Dribbling = capAt(clamp99(out.Attributes.Dribbling+up.Amount), up.MaxValue)
			case "def":
				out.Attributes.Defending = capAt(clamp99(out.Attributes.Defending+up.Amount), up.MaxValue)
			case "phy":
				out.Attributes.Physical = capAt(clamp99(out.Attributes.Physical+up.Amount), up.MaxValue)
			}
		case "sub_attribute", "ignored":
			if out.DetailedAttributes != nil {
				applyDetailedUpgrade(out.DetailedAttributes, up.Attr, up.Amount, up.MaxValue)
			}
		case "playstyle":
			out.PlayStyles = addPlayStyle(out.PlayStyles, up.PlayStyle)
		case "position":
			if !out.PlaysAt(up.Position) {
				out.AltPositions = append(out.AltPositions, up.Position)
			}
		case "skill_moves":
			out.SkillMoves = clampStars(out.SkillMoves + up.Amount)
		case "weak_foot":
			out.WeakFoot = clampStars(out.WeakFoot + up.Amount)
		}
	}
	return out
}

func addPlayStyle(list []PlayStyle, ps PlayStyle) []PlayStyle {
	for i, existing := range list {
		if strings.EqualFold(existing.Name, ps.Name) {
			// Um PlayStyle+ substitui a versão normal.
			if ps.Plus {
				list[i].Plus = true
			}
			return list
		}
	}
	return append(list, ps)
}

// applyDetailedUpgrade altera um subatributo publicado pela carta. O ponteiro
// só é criado quando a fonte já forneceu aquele campo: uma ausência não vira
// zero artificial no gráfico nem numa recomendação do agente.
func applyDetailedUpgrade(d *DetailedAttributes, attr string, amount, max int) {
	if d == nil {
		return
	}
	apply := func(dst **int) {
		if *dst == nil {
			return
		}
		v := clamp99(**dst + amount)
		v = capAt(v, max)
		*dst = &v
	}
	switch strings.ToLower(strings.TrimSpace(attr)) {
	case "acceleration":
		apply(&d.Acceleration)
	case "sprint_speed":
		apply(&d.SprintSpeed)
	case "agility":
		apply(&d.Agility)
	case "balance":
		apply(&d.Balance)
	case "jumping":
		apply(&d.Jumping)
	case "stamina":
		apply(&d.Stamina)
	case "strength":
		apply(&d.Strength)
	case "reactions":
		apply(&d.Reactions)
	case "aggression":
		apply(&d.Aggression)
	case "composure":
		apply(&d.Composure)
	case "interceptions":
		apply(&d.Interceptions)
	case "positioning":
		apply(&d.Positioning)
	case "vision":
		apply(&d.Vision)
	case "ball_control":
		apply(&d.BallControl)
	case "crossing":
		apply(&d.Crossing)
	case "dribbling":
		apply(&d.Dribbling)
	case "finishing":
		apply(&d.Finishing)
	case "fk_accuracy":
		apply(&d.FKAccuracy)
	case "heading_accuracy":
		apply(&d.HeadingAccuracy)
	case "long_passing":
		apply(&d.LongPassing)
	case "short_passing":
		apply(&d.ShortPassing)
	case "defensive_awareness":
		apply(&d.DefensiveAwareness)
	case "shot_power":
		apply(&d.ShotPower)
	case "long_shots":
		apply(&d.LongShots)
	case "standing_tackle":
		apply(&d.StandingTackle)
	case "sliding_tackle":
		apply(&d.SlidingTackle)
	case "volleys":
		apply(&d.Volleys)
	case "curve":
		apply(&d.Curve)
	case "penalties":
		apply(&d.Penalties)
	case "gk_diving":
		apply(&d.GKDiving)
	case "gk_handling":
		apply(&d.GKHandling)
	case "gk_kicking":
		apply(&d.GKKicking)
	case "gk_reflexes":
		apply(&d.GKReflexes)
	case "gk_speed":
		apply(&d.GKSpeed)
	case "gk_positioning":
		apply(&d.GKPositioning)
	}
}

// capAt aplica o teto que o fut.gg declarou para o upgrade ("+10, até 96").
// max == 0 significa "nenhum teto declarado": o valor passa como veio, já
// contido pelo clamp99/clampStars de quem chamou.
func capAt(v, max int) int {
	if max > 0 && v > max {
		return max
	}
	return v
}

func clamp99(v int) int {
	if v > 99 {
		return 99
	}
	if v < 1 {
		return 1
	}
	return v
}

func clampStars(v int) int {
	if v > 5 {
		return 5
	}
	if v < 1 {
		return 1
	}
	return v
}
