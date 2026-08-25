package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// A química do XI ATUAL não depende de GG Rating nenhum — só de
// posição/clube/liga/nação. Precisa sobreviver mesmo quando a SUGESTÃO
// (fluxo de custo mínimo) não pode nem começar por falta desse dado — bug
// real: CurrentQuimica só era calculada depois da checagem de GG Rating por
// slot, então sumia exatamente no caso mais comum de faltar dado.
func TestCurrentQuimicaSobreviveQuandoSugestaoFalhaPorFaltaDeGGRating(t *testing.T) {
	a := mk(85, domain.CB, 80, 40, 65, 70, 85, 80)
	a.League, a.Nation = "Liga A", "Nação A"
	b := mk(85, domain.CM, 75, 78, 82, 80, 70, 78)
	b.League, b.Nation = "Liga A", "Nação A"

	club := domain.Club{
		Players: []domain.ClubPlayer{starterCP(1, a), starterCP(2, b)},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 0, Position: domain.CB, PlayerID: 1},
			{Index: 1, Position: domain.CM, PlayerID: 2},
		}},
	}

	plan := OptimizeSquad(club)
	if plan.Status != "unavailable" || plan.Reason == "" {
		t.Fatalf("esperava a sugestão falhar por falta de GG Rating, veio status=%q reason=%q", plan.Status, plan.Reason)
	}
	if plan.CurrentQuimica == nil {
		t.Fatal("CurrentQuimica veio nil — a química do XI atual não deveria depender da sugestão ter dado certo")
	}
	if plan.CurrentQuimica.Total != 6 {
		t.Fatalf("CurrentQuimica.Total = %d, esperava 6 (2 titulares x 3, modelo padrão preenche por posição)", plan.CurrentQuimica.Total)
	}
	if plan.Quimica != nil {
		t.Fatalf("Quimica (da sugestão) deveria ficar nil sem Starters, veio %+v", plan.Quimica)
	}
}

// Caminho feliz: as duas químicas preenchem, e Quimica reflete a escalação
// SUGERIDA (o jogador 2, de GG Rating maior), não a atual.
func TestOptimizeSquadPreencheQuimicaDaSugestaoQuandoTudoDisponivel(t *testing.T) {
	atual := mk(80, domain.ST, 85, 80, 70, 82, 40, 78)
	atual.GGRating, atual.GGRatingPos = 80.0, domain.ST
	atual.League, atual.Nation = "Liga A", "Nação A"

	melhor := mk(90, domain.ST, 90, 88, 75, 88, 45, 82)
	melhor.GGRating, melhor.GGRatingPos = 90.0, domain.ST
	melhor.League, melhor.Nation = "Liga B", "Nação B"

	club := domain.Club{
		Players: []domain.ClubPlayer{starterCP(1, atual), starterCP(2, melhor)},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 0, Position: domain.ST, PlayerID: 1},
		}},
	}

	plan := OptimizeSquad(club)
	if plan.Quimica == nil || plan.CurrentQuimica == nil {
		t.Fatalf("esperava as duas químicas preenchidas, veio %+v", plan)
	}
	if len(plan.Starters) != 1 || plan.Starters[0].Player.ID != 2 {
		t.Fatalf("esperava a sugestão trocar para o jogador 2 (melhor GG Rating), starters=%+v", plan.Starters)
	}
	if plan.Quimica.Total != 3 {
		t.Fatalf("Quimica (sugestão) = %d, esperava 3 (1 titular em posição, modelo padrão)", plan.Quimica.Total)
	}
}
