package domain

import "time"

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
	Kind   string `json:"kind"` // "overall", "attribute", "playstyle", "position", "skill_moves", "weak_foot"
	Attr   string `json:"attr"` // "pac", "sho", ... quando Kind == "attribute"
	Amount int    `json:"amount"`
	// MaxValue é o teto que o fut.gg declara para o ganho ("+10, até 96").
	// Zero significa "sem teto declarado" — Apply() cai no clamp padrão.
	MaxValue  int       `json:"max_value,omitempty"`
	PlayStyle PlayStyle `json:"play_style"`
	Position  Position  `json:"position"`
	Raw       string    `json:"raw"`
}

// EvoLevel é um nível da evolução: custa objetivos e entrega upgrades.
type EvoLevel struct {
	Number     int          `json:"number"`
	Upgrades   []EvoUpgrade `json:"upgrades"`
	Objectives []string     `json:"objectives"` // "Marque 5 gols em Rivals"...
}

// Evolution é uma evolução ativa na loja.
type Evolution struct {
	ID           string           `json:"id"`
	Slug         string           `json:"slug"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	CoinCost     int              `json:"coin_cost"`
	PointCost    int              `json:"point_cost"`
	Requirements []EvoRequirement `json:"requirements"`
	Levels       []EvoLevel       `json:"levels"`
	ExpiresAt    time.Time        `json:"expires_at"`
	Cycle        string           `json:"cycle"`
	URL          string           `json:"url"`
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
		if existing.Name == ps.Name {
			// Um PlayStyle+ substitui a versão normal.
			if ps.Plus {
				list[i].Plus = true
			}
			return list
		}
	}
	return append(list, ps)
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
