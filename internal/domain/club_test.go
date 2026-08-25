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

func TestNetSellValueAplicaTaxaSemFloat(t *testing.T) {
	tradeable := ClubPlayer{Player: Player{Price: Price{Coins: 100001}}}
	if got := tradeable.NetSellValue(); got != 95000 {
		t.Fatalf("NetSellValue = %d, esperava 95000", got)
	}
	locked := ClubPlayer{Player: Player{Price: Price{Coins: 100001}}, Untradeable: true}
	if got := locked.NetSellValue(); got != 0 {
		t.Fatalf("carta inegociável vale %d líquido, esperava 0", got)
	}
}

func TestCapitalSeparaBrutoLiquidoReservaEComprometido(t *testing.T) {
	club := Club{
		Coins: 1000,
		Players: []ClubPlayer{
			{Player: Player{Price: Price{Coins: 101}}, InSquad: false},
			{Player: Player{Price: Price{Coins: 999}}, InSquad: true},
			{Player: Player{Price: Price{Coins: 500}}, Untradeable: true},
		},
	}

	got := club.Capital(200, 300, 50)
	if got.Cash != 1000 || got.ExtraBudget != 200 || got.GrossRaisable != 101 || got.NetRaisable != 95 {
		t.Fatalf("capital bruto/líquido incorreto: %+v", got)
	}
	if got.Available != 945 { // 1000 + 200 + 95 - 300 - 50
		t.Fatalf("disponível incorreto: %d", got.Available)
	}
}
