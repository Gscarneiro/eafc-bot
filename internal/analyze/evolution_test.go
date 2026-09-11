package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

type avaliadorEvolucaoPorFuncaoTeste struct{}

func (avaliadorEvolucaoPorFuncaoTeste) Perfis() []domain.PerfilMeta { return nil }

func (avaliadorEvolucaoPorFuncaoTeste) Avaliar(card domain.Player, pos domain.Position, ctx domain.ContextoAvaliacao) domain.AvaliacaoCarta {
	var nota float64
	switch ctx.Funcao {
	case "volante":
		switch card.ID {
		case 1:
			nota = 88
		case 3:
			nota = 80
			if card.Attributes.Pace >= 95 {
				nota = 94
			}
		}
	case "criador":
		switch card.ID {
		case 2:
			nota = 97
		case 3:
			nota = 75
			if card.Attributes.Pace >= 95 {
				nota = 82
			}
		}
	}
	return domain.AvaliacaoCarta{Disponivel: nota > 0, Nota: nota, Contexto: ctx}
}

func TestFindEvolutionsRespeitaPisoDeOverallInclusivo(t *testing.T) {
	club := domain.Club{Players: []domain.ClubPlayer{
		{Player: evolutionTestPlayer(1, 87)},
		{Player: evolutionTestPlayer(2, 88)},
	}}
	evo := domain.Evolution{
		ID:     "evo-1",
		Levels: []domain.EvoLevel{{Upgrades: []domain.EvoUpgrade{{Kind: "attribute", Attr: "pac", Amount: 5}}}},
	}

	got := FindEvolutionsWithOptions(club, []domain.Evolution{evo}, EvolutionOptions{Budget: 0, MinRating: 88, IncludeUnaffordable: true})
	if len(got) != 1 || got[0].Player.ID != 2 {
		t.Fatalf("matches = %+v, esperava somente a carta 88", got)
	}
}

func TestFindEvolutionsMarcaMetaForaDoOrcamento(t *testing.T) {
	club := domain.Club{Players: []domain.ClubPlayer{{Player: evolutionTestPlayer(1, 90)}}}
	evo := domain.Evolution{ID: "evo-cara", CoinCost: 100, Levels: []domain.EvoLevel{{Upgrades: []domain.EvoUpgrade{{Kind: "attribute", Attr: "pac", Amount: 5}}}}}

	got := FindEvolutionsWithOptions(club, []domain.Evolution{evo}, EvolutionOptions{Budget: 50, MinRating: 88, IncludeUnaffordable: true})
	if len(got) != 1 || got[0].Affordable {
		t.Fatalf("meta = %+v, esperava Affordable=false", got)
	}
}

func TestFindEvolutionsDizQualTitularPerdeNaVagaContextual(t *testing.T) {
	player := func(id int64) domain.ClubPlayer {
		p := evolutionTestPlayer(id, 90)
		p.Position = domain.CM
		return domain.ClubPlayer{Player: p}
	}
	club := domain.Club{
		Players: []domain.ClubPlayer{player(1), player(2), player(3)},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 7, Position: domain.CM, PlayerID: 1},
			{Index: 8, Position: domain.CM, PlayerID: 2},
		}},
	}
	evo := domain.Evolution{ID: "ritmo", Levels: []domain.EvoLevel{{Upgrades: []domain.EvoUpgrade{{Kind: "attribute", Attr: "pac", Amount: 5}}}}}

	got := FindEvolutionsWithOptions(club, []domain.Evolution{evo}, EvolutionOptions{
		Budget: 0, MinRating: 88, IncludeUnaffordable: true,
		Evaluator: avaliadorEvolucaoPorFuncaoTeste{},
		ContextosPorVaga: map[int]domain.ContextoAvaliacao{
			7: {Fonte: domain.FonteBot, Funcao: "volante"},
			8: {Fonte: domain.FonteBot, Funcao: "criador"},
		},
	})
	var match *EvoMatch
	for i := range got {
		if got[i].Player.ID == 3 {
			match = &got[i]
			break
		}
	}
	if match == nil {
		t.Fatalf("evolução da reserva 3 não foi encontrada: %+v", got)
	}
	if !match.BeatsStarter || !match.HasComparison || match.Index != 7 || match.Current == nil || match.Current.ID != 1 {
		t.Fatalf("comparação = %+v, esperava entrar como volante no lugar do titular 1 da vaga 7", *match)
	}
	if match.ReplacementGain != 6 || match.CandidateEvaluation.Contexto.Funcao != "volante" {
		t.Fatalf("ganho/contexto = %.1f/%q, esperava +6 na função volante", match.ReplacementGain, match.CandidateEvaluation.Contexto.Funcao)
	}
}

func evolutionTestPlayer(id int64, rating int) domain.Player {
	return domain.Player{ID: id, Rating: rating, Position: domain.ST, Attributes: domain.Attributes{
		Pace: 90, Shooting: 90, Passing: 90, Dribbling: 90, Defending: 70, Physical: 90,
	}}
}
