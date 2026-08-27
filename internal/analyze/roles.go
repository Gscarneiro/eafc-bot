// Package analyze compara o seu elenco com o que existe no mercado e decide
// o que vale a pena mudar.
package analyze

import "github.com/gscarneiro/eafc-bot/internal/domain"

// RoleWeights diz quanto cada atributo pesa numa função. Os pesos somam 1.
//
// Eles NÃO reproduzem a fórmula de overall da EA de propósito. O overall
// premia atributos que quase não aparecem em jogo (um zagueiro 89 lento é
// inútil em Rivals); estes pesos refletem o que decide partida no FC:
// ritmo e drible acima de tudo no ataque, ritmo e fisico na defesa.
type RoleWeights struct {
	Position domain.Position
	Weights  map[string]float64
	// KeyPlayStyles são os PlayStyles que mudam o comportamento da carta
	// nessa função. PlayStyle+ vale o dobro de um normal.
	KeyPlayStyles map[string]float64
	// MinPace é o piso abaixo do qual a carta é descartada na função,
	// por mais alto que seja o overall. 0 desliga o corte.
	MinPace int
}

// BotScoreProfile identifica a calibração usada para decidir compras. Ele
// nunca deve ser confundido com o GG Rating, que é uma observação do fut.gg
// para comparar cartas dentro do elenco.
type BotScoreProfile string

const DefaultBotScoreProfile BotScoreProfile = "mercado_v1"

// BotScoreComponent é uma parcela aditiva da nota. O conjunto é mantido
// pequeno de propósito: a soma reproduz exatamente Total sem esconder pesos
// em uma string de explicação.
type BotScoreComponent struct {
	Key   string  `json:"key"`
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// BotScore é a opinião reproduzível do bot sobre uma carta em uma função.
// Cycle e Version viajam junto para impedir uma comparação silenciosa entre
// perfis ou ciclos distintos quando a configuração virar para o próximo FC.
type BotScore struct {
	Profile    BotScoreProfile     `json:"profile"`
	Cycle      string              `json:"cycle"`
	Version    string              `json:"version"`
	Position   domain.Position     `json:"position"`
	Total      float64             `json:"total"`
	Components []BotScoreComponent `json:"components"`
	Missing    []string            `json:"missing,omitempty"`
	Confidence string              `json:"confidence"`
}

// CompatibleBotScores só permite comparação quando as duas notas têm o mesmo
// perfil, ciclo e função. Misturar opinião de mercado com GG Rating — ou uma
// calibração FC 26 com FC 27 — produz números atraentes, mas sem significado.
func CompatibleBotScores(a, b BotScore) bool {
	return a.Profile != "" && a.Profile == b.Profile &&
		a.Cycle != "" && a.Cycle == b.Cycle && a.Position == b.Position
}

// CompareBotScores devolve a diferença somente quando a comparação é válida.
func CompareBotScores(a, b BotScore) (float64, bool) {
	if !CompatibleBotScores(a, b) {
		return 0, false
	}
	return a.Total - b.Total, true
}

// roleTable é a tabela de funções. Ajustar aqui muda todo o ranking do bot,
// então é o ponto natural para calibrar com a sua forma de jogar.
var roleTable = map[domain.Position]RoleWeights{
	domain.ST: {
		Weights: map[string]float64{"pac": 0.26, "sho": 0.32, "dri": 0.22, "phy": 0.12, "pas": 0.06, "def": 0.02},
		KeyPlayStyles: map[string]float64{
			"Finesse Shot": 3.0, "Power Shot": 2.0, "Rapid": 3.0,
			"Quick Step": 2.5, "Trickster": 1.5, "Aerial": 1.5, "Acrobatic": 1.5,
		},
		MinPace: 80,
	},
	domain.CF: {
		Weights: map[string]float64{"pac": 0.22, "sho": 0.26, "dri": 0.28, "pas": 0.16, "phy": 0.06, "def": 0.02},
		KeyPlayStyles: map[string]float64{
			"Finesse Shot": 3.0, "Trickster": 2.0, "Rapid": 2.5, "Technical": 2.5, "Incisive Pass": 2.0,
		},
		MinPace: 78,
	},
	domain.RW: wingerRole(),
	domain.LW: wingerRole(),
	domain.RM: wingerRole(),
	domain.LM: wingerRole(),
	domain.CAM: {
		Weights: map[string]float64{"dri": 0.30, "pas": 0.24, "sho": 0.22, "pac": 0.18, "phy": 0.04, "def": 0.02},
		KeyPlayStyles: map[string]float64{
			"Finesse Shot": 3.0, "Incisive Pass": 2.5, "Technical": 2.5,
			"Trickster": 2.0, "Rapid": 2.0, "Press Proven": 1.0,
		},
		MinPace: 76,
	},
	domain.CM: {
		Weights: map[string]float64{"dri": 0.24, "pas": 0.24, "phy": 0.18, "pac": 0.16, "def": 0.12, "sho": 0.06},
		KeyPlayStyles: map[string]float64{
			"Press Proven": 2.5, "Incisive Pass": 2.5, "Technical": 2.0,
			"Relentless": 2.0, "Long Ball Pass": 1.5, "Intercept": 1.5,
		},
		MinPace: 72,
	},
	domain.CDM: {
		Weights: map[string]float64{"def": 0.28, "phy": 0.24, "pas": 0.16, "dri": 0.14, "pac": 0.16, "sho": 0.02},
		KeyPlayStyles: map[string]float64{
			"Anticipate": 3.0, "Intercept": 2.5, "Block": 2.0,
			"Press Proven": 2.5, "Relentless": 2.0, "Bruiser": 1.5,
		},
		MinPace: 70,
	},
	domain.CB: {
		Weights: map[string]float64{"def": 0.34, "phy": 0.26, "pac": 0.24, "dri": 0.08, "pas": 0.06, "sho": 0.02},
		KeyPlayStyles: map[string]float64{
			"Anticipate": 3.5, "Block": 2.5, "Bruiser": 2.0,
			"Aerial": 2.0, "Slide Tackle": 1.5, "Quick Step": 2.0,
		},
		MinPace: 78,
	},
	domain.RB:  fullBackRole(),
	domain.LB:  fullBackRole(),
	domain.RWB: wingBackRole(),
	domain.LWB: wingBackRole(),
	domain.GK: {
		// Campos reaproveitados: pac=DIV, sho=HAN, pas=KIC, dri=REF, def=SPD, phy=POS
		Weights: map[string]float64{"dri": 0.32, "pac": 0.24, "phy": 0.22, "sho": 0.14, "def": 0.04, "pas": 0.04},
		KeyPlayStyles: map[string]float64{
			"Far Reach": 3.0, "Quick Reflexes": 3.0, "Cross Claimer": 2.0,
			"Rush Out": 1.5, "Deflector": 2.0, "Footwork": 1.5,
		},
	},
}

func wingerRole() RoleWeights {
	return RoleWeights{
		Weights: map[string]float64{"pac": 0.30, "dri": 0.28, "sho": 0.20, "pas": 0.14, "phy": 0.06, "def": 0.02},
		KeyPlayStyles: map[string]float64{
			"Rapid": 3.5, "Quick Step": 3.0, "Trickster": 2.5,
			"Finesse Shot": 2.5, "Technical": 2.0, "Whipped Pass": 1.5,
		},
		MinPace: 85,
	}
}

func fullBackRole() RoleWeights {
	return RoleWeights{
		Weights: map[string]float64{"pac": 0.26, "def": 0.26, "phy": 0.18, "dri": 0.16, "pas": 0.12, "sho": 0.02},
		KeyPlayStyles: map[string]float64{
			"Anticipate": 3.0, "Quick Step": 2.5, "Relentless": 2.5,
			"Intercept": 2.0, "Whipped Pass": 1.5, "Block": 1.5,
		},
		MinPace: 82,
	}
}

func wingBackRole() RoleWeights {
	return RoleWeights{
		Weights: map[string]float64{"pac": 0.28, "dri": 0.20, "def": 0.22, "phy": 0.16, "pas": 0.12, "sho": 0.02},
		KeyPlayStyles: map[string]float64{
			"Rapid": 3.0, "Quick Step": 2.5, "Relentless": 2.5,
			"Anticipate": 2.5, "Whipped Pass": 2.0,
		},
		MinPace: 84,
	}
}

func init() {
	// Preenche o campo Position, que fica redundante na declaração acima.
	for pos, rw := range roleTable {
		rw.Position = pos
		roleTable[pos] = rw
	}
}

// WeightsFor devolve os pesos da função, e se a posição é conhecida.
func WeightsFor(pos domain.Position) (RoleWeights, bool) {
	rw, ok := roleTable[pos]
	return rw, ok
}

// attrOrder é a ordem FIXA em que Score soma os pesos de atributo. Somar
// direto com `for range rw.Weights` parecia inofensivo, mas Go aleatoriza
// de propósito a ordem de iteração de mapa — e soma de float64 não é
// associativa: a mesma carta, mesma função, podia sair com Score()
// diferente no último bit entre duas chamadas, só por causa da ordem de
// acumulação do arredondamento. Isso já produzia flakiness real em
// TestPlayStylePlusDesempata (duas cartas idênticas exceto PlayStyle
// comparadas por `!=`) — sintoma de que o bot podia, na prática, desempatar
// FindUpgrades/WeakestLinks de forma não-determinística entre execuções.
var attrOrder = []string{"pac", "sho", "pas", "dri", "def", "phy"}

// Score mantém a API histórica dos analisadores. Todo cálculo novo deve usar
// EvaluateBotScore quando precisar publicar perfil e breakdown; este wrapper
// evita uma migração quebrar ranking já testado de uma vez.
func Score(p domain.Player, pos domain.Position) float64 {
	return EvaluateBotScore(p, pos, DefaultBotScoreProfile).Total
}

// EvaluateBotScore calcula a nota de mercado e deixa cada parcela auditável.
// A ordem de soma é fixa, como Score sempre exigiu, para o breakdown ser
// reproduzível bit a bit entre chamadas.
func EvaluateBotScore(p domain.Player, pos domain.Position, profile BotScoreProfile) BotScore {
	out := BotScore{Profile: profile, Cycle: p.Cycle, Version: p.Version, Position: pos, Confidence: "confirmada"}
	rw, ok := roleTable[pos]
	if !ok {
		out.Components = []BotScoreComponent{{Key: "overall_fallback", Label: "overall (função sem perfil)", Value: float64(p.Rating)}}
		out.Total = float64(p.Rating)
		out.Confidence = "estimada"
		return out
	}

	for _, attr := range attrOrder {
		value := p.Attributes.Get(attr)
		if rw.Weights[attr] > 0 && value == 0 {
			out.Missing = append(out.Missing, attr)
		}
		component := BotScoreComponent{Key: "atributo_" + attr, Label: attr, Value: float64(value) * rw.Weights[attr]}
		out.Components = append(out.Components, component)
		out.Total += component.Value
	}

	// Bônus de PlayStyle: cada PlayStyle relevante soma seu peso,
	// e a versão + vale o dobro.
	var psBonus float64
	for _, ps := range p.PlayStyles {
		if w, relevant := rw.KeyPlayStyles[ps.Name]; relevant {
			if ps.Plus {
				psBonus += w * 2
			} else {
				psBonus += w
			}
		}
	}
	// O bônus é limitado para uma carta cheia de PlayStyle não superar
	// uma carta simplesmente muito melhor nos atributos.
	if psBonus > 12 {
		psBonus = 12
	}
	out.Components = append(out.Components, BotScoreComponent{Key: "playstyles", Label: "PlayStyles relevantes", Value: psBonus})
	out.Total += psBonus

	// Drible ruim e perna ruim fraca punem mais no ataque que na defesa.
	var techBonus float64
	if !pos.IsGK() {
		attacking := rw.Weights["dri"] + rw.Weights["sho"]
		techBonus = (float64(p.SkillMoves-3)*0.8 + float64(p.WeakFoot-3)*0.5) * attacking
	}
	out.Components = append(out.Components, BotScoreComponent{Key: "tecnica", Label: "perna fraca e dribles", Value: techBonus})
	out.Total += techBonus

	// Corte de ritmo: carta lenta demais para a função é penalizada forte,
	// em vez de descartada, para o relatório ainda poder citá-la.
	pacePenalty := 0.0
	if rw.MinPace > 0 && p.Attributes.Pace < rw.MinPace {
		pacePenalty = float64(rw.MinPace-p.Attributes.Pace) * 0.6
	}
	out.Components = append(out.Components, BotScoreComponent{Key: "corte_ritmo", Label: "ritmo abaixo do piso", Value: -pacePenalty})
	out.Total -= pacePenalty

	// Jogar fora da posição natural custa desempenho real em campo.
	positionPenalty := 0.0
	if !p.PlaysAt(pos) {
		positionPenalty = 4
	}
	out.Components = append(out.Components, BotScoreComponent{Key: "fora_posicao", Label: "fora da posição", Value: -positionPenalty})
	out.Total -= positionPenalty

	if len(out.Missing) > 0 || p.Cycle == "" {
		out.Confidence = "incompleta"
	}
	return out
}
