package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

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

func evolutionTestPlayer(id int64, rating int) domain.Player {
	return domain.Player{ID: id, Rating: rating, Position: domain.ST, Attributes: domain.Attributes{
		Pace: 90, Shooting: 90, Passing: 90, Dribbling: 90, Defending: 70, Physical: 90,
	}}
}
