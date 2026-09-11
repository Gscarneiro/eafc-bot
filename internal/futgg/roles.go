package futgg

import (
	"context"
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Role é uma função tática que o fut.gg reconhece (Wide Playmaker, Box-To-Box,
// Deep-Lying Playmaker...). Uma carta não recebe uma nota por função — o
// campo que carregaria isso, roleGgRatings, vem sempre nulo em toda carta que
// já testamos — mas declara EM QUAIS funções ela é boa/muito boa, e é essa
// proficiência que fica no relatório por posição.
type Role struct {
	Name     string
	Position domain.Position
}

// RolesTable indexa o catálogo por eaId — de dois espaços DIFERENTES: uma
// carta lista suas funções normais em "rolesPlus" e as reforçadas em
// "rolesPlusPlus", e cada lista usa o eaId correspondente do catálogo
// (plusEaId ou plusPlusEaId). É por isso que a tabela guarda os dois.
type RolesTable struct {
	Plus     map[int]Role // por plusEaId
	PlusPlus map[int]Role // por plusPlusEaId
}

// ensureRoles carrega o catálogo de funções, se ainda não carregado. Falha em
// silêncio, como ensurePlayStyles: sem o catálogo, o relatório de cartas
// mostra os eaIds crus em vez de travar por causa de um endpoint acessório.
func (c *Client) ensureRoles(ctx context.Context) {
	c.rolesOnce.Do(func() {
		u, err := c.URL("roles", nil)
		if err != nil {
			return
		}
		body, err := c.GetRaw(ctx, u)
		if err != nil {
			return
		}
		nodes, err := c.decodeList(body, "roles")
		if err != nil {
			return
		}
		table := RolesTable{Plus: map[int]Role{}, PlusPlus: map[int]Role{}}
		for _, n := range nodes {
			name := n.str("name")
			if name == "" {
				continue
			}
			pos, ok := domain.PositionFromID(n.int("position"))
			if !ok {
				continue
			}
			r := Role{Name: name, Position: pos}
			if v, ok := n.lookup("plusEaId"); ok && v != nil {
				table.Plus[n.int("plusEaId")] = r
			}
			if v, ok := n.lookup("plusPlusEaId"); ok && v != nil {
				table.PlusPlus[n.int("plusPlusEaId")] = r
			}
		}
		if len(table.Plus) > 0 || len(table.PlusPlus) > 0 {
			c.rolesTable = &table
		}
	})
}

// Roles devolve o catálogo, carregando-o se preciso.
func (c *Client) Roles(ctx context.Context) RolesTable {
	c.ensureRoles(ctx)
	if c.rolesTable == nil {
		return RolesTable{Plus: map[int]Role{}, PlusPlus: map[int]Role{}}
	}
	return *c.rolesTable
}

// PreencherFamiliaridadesFuncoes materializa o catÃ¡logo no retrato da carta.
// A coleta conserva os IDs crus por compatibilidade, mas a avaliaÃ§Ã£o precisa
// de nome, posiÃ§Ã£o e nÃ­vel para explicar por que uma funÃ§Ã£o ajudou a nota.
func PreencherFamiliaridadesFuncoes(player *domain.Player, roles RolesTable) {
	if player == nil {
		return
	}
	known := make(map[string]domain.FamiliaridadeFuncao)
	add := func(role Role, nivel string) {
		key := string(role.Position) + "\x00" + role.Name
		current, ok := known[key]
		if !ok || (current.Nivel != "plus_plus" && nivel == "plus_plus") {
			known[key] = domain.FamiliaridadeFuncao{Nome: role.Name, Posicao: role.Position, Nivel: nivel}
		}
	}
	for _, id := range player.RolesPlus {
		if role, ok := roles.Plus[id]; ok {
			add(role, "plus")
		}
	}
	for _, id := range player.RolesPlusPlus {
		if role, ok := roles.PlusPlus[id]; ok {
			add(role, "plus_plus")
		}
	}
	player.FamiliaridadesFuncao = make([]domain.FamiliaridadeFuncao, 0, len(known))
	for _, role := range known {
		player.FamiliaridadesFuncao = append(player.FamiliaridadesFuncao, role)
	}
	sort.Slice(player.FamiliaridadesFuncao, func(i, j int) bool {
		left, right := player.FamiliaridadesFuncao[i], player.FamiliaridadesFuncao[j]
		if left.Posicao != right.Posicao {
			return left.Posicao < right.Posicao
		}
		return left.Nome < right.Nome
	})
}

// PreencherFamiliaridadesDoClube reaplica o catÃ¡logo de um snapshot salvo.
// Isso deixa snapshots anteriores Ã  nova projeÃ§Ã£o tÃ£o explicÃ¡veis quanto uma
// coleta nova, sem escrever de volta a fotografia histÃ³rica.
func PreencherFamiliaridadesDoClube(club *domain.Club, roles RolesTable) {
	if club == nil {
		return
	}
	for i := range club.Players {
		PreencherFamiliaridadesFuncoes(&club.Players[i].Player, roles)
	}
}

// PreencherFamiliaridadesDoMercado faz a mesma projeÃ§Ã£o para alternativas
// de compra. O avaliador recebe Player puro nestas comparaÃ§Ãµes.
func PreencherFamiliaridadesDoMercado(players []domain.Player, roles RolesTable) {
	for i := range players {
		PreencherFamiliaridadesFuncoes(&players[i], roles)
	}
}
