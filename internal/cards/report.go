// Package cards monta o relatório "atual x potencial" por carta: para cada
// jogador acima de uma nota mínima, o estado de hoje, as funções que o
// fut.gg reconhece nele por posição, e o teto que as evoluções disponíveis
// alcançam — tudo lido da API deles, sem fórmula própria por cima.
package cards

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
)

// PositionRoles são as funções que a carta domina numa posição — PlusPlus é
// o nível forte (Role++), Plus o nível fraco (Role+). Vem direto do que a
// carta declara (rolesPlus/rolesPlusPlus) cruzado com o catálogo de nomes.
type PositionRoles struct {
	Position domain.Position `json:"position"`
	PlusPlus []string        `json:"plus_plus"`
	Plus     []string        `json:"plus"`
}

// EvoPotential é um caminho de evolução válido, já com o ganho calculado
// contra a carta REAL de hoje — não contra o path[0] da API, que não traz
// GG Rating nenhum (ver domain.EvolutionPath.Initial).
type EvoPotential struct {
	Path             domain.EvolutionPath `json:"path"`
	FinalOverall     int                  `json:"final_overall"`
	FinalGGRating    float64              `json:"final_gg_rating"`
	GGRatingGain     float64              `json:"gg_rating_gain"`
	GainedPlayStyles []domain.PlayStyle   `json:"gained_play_styles"`
	CoinsCost        int                  `json:"coin_cost"`
	PointsCost       int                  `json:"point_cost"`
	TrainingTime     string               `json:"training_time"`
}

// CardReport é uma carta pronta pro relatório.
type CardReport struct {
	// Slug identifica a carta na URL (/cards/:slug) e é ÚNICO dentro de um
	// mesmo BuildReports — mesmo quando duas cartas do elenco compartilham
	// o eaId (caso real: duas entradas de "Carlos Alberto", overall 95 e
	// 92, aparentam ser a mesma carta capturada em dois estágios de
	// evolução pelo fut.gg). Calculado aqui, não no front, para as duas
	// pontas nunca divergirem em como desambiguar.
	Slug       string            `json:"slug"`
	Player     domain.ClubPlayer `json:"player"`
	ByPosition []PositionRoles   `json:"by_position"`
	// Best é nil quando a carta não tem nenhum caminho de evolução válido
	// hoje — ela já está no teto que as evoluções ativas alcançam. Isso é
	// uma resposta, não uma falha: aparece no relatório assim mesmo.
	Best       *EvoPotential  `json:"best"`
	Alternates []EvoPotential `json:"alternates"`
}

// BuildReports monta um CardReport para cada jogador do clube com rating
// acima de minRating. A ordem de saída é por GGRatingGain do Best
// decrescente (quem ganha mais primeiro); cartas sem Best vão ao final,
// ordenadas por nome.
func BuildReports(ctx context.Context, client *futgg.Client, club domain.Club, minRating int) ([]CardReport, error) {
	roles := client.Roles(ctx)

	var out []CardReport
	for _, cp := range club.Players {
		if cp.Player.Rating <= minRating {
			continue
		}
		r := CardReport{
			Player:     cp,
			ByPosition: positionRoles(cp.Player, roles),
		}

		if cp.Player.BasePlayerEaID > 0 {
			paths, err := client.EvolutionPaths(ctx, cp.Player.BasePlayerEaID, cp.Player.ID)
			if err != nil {
				// Uma carta sem caminho não pode derrubar as outras 93 —
				// mesmo princípio do Collect(): fonte acessória falha,
				// o resto do relatório continua.
				paths = nil
			}
			best, alts := bestPaths(paths, cp.Player)
			r.Best, r.Alternates = best, alts
		}

		out = append(out, r)
	}

	assignSlugs(out)

	sort.Slice(out, func(i, j int) bool {
		bi, bj := out[i].Best, out[j].Best
		switch {
		case bi == nil && bj == nil:
			return out[i].Player.Display() < out[j].Player.Display()
		case bi == nil:
			return false // sem Best vai para o final
		case bj == nil:
			return true
		default:
			return bi.GGRatingGain > bj.GGRatingGain
		}
	})
	return out, nil
}

// cardSlug decide a identidade pública de uma carta: o slug do fut.gg
// quando existe (único e legível: "26-117633497"), senão o ID como último
// recurso — nunca vazio, porque duas cartas sem slug se colidiriam.
func cardSlug(p domain.Player) string {
	if p.FutGGSlug != "" {
		return sanitizeSlug(p.FutGGSlug)
	}
	return "player-" + strconv.FormatInt(p.ID, 10)
}

func sanitizeSlug(s string) string {
	s = strings.Trim(s, "/")
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
	}
	if s == "" {
		return "carta"
	}
	return s
}

// assignSlugs preenche CardReport.Slug em cada item, desambiguando quando
// duas cartas do elenco caem no mesmo slug — caso real: duas entradas de
// "Carlos Alberto" com o MESMO eaId, overall 95 e 92 (parecem a mesma carta
// capturada em dois estágios de evolução pelo próprio fut.gg). Sem isto, a
// segunda "some" pro cliente: duas linhas na tabela apontando pra rota
// idêntica. O sufixo usa o overall, que é justamente o que diferencia as
// duas nesse caso real, e cai num contador se nem isso desempatar.
func assignSlugs(reports []CardReport) {
	seen := map[string]int{}
	for i := range reports {
		base := cardSlug(reports[i].Player.Player)
		slug := base
		if n := seen[base]; n > 0 {
			slug = fmt.Sprintf("%s-%d", base, reports[i].Player.Rating)
			if seen[slug] > 0 {
				slug = fmt.Sprintf("%s-%d", slug, n+1)
			}
		}
		seen[base]++
		seen[slug]++
		reports[i].Slug = slug
	}
}

// bestPaths escolhe o caminho de maior GG Rating final (desempate: menor
// custo em moedas) e devolve os demais válidos como alternativas, na mesma
// ordem. Caminho expirado não entra — sugerir algo que não dá mais para
// pegar é pior que não sugerir.
func bestPaths(paths []domain.EvolutionPath, current domain.Player) (best *EvoPotential, alternates []EvoPotential) {
	var valid []EvoPotential
	for _, p := range paths {
		if p.IsExpired {
			continue
		}
		final := p.Final()
		gain := ggGain(current, final)
		// Uma carta muito boa pode ainda "caber" no requisito de overall de
		// uma evolução pensada para carta mais fraca — o overall bate, mas
		// o teto de GG Rating daquela evolução é mais baixo do que a carta
		// já tem hoje. Confirmado ao vivo: Josh Acheampong (GG 97.3) só
		// achava caminhos até 96.0. Sugerir isso como "potencial" seria
		// recomendar uma piora — mesmo tratamento de "sem caminho": essa
		// carta já está no teto que as evoluções ativas oferecem.
		if gain <= 0 {
			continue
		}
		valid = append(valid, EvoPotential{
			Path:             p,
			FinalOverall:     final.Rating,
			FinalGGRating:    final.GGRating,
			GGRatingGain:     gain,
			GainedPlayStyles: p.GainedPlayStyles(),
			CoinsCost:        p.CoinsCost,
			PointsCost:       p.PointsCost,
			TrainingTime:     p.TrainingTime,
		})
	}
	if len(valid) == 0 {
		return nil, nil
	}
	sort.Slice(valid, func(i, j int) bool {
		if valid[i].FinalGGRating != valid[j].FinalGGRating {
			return valid[i].FinalGGRating > valid[j].FinalGGRating
		}
		return valid[i].CoinsCost < valid[j].CoinsCost
	})
	return &valid[0], valid[1:]
}

// ggGain compara o final do caminho contra a carta REAL de hoje (a do
// elenco, com GGRating de verdade) — não contra path[0], que a API sempre
// manda sem nota.
func ggGain(current, final domain.Player) float64 {
	if current.GGRating <= 0 || final.GGRating <= 0 {
		return 0
	}
	return final.GGRating - current.GGRating
}

// positionRoles agrupa as funções da carta por posição, cruzando os ids
// crus (RolesPlus/RolesPlusPlus) com o catálogo. Id sem correspondência no
// catálogo (tabela não carregada, ou id novo que o catálogo ainda não tem)
// é ignorado — a posição principal e as alternativas ainda aparecem, só sem
// o detalhe de função.
func positionRoles(p domain.Player, roles futgg.RolesTable) []PositionRoles {
	byPos := map[domain.Position]*PositionRoles{}
	get := func(pos domain.Position) *PositionRoles {
		if pr, ok := byPos[pos]; ok {
			return pr
		}
		pr := &PositionRoles{Position: pos}
		byPos[pos] = pr
		return pr
	}

	for _, id := range p.RolesPlusPlus {
		if r, ok := roles.PlusPlus[id]; ok {
			pr := get(r.Position)
			pr.PlusPlus = appendUnique(pr.PlusPlus, r.Name)
		}
	}
	for _, id := range p.RolesPlus {
		if r, ok := roles.Plus[id]; ok {
			pr := get(r.Position)
			pr.Plus = appendUnique(pr.Plus, r.Name)
		}
	}

	out := make([]PositionRoles, 0, len(byPos))
	for _, pr := range byPos {
		out = append(out, *pr)
	}
	// A posição natural primeiro, depois as alternativas na ordem que a
	// carta as declara — é a ordem que mais importa para quem está lendo.
	sort.Slice(out, func(i, j int) bool {
		ri, rj := positionRank(out[i].Position, p), positionRank(out[j].Position, p)
		if ri != rj {
			return ri < rj
		}
		return out[i].Position < out[j].Position
	})
	return out
}

func positionRank(pos domain.Position, p domain.Player) int {
	if pos == p.Position {
		return 0
	}
	for i, alt := range p.AltPositions {
		if alt == pos {
			return i + 1
		}
	}
	return len(p.AltPositions) + 1
}

func appendUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}
