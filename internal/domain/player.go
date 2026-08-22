// Package domain contém o modelo de negócio do bot, independente de fut.gg,
// do banco ou de qualquer formato de relatório. Trocar o ciclo do jogo
// (FC 26 -> FC 27) não deve exigir mudança aqui.
package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Position é uma posição do Ultimate Team.
type Position string

const (
	GK  Position = "GK"
	RB  Position = "RB"
	LB  Position = "LB"
	CB  Position = "CB"
	RWB Position = "RWB"
	LWB Position = "LWB"
	CDM Position = "CDM"
	CM  Position = "CM"
	CAM Position = "CAM"
	RM  Position = "RM"
	LM  Position = "LM"
	RW  Position = "RW"
	LW  Position = "LW"
	CF  Position = "CF"
	ST  Position = "ST"
)

// AllPositions na ordem em que fazem sentido num relatório (defesa -> ataque).
var AllPositions = []Position{GK, RB, CB, LB, RWB, LWB, CDM, CM, CAM, RM, LM, RW, LW, CF, ST}

// IsGK diz se a posição é de goleiro, que usa um conjunto de atributos diferente.
func (p Position) IsGK() bool { return p == GK }

// positionByID é a tabela de IDs de posição da EA.
//
// Ela existe porque as duas pontas do fut.gg falam línguas diferentes: a
// listagem de mercado manda position:"CM", e o elenco do GG Club manda
// position:14. Sem a tabela, todo jogador do SEU clube entra sem posição e
// o bot não consegue montar escalação nenhuma.
//
// Os números NÃO são chute: doze deles saíram dos próprios dados (a
// listagem de mercado traz "position" e "positionId" lado a lado) e os três
// que faltavam — RWB, LWB e CF — saíram do enum do bundle do fut.gg. Vale
// conferir o LWB: é 8, e não 6, que é o que a sequência sugeriria.
var positionByID = map[int]Position{
	0: GK, 2: RWB, 3: RB, 5: CB, 7: LB, 8: LWB, 10: CDM, 12: RM,
	14: CM, 16: LM, 18: CAM, 21: CF, 23: RW, 25: ST, 27: LW,
}

// PositionFromID traduz o id numérico da EA para uma Position.
func PositionFromID(id int) (Position, bool) {
	p, ok := positionByID[id]
	return p, ok
}

// ParsePosition normaliza o que vier do fut.gg/EA para uma Position, seja o
// rótulo ("CM") ou o id numérico em texto ("14").
func ParsePosition(s string) (Position, error) {
	s = strings.TrimSpace(s)
	n := Position(strings.ToUpper(s))
	for _, p := range AllPositions {
		if p == n {
			return p, nil
		}
	}
	if id, err := strconv.Atoi(s); err == nil {
		if p, ok := positionByID[id]; ok {
			return p, nil
		}
	}
	return "", fmt.Errorf("posição desconhecida: %q", s)
}

// Attributes são os seis atributos de linha. Para goleiros os mesmos campos
// carregam DIV/HAN/KIC/REF/SPD/POS, na mesma ordem em que a EA os exibe.
type Attributes struct {
	Pace      int `json:"pace"`      // GK: Diving
	Shooting  int `json:"shooting"`  // GK: Handling
	Passing   int `json:"passing"`   // GK: Kicking
	Dribbling int `json:"dribbling"` // GK: Reflexes
	Defending int `json:"defending"` // GK: Speed
	Physical  int `json:"physical"`  // GK: Positioning
}

// Get devolve o atributo pelo nome curto usado nos pesos de função.
func (a Attributes) Get(key string) int {
	switch key {
	case "pac":
		return a.Pace
	case "sho":
		return a.Shooting
	case "pas":
		return a.Passing
	case "dri":
		return a.Dribbling
	case "def":
		return a.Defending
	case "phy":
		return a.Physical
	}
	return 0
}

// PlayStyle é um PlayStyle normal ou PlayStyle+ (que vale bem mais em jogo).
type PlayStyle struct {
	Name string `json:"name"`
	Plus bool   `json:"plus"`
}

func (ps PlayStyle) String() string {
	if ps.Plus {
		return ps.Name + "+"
	}
	return ps.Name
}

// Price é a cotação de mercado de uma carta num dado momento.
type Price struct {
	Coins     int       `json:"coins"`
	Extinct   bool      `json:"extinct"` // sem oferta no mercado
	IsSBC     bool      `json:"is_sbc"`  // carta de SBC, não é comprável
	UpdatedAt time.Time `json:"updated_at"`
}

// Tradeable diz se dá para simplesmente comprar a carta no mercado.
func (p Price) Tradeable() bool { return !p.IsSBC && p.Coins > 0 }

// Player é uma carta do jogo, do jeito que o fut.gg a descreve.
type Player struct {
	ID           int64       `json:"id"`   // id do recurso na EA
	FutGGSlug    string      `json:"slug"` // slug da página no fut.gg
	Name         string      `json:"name"`
	CommonName   string      `json:"common_name"`
	Rating       int         `json:"rating"`
	Position     Position    `json:"position"`
	AltPositions []Position  `json:"alt_positions"`
	Version      string      `json:"version"` // "Gold Rare", "TOTW", "Icon", "Hero"...
	Club         string      `json:"club"`
	League       string      `json:"league"`
	Nation       string      `json:"nation"`
	Attributes   Attributes  `json:"attributes"`
	PlayStyles   []PlayStyle `json:"play_styles"`
	WeakFoot     int         `json:"weak_foot"`
	SkillMoves   int         `json:"skill_moves"`
	Foot         string      `json:"foot"`
	Height       int         `json:"height_cm"`
	Price        Price       `json:"price"`
	Cycle        string      `json:"cycle"` // "26", "27" — permite conviver com dois ciclos
	ReleasedAt   time.Time   `json:"released_at"`

	// GGRating é a nota que o próprio fut.gg calcula pra carta — não o
	// overall da EA, um número deles, ~0-99, achado NA MELHOR posição do
	// cartão (que é GGRatingPos, e pode ser diferente de Position: o
	// Mbappé, por exemplo, tem Position ST mas GGRatingPos RM). Zero
	// significa "essa fonte não trouxe" — só a listagem de elenco do GG
	// Club manda esse campo; a listagem de mercado não manda.
	//
	// É deliberadamente um número DIFERENTE do Score() deste pacote:
	// Score() pesa atributo e PlayStyle do jeito que ESTE bot decidiu que
	// importa pra decisão de troca, e pode passar de 99 pelos bônus. Pra
	// decisão de troca continua valendo Score(); GGRating é só referência
	// — o número que você já conhece do site, e que nunca estoura a escala.
	GGRating    float64  `json:"gg_rating,omitempty"`
	GGRatingPos Position `json:"gg_rating_pos,omitempty"`

	// ImageURL é a arte da carta, pronta pra usar num <img> — o fut.gg já
	// devolve a URL final, redimensionada pelo CDN deles.
	ImageURL string `json:"image_url,omitempty"`

	// BasePlayerEaID identifica o JOGADOR, não a carta: todas as versões do
	// Gnabry (ouro 85, TOTW 90, FUTTIES 98) compartilham o mesmo valor. É a
	// chave da rota de evoluções — /evolutions/v2/26/paths/v2/{base}/ —, que
	// devolve os caminhos de todas as versões dele de uma vez; o filtro por
	// carta é feito depois, casando path[0].eaId com Player.ID.
	BasePlayerEaID int64  `json:"base_player_ea_id,omitempty"`
	BasePlayerSlug string `json:"base_player_slug,omitempty"`

	// RolesPlus/RolesPlusPlus são ids de função (eaId do catálogo
	// /api/fut/roles/) que a carta domina — Plus é o nível fraco, PlusPlus
	// o forte. Ficam como id cru e não resolvidos aqui de propósito: o
	// catálogo que traduz id -> nome+posição é uma tabela externa,
	// carregada uma vez por execução (futgg.RolesTable), do mesmo jeito
	// que GGRating não vem com nome de posição pronto.
	RolesPlus     []int `json:"roles_plus,omitempty"`
	RolesPlusPlus []int `json:"roles_plus_plus,omitempty"`
}

// Display devolve o nome mais curto e reconhecível da carta.
func (p Player) Display() string {
	if p.CommonName != "" {
		return p.CommonName
	}
	return p.Name
}

// Label é como o jogador aparece no relatório: "Mbappé (91 ST, TOTW)".
func (p Player) Label() string {
	return fmt.Sprintf("%s (%d %s, %s)", p.Display(), p.Rating, p.Position, p.Version)
}

// PlaysAt diz se o jogador atua na posição sem precisar de card de posição,
// considerando a posição natural e as alternativas.
func (p Player) PlaysAt(pos Position) bool {
	if p.Position == pos {
		return true
	}
	for _, alt := range p.AltPositions {
		if alt == pos {
			return true
		}
	}
	return false
}

// HasPlayStyle diz se o jogador tem o PlayStyle. plusOnly exige a versão +.
func (p Player) HasPlayStyle(name string, plusOnly bool) bool {
	for _, ps := range p.PlayStyles {
		if strings.EqualFold(ps.Name, name) && (!plusOnly || ps.Plus) {
			return true
		}
	}
	return false
}
