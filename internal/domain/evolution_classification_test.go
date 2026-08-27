package domain

import "testing"

func TestClassifyEvolutionRecompensaMantemCategoriaMesmoComPlayStylePlus(t *testing.T) {
	evo := Evolution{
		Name:               "Rapid+ de objetivo",
		IsRewardEvolution:  true,
		ObjectiveGroupName: "FUTTIES",
		Levels:             []EvoLevel{{Upgrades: []EvoUpgrade{{Kind: "playstyle", PlayStyle: PlayStyle{Name: "Rapid", Plus: true}}}}},
	}
	got := evo.ClassifyEvolution().Classification
	if got.Category != EvolutionCategoryRewards {
		t.Fatalf("categoria = %q, want %q", got.Category, EvolutionCategoryRewards)
	}
	if got.Origin != EvolutionOriginObjective {
		t.Fatalf("origem = %q, want %q", got.Origin, EvolutionOriginObjective)
	}
	if got.CategorySource != "futgg:is_reward_evolution" {
		t.Fatalf("fonte da categoria = %q", got.CategorySource)
	}
}

func TestClassifyEvolutionPlayStylesLabDivideSomentePeloUpgradeUnico(t *testing.T) {
	tests := []struct {
		name, want string
		plus       bool
	}{
		{name: "normal", want: EvolutionCategoryPlayStyles},
		{name: "plus", want: EvolutionCategoryPlayStylesPlus, plus: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evo := Evolution{CategorySlug: "playstyle-lab", CategoryName: "PlayStyles Lab", Levels: []EvoLevel{{Upgrades: []EvoUpgrade{{Kind: "playstyle", PlayStyle: PlayStyle{Name: "Tiki Taka", Plus: tt.plus}}}}}}
			got := evo.ClassifyEvolution().Classification.Category
			if got != tt.want {
				t.Fatalf("categoria = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyEvolutionCategoriaExplicitaNaoDependeDoCusto(t *testing.T) {
	evo := Evolution{CategorySlug: "roles", CategoryName: "Roles++", CoinCost: 50_000, DoesNotUpgradePlayer: true}
	got := evo.ClassifyEvolution().Classification
	if got.Category != EvolutionCategoryRolesPlusPlus {
		t.Fatalf("categoria = %q, want %q", got.Category, EvolutionCategoryRolesPlusPlus)
	}
	if got.Origin != EvolutionOriginPaid {
		t.Fatalf("origem = %q, want %q", got.Origin, EvolutionOriginPaid)
	}
}

func TestClassifyEvolutionTreinoERecompensaSemFlagUsamMetadadosPublicados(t *testing.T) {
	training := (Evolution{TotalTrainingTime: 3_600}).ClassifyEvolution().Classification
	if training.Category != EvolutionCategoryTrainingCamp {
		t.Fatalf("training category = %q", training.Category)
	}
	reward := (Evolution{CategorySlug: "rewards", CategoryName: "Rewards", SBCName: "Desafio"}).ClassifyEvolution().Classification
	if reward.Category != EvolutionCategoryRewards || reward.Origin != EvolutionOriginSBC {
		t.Fatalf("recompensa = %#v", reward)
	}
}

func TestEvolutionApplySubatributoRespeitaTetoESemFonteNaoInventaZero(t *testing.T) {
	value := 95
	evo := Evolution{Levels: []EvoLevel{{Upgrades: []EvoUpgrade{
		{Kind: "sub_attribute", Attr: "finishing", Amount: 8, MaxValue: 96},
		{Kind: "sub_attribute", Attr: "shot_power", Amount: 4},
	}}}}
	player := Player{DetailedAttributes: &DetailedAttributes{Finishing: &value}}
	got := evo.Apply(player)
	if got.DetailedAttributes == nil || got.DetailedAttributes.Finishing == nil || *got.DetailedAttributes.Finishing != 96 {
		t.Fatalf("finalização = %#v, want 96", got.DetailedAttributes)
	}
	if got.DetailedAttributes.ShotPower != nil {
		t.Fatalf("subatributo ausente virou valor: %#v", got.DetailedAttributes.ShotPower)
	}
}
