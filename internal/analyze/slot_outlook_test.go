package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func outlookClub(titularGG float64) domain.Club {
	titular := mk(88, domain.RB, 80, 40, 70, 78, 82, 76)
	titular.GGRating = titularGG
	titular.GGRatingPos = domain.RB
	return domain.Club{
		Players: []domain.ClubPlayer{starterCP(1, titular)},
		Squad:   domain.Squad{Starters: []domain.SquadSlot{{Index: 5, Position: domain.RB, PlayerID: 1}}},
	}
}

// Um alvo de mercado sem cotação não pode virar "teto" — "teto" afirma que
// nada supera o titular, e a coleta nem chegou a saber o preço do alvo. Ver
// CLAUDE.md "na dúvida, não afirma".
func TestSlotSemCotacaoNaoViraTeto(t *testing.T) {
	club := outlookClub(82.8)
	upgrades := []Upgrade{{Slot: domain.RB, Unpriced: true}}

	out := BuildSlotOutlooks(club, nil, upgrades)
	if len(out) != 1 {
		t.Fatalf("achou %d slots, esperava 1", len(out))
	}
	if out[0].Kind != SlotOutlookSemCotacao {
		t.Errorf("Kind = %q, esperava %q", out[0].Kind, SlotOutlookSemCotacao)
	}
	if out[0].Delta != 0 {
		t.Errorf("Delta = %v, esperava 0 (sem_cotacao não é um ganho conhecido)", out[0].Delta)
	}
}

// "Teto" só quando NEM o banco (SquadSwap) NEM o mercado com cotação
// superam o titular — não é "o melhor jogador do mundo", é "nada bateu o
// piso desta coleta".
func TestTetoSoQuandoNemMercadoNemBancoSuperam(t *testing.T) {
	club := outlookClub(89.0)

	// Sem swap nenhum, sem upgrade nenhum na posição: teto.
	out := BuildSlotOutlooks(club, nil, nil)
	if len(out) != 1 || out[0].Kind != SlotOutlookTeto {
		t.Fatalf("out = %+v, esperava 1 slot com Kind=%q", out, SlotOutlookTeto)
	}

	// Um upgrade PRECIFICADO na mesma posição não empurra pra "sem_cotacao"
	// nem "teto" vira "melhor_disponivel" sozinho — Upgrade não é banco, é
	// mercado, e SlotOutlook só fala de banco (melhor_disponivel) e preço
	// (sem_cotacao); um upgrade cotado que ainda não bate o titular continua
	// teto.
	priced := BuildSlotOutlooks(club, nil, []Upgrade{{Slot: domain.RB, Unpriced: false}})
	if priced[0].Kind != SlotOutlookTeto {
		t.Errorf("Kind = %q com upgrade cotado, esperava %q", priced[0].Kind, SlotOutlookTeto)
	}

	// Com um SquadSwap de verdade pro mesmo slot: melhor_disponivel, não teto.
	comBanco := BuildSlotOutlooks(club, []SquadSwap{{Index: 5, Slot: domain.RB, GGRatingGap: 2.0}}, nil)
	if comBanco[0].Kind != SlotOutlookMelhorDisponivel {
		t.Errorf("Kind = %q com banco melhor, esperava %q", comBanco[0].Kind, SlotOutlookMelhorDisponivel)
	}
	if comBanco[0].Delta != 2.0 {
		t.Errorf("Delta = %v, esperava 2.0 (GG Rating do SquadSwap, não a escala de Score())", comBanco[0].Delta)
	}
}
