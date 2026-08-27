package futgg

import (
	"context"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// EvolutionGraph busca os caminhos confirmados (EvolutionPaths, já
// existente) e monta o grafo em cima deles — zero decode novo, zero HTTP
// novo. current precisa ser a carta REAL do elenco (com GG Rating), nunca
// uma reconstrução a partir da resposta de paths: ver o comentário de
// domain.EvolutionPath.Initial sobre path[0] nunca trazer nota.
func (c *Client) EvolutionGraph(ctx context.Context, current domain.Player) (domain.EvolutionGraph, error) {
	paths, err := c.EvolutionPaths(ctx, current.BasePlayerEaID, current.ID)
	if err != nil {
		return domain.EvolutionGraph{}, err
	}
	g := domain.LinearGraph(current, paths)
	if err := g.Validate(); err != nil {
		// Fail-closed real: um payload com self-loop (mesma carta como
		// início e fim de um "path") ou misturando ciclo de jogo não vira
		// um grafo torto que alguém tenta atravessar — vira erro.
		return domain.EvolutionGraph{}, err
	}
	return g, nil
}
