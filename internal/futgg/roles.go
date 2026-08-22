package futgg

import (
	"context"

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
