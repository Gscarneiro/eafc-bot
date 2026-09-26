package api

import (
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

func TestDistribuicaoOVRContaCopiasEMarcaAsDoXI(t *testing.T) {
	titular := domain.ClubPlayer{Player: domain.Player{ID: 1, Rating: 88}, ClubItemID: "a"}
	copia := domain.ClubPlayer{Player: domain.Player{ID: 1, Rating: 88}, ClubItemID: "b"}
	bronze := domain.ClubPlayer{Player: domain.Player{ID: 2, Rating: 64}, ClubItemID: "c"}
	icone := domain.ClubPlayer{Player: domain.Player{ID: 3, Rating: 93}, ClubItemID: "d"}
	faixas := distribuicaoOVR([]domain.ClubPlayer{titular, copia, bronze, icone}, map[string]bool{titular.IdentityKey(): true})
	byLabel := map[string]FaixaOVR{}
	total := 0
	for _, f := range faixas {
		byLabel[f.Faixa] = f
		total += f.Total
	}
	if total != 4 {
		t.Fatalf("toda cópia física precisa cair numa faixa; contou %d de 4", total)
	}
	if f := byLabel["88-89"]; f.Total != 2 || f.NoXI != 1 {
		t.Fatalf("duas cópias iguais são duas cartas, e só a titular conta no XI: %+v", f)
	}
	if byLabel["≤77"].Total != 1 || byLabel["90+"].Total != 1 {
		t.Fatalf("as pontas abertas precisam pegar bronze e ícone: %+v", faixas)
	}
}

func TestValorDoXIPulaDiaEmQueUmaCartaFicouSemPreco(t *testing.T) {
	dia := func(d int) time.Time { return time.Date(2026, 9, d, 5, 0, 0, 0, time.UTC) }
	starters := []StarterCard{
		{RosterCard: RosterCard{Player: domain.ClubPlayer{Player: domain.Player{ID: 1}}}},
		{RosterCard: RosterCard{Player: domain.ClubPlayer{Player: domain.Player{ID: 2}}}},
		{RosterCard: RosterCard{Player: domain.ClubPlayer{Player: domain.Player{ID: 3}, Untradeable: true}}},
		{RosterCard: RosterCard{Player: domain.ClubPlayer{Player: domain.Player{ID: 4}}}},
	}
	series := map[int64][]store.PricePoint{
		1: {{Coins: 100, ObservedAt: dia(1)}, {Coins: 110, ObservedAt: dia(2)}, {Coins: 120, ObservedAt: dia(3)}},
		2: {{Coins: 50, ObservedAt: dia(1)}, {Coins: 55, ObservedAt: dia(3)}},
		3: {{Coins: 9999, ObservedAt: dia(1)}},
	}
	valor := valorDoXI(starters, series)
	if valor.Cotadas != 2 || valor.SemCotacao != 2 {
		t.Fatalf("untradeable e carta sem histórico ficam fora da soma: %+v", valor)
	}
	if len(valor.Pontos) != 2 || valor.Pontos[0] != (PontoValorXI{Dia: "2026-09-01", Moedas: 150}) || valor.Pontos[1] != (PontoValorXI{Dia: "2026-09-03", Moedas: 175}) {
		t.Fatalf("o dia 02 (carta 2 sem preço) não pode virar uma queda falsa: %+v", valor.Pontos)
	}
}
