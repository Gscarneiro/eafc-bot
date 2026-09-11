package api

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

func TestLeituraDoBotEscolheEvoluirAntesDeFodder(t *testing.T) {
	sell := analyze.SellCandidate{
		Player:         domain.ClubPlayer{Player: domain.Player{GGRating: 84.8}},
		Recommendation: "segurar_potencial", EvoGGGain: 8.4, EvoCost: 18000,
	}
	got := leituraDoBot(sell, store.PriceTrend{}, false, []analyze.FodderMatch{{SBCName: "Seleção"}}, nil)
	if got.Kind != "evoluir" || got.FinalGGRating != 93.2 || got.CoinsCost != 18000 {
		t.Errorf("got = %+v, esperava evoluir com final 93.2 e custo 18000", got)
	}
}

func TestLeituraDoBotVenderCaindoUsaTendencia(t *testing.T) {
	sell := analyze.SellCandidate{Recommendation: "vender"}
	trend := store.PriceTrend{ChangePct: -12.0}
	got := leituraDoBot(sell, trend, true, nil, nil)
	if got.Kind != "vender_caindo" || got.ChangePct30d != -12.0 {
		t.Errorf("got = %+v, esperava vender_caindo com -12.0", got)
	}
}

func TestLeituraDoBotNaoVendavelUsaFodderQuandoDisponivel(t *testing.T) {
	sell := analyze.SellCandidate{Recommendation: "nao_vendavel"}
	got := leituraDoBot(sell, store.PriceTrend{}, false, []analyze.FodderMatch{{SBCName: "Upgrade 85+"}}, nil)
	if got.Kind != "nao_vendavel" || got.SBCName != "Upgrade 85+" {
		t.Errorf("got = %+v, esperava nao_vendavel com SBCName preenchido", got)
	}
}

func TestLeituraDoBotPromoverExplicaVagaTitularENotasPosicionais(t *testing.T) {
	titular := domain.ClubPlayer{Player: domain.Player{ID: 1, Name: "Vitinha"}}
	reserva := domain.ClubPlayer{Player: domain.Player{ID: 2, Name: "Florian Wirtz"}}
	swap := &analyze.SquadSwap{
		Index: 9, Slot: domain.CAM, Current: titular, Candidate: reserva,
		CurrentRating: 99.1, CandidateRating: 99.4, GGRatingGap: 0.3,
	}
	got := leituraDoBot(analyze.SellCandidate{Player: reserva, Recommendation: "promover"}, store.PriceTrend{}, false, nil, swap)
	if got.Kind != "promover" || got.Promocao == nil {
		t.Fatalf("leitura = %+v, esperava promoção detalhada", got)
	}
	if got.Promocao.Posicao != domain.CAM || got.Promocao.Titular != "Vitinha" || got.Promocao.NotaTitular != 99.1 || got.Promocao.NotaCandidato != 99.4 || got.Promocao.Ganho != 0.3 {
		t.Errorf("promoção = %+v, esperava CAM, Vitinha, 99.1, 99.4 e +0.3", got.Promocao)
	}
}
