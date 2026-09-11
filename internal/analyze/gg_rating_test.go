package analyze

import (
	"math"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestPlanejadorConsideraNotaMaiorDoClubeNaEscolhaETrocas(t *testing.T) {
	club := squadPlanFixtureClub(3)
	// O reserva intermediário supera o último somente com a nota da cópia.
	club.Players[1].GGRating = 99.74
	club.Players[1].GGRatings = map[domain.Position]float64{domain.GK: 61}
	plan := BuildSquadPlan(club, DefaultSquadPlanRequest())
	if plan.Status != "ok" || len(plan.Scenarios) == 0 {
		t.Fatalf("plano indisponível: %+v", plan)
	}
	for _, sc := range plan.Scenarios {
		if sc.Starters[0].Player.ID != club.Players[1].ID || sc.Starters[0].Rating != 99.74 {
			t.Fatalf("titular escolhido = %+v; esperado reserva com nota 99.74", sc.Starters[0])
		}
		esperado := 99.74 + 10*62
		if math.Abs(sc.TotalRating-esperado) > 1e-9 || math.Abs(sc.AverageRating-esperado/11) > 1e-9 {
			t.Fatalf("total = %v, média = %v", sc.TotalRating, sc.AverageRating)
		}
		var achou bool
		for _, m := range sc.Moves {
			if m.Index == 0 {
				achou = true
				if m.CurrentRating != 60 || m.SuggestedRating != 99.74 || math.Abs(m.Gain-39.74) > 1e-9 {
					t.Fatalf("troca não usa as notas corrigidas: %+v", m)
				}
			}
		}
		if !achou {
			t.Fatal("troca do goleiro não encontrada")
		}
	}
	opt := OptimizeSquad(club)
	if opt.Starters[0].Player.ID != club.Players[1].ID || math.Abs(opt.Gain-((99.74+10*62)/11-60)) > 1e-9 {
		t.Fatalf("otimização do Meu time não considera a nota do clube: %+v", opt)
	}
}

func TestGauntletConsideraNotaMaiorDoClubeNaEscolhaETotais(t *testing.T) {
	club := gauntletFixtureClub(8)
	club.Players[0].GGRating = 99.74
	club.Players[0].GGRatings = map[domain.Position]float64{domain.GK: 60}
	plan := BuildGauntletPlan(club)
	if plan.Status != "ok" {
		t.Fatalf("plano indisponível: %s", plan.Reason)
	}
	var achou bool
	for _, round := range plan.Rounds {
		var total float64
		for _, a := range round.Starters {
			total += a.Rating
			if a.Player.ID == club.Players[0].ID {
				achou = true
				if a.Rating != 99.74 {
					t.Fatalf("nota do titular = %v; esperado 99.74", a.Rating)
				}
			}
		}
		if math.Abs(round.TotalRating-total) > 1e-9 || math.Abs(round.AverageRating-total/11) > 1e-9 {
			t.Fatalf("rodada %d com totais inconsistentes", round.Round)
		}
	}
	if !achou {
		t.Fatal("carta com maior nota do clube ficou fora dos titulares")
	}
}

func TestPlanosMarcamVersaoDasNotasMesmoQuandoIndisponiveis(t *testing.T) {
	for _, club := range []domain.Club{{}, squadPlanFixtureClub(8)} {
		if got := OptimizeSquad(club).RatingVersion; got != domain.GGRatingVersion {
			t.Fatalf("versão do plano de elenco = %d", got)
		}
		if got := BuildGauntletPlan(club).RatingVersion; got != domain.GGRatingVersion {
			t.Fatalf("versão do plano de Gauntlet = %d", got)
		}
	}
}
