package domain

import "testing"

// Uma formação real repete posição — um 4-4-1-1 tem dois CB. Starter(pos)
// tem um único consumidor (analyze.FindEvolutions, para responder "essa
// evolução vira titular?") e a resposta certa ali é comparar contra o PIOR
// dos dois: superar o mais fraco já é o suficiente para virar titular de
// verdade, substituindo-o. Comparar contra o melhor sub-relataria os casos.
func TestStarterDevolveOPiorEntreDuplicados(t *testing.T) {
	forte := ClubPlayer{Player: Player{ID: 1, Rating: 88, Position: CB}}
	fraco := ClubPlayer{Player: Player{ID: 2, Rating: 79, Position: CB}}
	club := Club{
		Players: []ClubPlayer{forte, fraco},
		Squad: Squad{Starters: []SquadSlot{
			{Index: 0, Position: CB, PlayerID: 1},
			{Index: 1, Position: CB, PlayerID: 2},
		}},
	}

	got, ok := club.Starter(CB)
	if !ok {
		t.Fatal("esperava achar um titular em CB")
	}
	if got.ID != 2 {
		t.Errorf("Starter(CB) devolveu id %d (rating %d), esperava o mais fraco (id 2, rating 79)",
			got.ID, got.Rating)
	}
}

// Posição sem titular nenhum devolve "não achou" — não pode inventar.
func TestStarterSemTitularNaPosicao(t *testing.T) {
	club := Club{
		Players: []ClubPlayer{{Player: Player{ID: 1, Rating: 85, Position: CB}}},
		Squad:   Squad{Starters: []SquadSlot{{Index: 0, Position: CB, PlayerID: 1}}},
	}
	if _, ok := club.Starter(ST); ok {
		t.Error("não deveria achar titular em ST")
	}
}
