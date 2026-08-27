package cards

import (
	"context"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// fixtureEvolutionGraphSource é um fake sem rede da porta EvolutionGraphSource
// — único consumidor por enquanto é este teste, provando que a porta é
// satisfazível sem *futgg.Client. Fica só aqui (não exportado): promover
// pra um fixture reutilizável é decisão da parte 2, quando existir um
// segundo consumidor real (o endpoint /plano ou serve -demo).
type fixtureEvolutionGraphSource struct {
	graph domain.EvolutionGraph
	err   error
}

func (f fixtureEvolutionGraphSource) EvolutionGraph(ctx context.Context, current domain.Player) (domain.EvolutionGraph, error) {
	return f.graph, f.err
}

func TestEvolutionGraphSourceAceitaFixtureSemRede(t *testing.T) {
	want := domain.EvolutionGraph{
		Cycle:  "26",
		RootID: "raiz",
		Nodes: map[string]domain.EvolutionNode{
			"raiz": {ID: "raiz", Card: domain.Player{ID: 1, Cycle: "26"}},
		},
	}
	var src EvolutionGraphSource = fixtureEvolutionGraphSource{graph: want}

	got, err := src.EvolutionGraph(context.Background(), domain.Player{ID: 1, Cycle: "26"})
	if err != nil {
		t.Fatal(err)
	}
	if got.RootID != want.RootID || got.Cycle != want.Cycle {
		t.Errorf("grafo devolvido pelo fake = %+v, esperava %+v", got, want)
	}
}
