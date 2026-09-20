package domain

import "testing"

func TestGGRatingNaPosicaoPriorizaNotaDaCarta(t *testing.T) {
	for _, caso := range []struct {
		nome              string
		atual, posicional float64
		posAtual, slot    Position
		esperado          float64
		conhecida         bool
	}{
		{"nota da carta maior", 99.74, 99.22, CM, CM, 99.74, true},
		// O metarank não é a mesma escala do GG Rating que vem na carta.
		// A carta Squad Foundations de Carl Starfelt, por exemplo, vem com
		// 79,98 no elenco e 92,4 no metarank para CB. Escolher o maior faria
		// o bot trocar a régua exibida sem avisar.
		{"nota da carta prevalece sobre metarank", 79.98, 92.4, CB, CB, 79.98, true},
		{"notas iguais", 99.22, 99.22, CM, CM, 99.22, true},
		{"metarank em outra posição não substitui GG Rating", 99.41, 98.8, LB, CB, 0, false},
		{"somente clube", 99.74, 0, CM, CM, 99.74, true},
		{"somente metarank não confirma GG Rating", 0, 98.8, "", CB, 0, false},
		{"nenhuma fonte", 0, 0, "", CM, 0, false},
		{"clube sem posição", 99.74, 0, "", CM, 0, false},
		{"sem nota na outra posição", 99.41, 0, LB, CB, 0, false},
		{"valores negativos", -1, -2, CM, CM, 0, false},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			p := Player{GGRating: caso.atual, GGRatingPos: caso.posAtual, GGRatings: map[Position]float64{caso.slot: caso.posicional}}
			got, ok := p.GGRatingAt(caso.slot)
			if got != caso.esperado || ok != caso.conhecida {
				t.Fatalf("nota = %v, conhecida = %v; esperado %v, %v", got, ok, caso.esperado, caso.conhecida)
			}
			if p.GGRating != caso.atual || p.GGRatings[caso.slot] != caso.posicional {
				t.Fatal("consulta alterou as notas das fontes")
			}
		})
	}
}

func TestGGRatingPreservaNotaDeCadaCopia(t *testing.T) {
	mapa := map[Position]float64{CM: 99.22}
	forte := ClubPlayer{Player: Player{ID: 151232182, GGRating: 99.74, GGRatingPos: CM, GGRatings: mapa}, ClubItemID: "evoluida"}
	fraca := ClubPlayer{Player: Player{ID: forte.ID, GGRating: 90.28, GGRatingPos: CM, GGRatings: mapa}, ClubItemID: "original"}
	if got, _ := forte.GGRatingAt(CM); got != 99.74 {
		t.Fatalf("cópia evoluída = %v; esperado 99.74", got)
	}
	if got, _ := fraca.GGRatingAt(CM); got != 90.28 {
		t.Fatalf("cópia original = %v; esperado 90.28", got)
	}
}
