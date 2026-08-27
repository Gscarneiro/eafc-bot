package cards

import (
	"context"

	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
)

// EvolutionGraphSource é o mínimo que o planner de evolução precisa da fonte
// de dados — produção ou fixture, sem HTTP escondido em quem consome. Mesmo
// padrão de internal/discover.Fetcher: interface pequena, dona é quem
// consome (aqui, cards, o pacote da análise "atual x potencial" carta a
// carta).
type EvolutionGraphSource interface {
	EvolutionGraph(ctx context.Context, current domain.Player) (domain.EvolutionGraph, error)
}

// *futgg.Client satisfaz isto estruturalmente — ver internal/futgg/evolution_graph.go.
var _ EvolutionGraphSource = (*futgg.Client)(nil)
